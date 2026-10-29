"""Whole-result differential checks against the frozen pre-optimization reducer."""
import copy
import hashlib
import pathlib
import tempfile
import unittest
from unittest.mock import patch

import investigate as inv
from fixtures import original_packet_reducer as original
from test_investigate import (identifier, make_chain_report, make_control_flow,
                              make_edge, make_node, make_provenance, make_report)


def reference_build(report, **options):
    with patch.object(inv, "_refresh_reduction_report", original._refresh_reduction_report), \
         patch.object(inv, "_progressively_reduce_packet", original._progressively_reduce_packet):
        return inv.build(report, **options)


def deterministic_cases():
    """Budgets are derived with the oracle, never with the optimized measurement."""
    report = make_report(two_sites=True)
    full = reference_build(report, max_packet_bytes=1_000_000)
    exact = len(inv.canonical_json(full["packets"][0]))
    yield "expanded", report, {"max_packet_bytes": 1_000_000}
    yield "exact-expanded", report, {"max_packet_bytes": exact}
    yield "one-byte-over", report, {"max_packet_bytes": exact - 1}
    yield "candidate-cap", report, {"max_candidates": 1, "max_packet_bytes": 1_000_000}

    chain = make_chain_report(30)
    # Preserve mixed resolutions, cycles/truncation and literal escaping in nested evidence.
    for index, edge in enumerate(chain["edges"]):
        if index % 3 == 0:
            edge["candidate"] = True
            edge["source"]["resolution"] = "candidate_function_value"
    chain["sites"][0]["uncertainty_reasons"] = ['é漢🙂 "quoted"\\line\n\t\x01']
    chain["sites"][0]["uncertain"] = True
    chain["coverage"]["nested"] = {"strings": ["é", "漢", "🙂", "\n\\\""], "empty": {}}
    chain["truncated"]["depth"] = True
    chain["reverse"][0]["truncated"]["edges"] = True
    provenance = make_provenance(chain)
    provenance["nodes"][0]["label"] = 'é漢🙂 "\\\n'
    control = make_control_flow(chain)
    options = {"argument_provenance": provenance, "control_flow": control}
    minimum = reference_build(chain, max_packet_bytes=1, **options)["candidates"][0]["minimum_packet_bytes"]
    yield "minimum-rejected", chain, {**options, "max_packet_bytes": minimum - 1}
    yield "exact-minimum", chain, {**options, "max_packet_bytes": minimum}
    yield "progressive-nested", chain, {**options, "max_packet_bytes": minimum + 4_000}
    audit = [{"record_type": "site", "schema_version": 1, "id": chain["sites"][0]["id"],
              "package": "example/pkg", "function": "operation", "kind": "call",
              "location": {"file": "src.go", "line": 4}, "uncertain": True,
              "possible_effects": ["file.write"], "vta_candidates": [{"target": "é漢🙂"}],
              "detail": {"path": "/outside/source.go", "rows": ["escaped\\n"] * 500}},
             {"record_type": "function", "schema_version": 1, "package": "example/pkg",
              "function": "operation", "effects": ["file.read", {"path": "unknown.go"}],
              "uncertain": True}]
    yield "progressive-audit", chain, {**options, "audit": audit, "max_packet_bytes": minimum + 4_000}

    # Many path-free graph additions cannot fit and must restore the previous graph/report.
    wide = make_report()
    for index in range(16):
        name = f"caller-{index}-" + "é漢🙂" * 100
        node = make_node(name)
        edge = make_edge(str(index), name, "operation", candidate=bool(index % 2),
                         resolution="candidate_vta" if index % 2 else "established")
        wide["nodes"].append(node)
        wide["edges"].append(edge)
        wide["reverse"][0]["nodes"].append({"id": node["id"], "depth": 1})
        wide["reverse"][0]["edges"].append(edge["id"])
    minimum = reference_build(wide, max_packet_bytes=1)["candidates"][0]["minimum_packet_bytes"]
    yield "repeated-rollbacks", wide, {"max_packet_bytes": minimum + 800}


def source_case(root):
    content = ('package example\n// é漢🙂 "\\\t\n' + '// source\n' * 30).encode()
    (root / "src.go").write_bytes(content)
    manifest = {"files": {"src.go": {"sha256": hashlib.sha256(content).hexdigest()}}}
    report = make_report()
    model = {"schema_version": 1, "declarations": [
        {"package": "example/pkg", "name": name, "file": "src.go",
         "start_line": 1, "end_line": 30} for name in ("operation", "caller-a", "shared")
    ]}
    report["sites"][0]["location"]["file"] = str(root / "src.go")
    options = {"source_root": str(root), "manifest": manifest, "program_model": model,
               "argument_provenance": make_provenance(report), "control_flow": make_control_flow(report)}
    return report, options


class ReducerEquivalenceTests(unittest.TestCase):
    def assert_equivalent(self, report, options):
        untouched = copy.deepcopy((report, options))
        expected = reference_build(report, **options)
        actual = inv.build(report, **options)
        self.assertEqual(inv.canonical_json(actual), inv.canonical_json(expected))
        self.assertEqual((report, options), untouched, "shared canonical inputs were mutated")
        for packet in actual["packets"]:
            size = len(inv.canonical_json(packet))
            self.assertEqual(packet["reduction_report"]["final_serialized_bytes"], size)
            self.assertLessEqual(size, options["max_packet_bytes"])
            candidate = next(row for row in actual["candidates"] if row["id"] == packet["candidate_id"])
            self.assertEqual(candidate["final_packet_bytes"], size)
            for edge in packet["graph"].get("edges", []):
                self.assertIn(edge["id"], actual["relationship_catalog"])
            for node_id in packet["graph"]["node_refs"]:
                self.assertIn(node_id, actual["node_catalog"])
        return actual

    def test_complete_canonical_results_match_original_reducer(self):
        for name, report, options in deterministic_cases():
            with self.subTest(case=name):
                self.assert_equivalent(report, options)

    def test_source_additions_normalize_new_values_without_mutating_evidence(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            report, options = source_case(root)
            minimum = reference_build(report, max_packet_bytes=1, **options)["candidates"][0]["minimum_packet_bytes"]
            for budget in (1_000_000, minimum - 1, minimum, minimum + 2_000):
                with self.subTest(budget=budget):
                    self.assert_equivalent(report, {**options, "max_packet_bytes": budget})
            # A later build must reverify changed content, not reuse stale normalized paths.
            (root / "src.go").write_text("changed\n", encoding="utf-8")
            result = self.assert_equivalent(report, {**options, "max_packet_bytes": minimum + 2_000})
            self.assertIn(b"source hash mismatch", inv.canonical_json(result))
            # Missing and escaping paths remain unavailable, not normalized via a cache.
            for name in ("absent.go", "../escape.go", "/outside/source.go"):
                report["sites"][0]["location"]["file"] = name
                self.assert_equivalent(report, {**options, "max_packet_bytes": minimum + 2_000})

    def test_exact_size_convergence_handles_decimal_boundaries_and_negative_savings(self):
        for padding in (0, 9, 10, 89, 90, 999, 1000):
            for expanded_size in (0, 1, 999, 10_000):
                with self.subTest(padding=padding, expanded_size=expanded_size):
                    packet = {"nested": [{"text": 'é漢🙂 "\\\\\n\\t' * padding}], "array": [[], {}, None]}
                    state = {}
                    def report(_packet, _state):
                        return {"original_expanded_bytes": expanded_size, "final_serialized_bytes": 0,
                                "net_bytes_saved": 0, "steps": ["escaped\\nstep"]}
                    expected = copy.deepcopy(packet)
                    with patch.object(original, "_reduction_report", report):
                        expected_size = original._refresh_reduction_report(expected, state)
                    with patch.object(inv, "_reduction_report", report), \
                         patch.object(inv, "canonical_json", wraps=inv.canonical_json) as encode:
                        actual_size = inv._refresh_reduction_report(packet, state)
                        # One body measurement, one independent final serialization.
                        self.assertEqual(encode.call_count, 2)
                    self.assertEqual(actual_size, expected_size)
                    self.assertEqual(inv.canonical_json(packet), inv.canonical_json(expected))

    def test_eight_pass_trial_fallback_is_preserved_but_invalid_completion_fails_closed(self):
        # Existing decimal-width oscillation: the old eight-pass fallback returns
        # 120 bytes while its report says 121. Keep trial decisions identical;
        # the requested final integrity check must reject such a completed packet.
        def report(_packet, _state):
            return {"original_expanded_bytes": 130, "final_serialized_bytes": 0, "net_bytes_saved": 0}
        expected = {"text": "é漢🙂"}
        with patch.object(original, "_reduction_report", report):
            size = original._refresh_reduction_report(expected, {})
        self.assertNotEqual(size, expected["reduction_report"]["final_serialized_bytes"])
        actual = {"text": "é漢🙂"}
        with patch.object(inv, "_reduction_report", report):
            self.assertEqual(inv._refresh_reduction_report(actual, {}, verify=False), size)
            self.assertEqual(inv.canonical_json(actual), inv.canonical_json(expected))
            with self.assertRaisesRegex(ValueError, "canonical size"):
                inv._refresh_reduction_report({"text": "é漢🙂"}, {})

    def test_independent_final_check_rejects_stale_or_incorrect_sizes(self):
        packet = reference_build(make_report(), max_packet_bytes=100_000)["packets"][0]
        size = len(inv.canonical_json(packet))
        self.assertEqual(inv._checked_reduction_size(packet, size), size)
        with self.assertRaisesRegex(ValueError, "canonical size"):
            inv._checked_reduction_size(packet, size - 1)
        for field in ("final_serialized_bytes", "net_bytes_saved"):
            broken = copy.deepcopy(packet)
            broken["reduction_report"][field] += 1
            with self.assertRaisesRegex(ValueError, "canonical size"):
                inv._checked_reduction_size(broken, len(inv.canonical_json(broken)))

    def test_failed_additions_restore_graph_steps_and_reduction_metadata(self):
        name, report, options = next(case for case in deterministic_cases() if case[0] == "repeated-rollbacks")
        failures = []
        original_try = inv._try_packet_addition

        def observe(packet, state, budget, label, rollback):
            before = copy.deepcopy(packet["reduction_report"])
            accepted = original_try(packet, state, budget, label, rollback)
            if not accepted:
                failures.append(label)
                self.assertEqual(packet["reduction_report"], before)
                self.assertEqual(len(inv.canonical_json(packet)), before["final_serialized_bytes"])
            return accepted

        with patch.object(inv, "_try_packet_addition", observe):
            self.assert_equivalent(report, options)
        self.assertGreaterEqual(failures.count("caller_relationships"), 8)


if __name__ == "__main__":
    unittest.main()
