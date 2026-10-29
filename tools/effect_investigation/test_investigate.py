"""Focused behavioral tests for deterministic investigation packets."""
import hashlib
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import unittest

from investigate import (CONTROL_FLOW_STRUCTURAL_LIMITATION, build,
                         canonical_component_bytes, canonical_json, validate_effect_report,
                         validate_event_ledger)


def identifier(prefix, value):
    return f"{prefix}:{hashlib.sha256(value.encode()).hexdigest()}"


def make_node(name, package="example/pkg"):
    return {
        "id": identifier("node", name),
        "package": package,
        "function": name,
        "variant": name,
        "selected": True,
        "synthetic": False,
        "uncertain": False,
        "direct_uncertainty": False,
        "inherited_uncertainty": False,
    }


def make_edge(name, caller, callee, kind="call", resolution="established", candidate=False):
    return {
        "id": identifier("link", name),
        "caller": identifier("node", caller),
        "callee": identifier("node", callee),
        "source": {
            "id": identifier("edge", name),
            "kind": kind,
            "resolution": resolution,
            "location": {"file": "src.go", "line": 4, "column": 1},
        },
        "candidate": candidate,
        "synthetic": False,
    }


def make_site(name="open", site_id=None, uncertain=False):
    return {
        "id": site_id or identifier("site", name),
        "package": "example/pkg",
        "function": "operation",
        "kind": "call",
        "api": "os.Open",
        "apis": ["os.Open"],
        "effects": ["file.read"],
        "uncertain": uncertain,
        "location": {"file": "src.go", "line": 4, "column": 2},
    }


def make_reverse(site_id, edge_ids):
    names = ("caller-a", "caller-b", "shared", "operation")
    return {
        "site": site_id,
        "origins": [identifier("node", "operation")],
        "nodes": [{"id": identifier("node", name), "depth": depth}
                  for depth, name in enumerate(names)],
        "edges": edge_ids,
        "revisit": True,
        "cycle": True,
        "truncated": {"sites": False, "edges": False, "depth": False},
    }


def make_report(two_sites=False):
    site = make_site()
    edges = [
        make_edge("a-shared", "caller-a", "shared"),
        make_edge("b-shared", "caller-b", "shared", "go", "candidate_vta", True),
        make_edge("shared-operation", "shared", "operation", "defer"),
    ]
    sites = [site]
    reverse = [make_reverse(site["id"], [edge["id"] for edge in edges])]
    if two_sites:
        second = make_site("close", identifier("site", "close"))
        sites.append(second)
        reverse.append(make_reverse(second["id"], [edge["id"] for edge in edges]))
    return {
        "schema_version": 1,
        "sites": sites,
        "nodes": [make_node(name, "other/package" if name == "caller-b" else "example/pkg")
                  for name in ("caller-a", "caller-b", "shared", "operation")],
        "edges": edges,
        "reverse": reverse,
        "groups": [{"effect": "file.read", "sites": [site["id"]]}],
        "coverage": {"operation_sites_identified": len(sites), "emitted_sites": len(sites)},
        "truncated": {"sites": False, "edges": False, "depth": False},
    }


def make_chain_report(chain_length=80):
    site = make_site("operation")
    node_names = ["operation", "join"]
    edge_rows = []
    for chain in ("a", "b"):
        node_names.append(f"frontier-{chain}")
        names = [f"{chain}-{index}" for index in range(chain_length)]
        node_names.extend(names)
        path = [f"frontier-{chain}", *names, "join"]
        edge_rows.extend(make_edge(f"{left}-{right}", left, right)
                          for left, right in zip(path, path[1:]))
    edge_rows.append(make_edge("join-operation", "join", "operation"))
    nodes = [make_node(name) for name in node_names]
    edges = edge_rows
    edge_ids = [edge["id"] for edge in edges]
    reverse_nodes = [{"id": node["id"], "depth": 0} for node in nodes]
    return {"schema_version": 1, "sites": [site], "nodes": nodes, "edges": edges,
            "reverse": [{"site": site["id"], "origins": [identifier("node", "operation")],
                         "nodes": reverse_nodes, "edges": edge_ids,
                         "revisit": False, "cycle": False,
                         "truncated": {"sites": False, "edges": False, "depth": False}}],
            "groups": [{"effect": "file.read", "sites": [site["id"]]}],
            "coverage": {"operation_sites_identified": 1, "emitted_sites": 1},
            "truncated": {"sites": False, "edges": False, "depth": False}}


def make_control_flow(report, site_id=None, api="os.Open", status="complete"):
    site_id = site_id or report["sites"][0]["id"]
    return {"schema_version": 1, "schema": "reffector.control-flow.v1",
            "limits": {"max_functions": 64, "max_blocks_per_function": 128,
                       "max_block_visits": 4096, "max_branches": 256,
                       "max_relationships": 2048, "max_depth": 8, "max_json_bytes": 1048576},
            "truncated": {"branches": False},
            "limitations": [CONTROL_FLOW_STRUCTURAL_LIMITATION], "rows": [{"id": identifier("control", "row"), "site": site_id,
                "api": api, "function": {"package": "example/pkg", "function": "operation"},
                "block": -1 if status == "ambiguous_instruction" else 0,
                "instruction": -1 if status == "ambiguous_instruction" else 1,
                "kind": "*ssa.Call", "status": status, "complete": status == "complete",
                **({"instruction_candidates": [{"block": 0, "instruction": 1, "kind": "*ssa.Call"},
                                                 {"block": 0, "instruction": 2, "kind": "*ssa.Call"}]}
                   if status == "ambiguous_instruction" else {})}]}


def make_provenance(report):
    value_id = lambda name: identifier("value", name)
    origin_id = lambda name: identifier("origin", name)
    call_id = identifier("call", "call")
    site_id = identifier("operation", "operation")
    site_ref = report["sites"][0]["id"]
    return {"schema_version": 1, "schema": "reffector.argument-provenance.v1",
            "nodes": [{"id": value_id("origin"), "kind": "constant", "label": "semantic/path-string",
                       "location": {"file": "src.go", "line": 4, "column": 1}},
                      {"id": value_id("parameter"), "kind": "parameter", "label": "arg0",
                       "location": {"file": "src.go", "line": 3, "column": 1}}],
            "edges": [{"id": origin_id("edge"), "from": value_id("origin"), "to": value_id("parameter"),
                       "kind": "input", "resolution": "candidate_vta", "call_ref": call_id}],
            "sites": [{"id": site_id, "site_ref": site_ref, "api": "os.Open",
                       "location": {"file": "src.go", "line": 4, "column": 1},
                       "roles": [{"role": "path", "index": 0, "value_ref": value_id("parameter")}],
                       "call_ref": call_id, "resolution": "established"}],
            "calls": [{"id": call_id, "callsite_ref": "source-call:local", "caller": "pkg.caller",
                       "callee": "os.Open", "location": {"file": "src.go", "line": 4, "column": 1},
                       "resolution": "candidate_function_value", "arguments": [value_id("origin")],
                       "results": []}], "truncation": []}


class InvestigationTests(unittest.TestCase):
    def test_event_ledger_is_optional_validated_and_distinguishes_packet_reads(self):
        self.assertEqual(validate_event_ledger(None)["availability"], "not_provided")
        self.assertIsNone(validate_event_ledger(None)["recorded_counts"])
        empty = validate_event_ledger([])
        self.assertEqual(empty["availability"], "provided")
        self.assertEqual(empty["recorded_counts"]["source_read_events"], 0)
        events = [
            {"type": "reffector_invocation", "invocation_id": "run-1", "target_revision": "abc123",
             "argv": ["reffector", "-stats", "."], "status": "success", "exit_code": 0,
             "binary_sha256": "a" * 64},
            {"type": "source_read", "source_path": "src.go", "start_line": 3, "end_line": 8, "status": "success"},
            {"type": "source_read", "source_path": "missing.go", "status": "failed", "reason": "not found"},
            {"type": "source_search", "query": "os.Open", "scope": "internal", "status": "success", "matches": 2},
            {"type": "candidate", "candidate_id": "candidate:1", "status": "open"},
            {"type": "reproduction", "candidate_id": "candidate:1", "attempt": "go test", "outcome": "passed"},
            {"type": "operation_group", "group_id": "files", "operation_ids": ["site:1"]},
            {"type": "termination", "reason": "phase complete", "wall_duration_ms": 1200,
             "active_duration_status": "unavailable"},
        ]
        ledger = validate_event_ledger(events)
        self.assertEqual(len(ledger["source_reads"]), 2)
        self.assertEqual(ledger["source_searches"][0]["matches"], 2)
        self.assertEqual(ledger["termination"]["wall_duration_ms"], 1200)
        self.assertEqual(ledger["recorded_counts"]["distinct_successfully_read_paths"], 1)
        self.assertEqual(ledger["recorded_counts"]["distinct_operation_groups"], 1)
        self.assertEqual(ledger["completeness"], "only supplied ledger entries; not independently verified")
        measured = validate_event_ledger([{"type": "termination", "reason": "budget stop",
                                           "active_duration_status": "measured", "active_duration_ms": 900}])
        self.assertEqual(measured["termination"]["active_duration_ms"], 900)
        with self.assertRaises(ValueError):
            validate_event_ledger([{"type": "source_read", "source_path": "x", "start_line": 8, "end_line": 2, "status": "success"}])
        with self.assertRaises(ValueError):
            validate_event_ledger([{"type": "source_read", "source_path": "x", "end_line": 2, "status": "success"}])
        with self.assertRaises(ValueError):
            validate_event_ledger([{"type": "source_read", "source_path": "../outside.go", "status": "success"}])
        with self.assertRaises(ValueError):
            validate_event_ledger([{"type": "termination", "reason": "done", "active_duration_status": "measured"}])
        with self.assertRaises(ValueError):
            validate_event_ledger([{"type": "source_search", "query": "search", "scope": "all", "status": "success", "environment": "unsafe"}])

    def test_build_marks_external_activity_not_provided(self):
        result = build(make_report())
        evidence = result["investigation_evidence"]
        self.assertEqual(evidence["availability"], "not_provided")
        self.assertEqual(evidence["external_activity_capture"], "not automatic; only supplied events are represented")
        self.assertEqual(evidence["packet_builder_source_reads"], "separate; excerpts and unavailable-source markers are in packets")
        self.assertIsNone(evidence["recorded_counts"])

    def test_cli_loads_supplied_jsonl_events(self):
        with tempfile.TemporaryDirectory() as temp_dir:
            root = pathlib.Path(temp_dir)
            (root / "effects.json").write_text(json.dumps(make_report()), encoding="utf-8")
            event = {"type": "source_read", "source_path": "src.go", "start_line": 2, "end_line": 4, "status": "success"}
            (root / "events.jsonl").write_text(json.dumps(event) + "\n", encoding="utf-8")
            output = root / "out"
            script = pathlib.Path(__file__).with_name("investigate.py")
            process = subprocess.run(
                [sys.executable, str(script), str(root / "effects.json"), "--events",
                 str(root / "events.jsonl"), "--out", str(output)],
                capture_output=True, text=True, check=False,
            )
            self.assertEqual(process.returncode, 0, process.stderr)
            result = json.loads((output / "investigation.json").read_text(encoding="utf-8"))
            evidence = result["investigation_evidence"]
            self.assertEqual(evidence["source_reads"][0]["source_path"], "src.go")
            self.assertEqual(evidence["recorded_counts"]["successful_source_read_events"], 1)

    def test_shared_catalog_frontiers_degrees_and_reduction_math(self):
        report = make_report(two_sites=True)
        result = build(report, max_packet_bytes=100_000)
        self.assertEqual(result["counts"]["candidates"], 2)
        reduction = result["graph_reduction"]
        self.assertEqual(reduction["raw_site_edge_refs"], 6)
        self.assertEqual(reduction["unique_edge_refs"], 3)
        self.assertEqual(reduction["edge_refs_saved"], 3)
        self.assertEqual(reduction["raw_site_node_refs"], 8)
        self.assertEqual(reduction["unique_node_refs"], 4)
        self.assertEqual(len(result["node_catalog"]), 4)
        self.assertEqual(len(result["relationship_catalog"]), 3)
        candidate = result["candidates"][0]
        self.assertEqual(len(candidate["frontiers"]), 2)
        self.assertIn("multi-frontier-reachability", candidate["categories"])
        self.assertIn("branching-convergence", candidate["categories"])
        self.assertIn("cross-component-shared-operation-site", candidate["categories"])
        packet = result["packets"][0]
        self.assertEqual(packet["graph"]["nodes"][0]["package"], "example/pkg")
        self.assertTrue(packet["graph"]["edges"][0]["caller"]["function"])
        self.assertIn("candidate", packet["graph"]["edges"][1])
        self.assertTrue(packet["graph"]["cycle"])
        self.assertTrue(packet["graph"]["revisit"])
        self.assertTrue(all(node_id in result["node_catalog"] for node_id in packet["graph"]["node_refs"]))
        self.assertTrue(all(edge_id in result["relationship_catalog"] for edge_id in packet["graph"]["edge_refs"]))

    def test_audit_join_coverage_and_audit_only_uncertainty(self):
        report = make_report()
        site_id = report["sites"][0]["id"]
        outside_id = identifier("site", "outside")
        audit = [
            {"record_type": "site", "schema_version": 1, "id": site_id,
             "package": "example/pkg", "function": "operation", "kind": "call",
             "location": {"file": "src.go", "line": 4}, "uncertain": True, "vta_candidates": [{"target": "example.Other"}],
             "dispatch_kinds": ["function_value"], "uncertainty_reasons": ["function_value"],
             "possible_effects": ["file.write"]},
            {"record_type": "site", "schema_version": 1, "id": outside_id,
             "package": "outside", "function": "ignored", "kind": "call",
             "location": {"file": "outside.go", "line": 1}, "uncertain": False},
            {"record_type": "function", "schema_version": 1, "package": "example/pkg",
             "function": "caller-a", "effects": ["env.read"], "uncertain": True,
             "direct_effects": ["env.read"], "inherited_effects": []},
        ]
        result = build(report, audit, max_packet_bytes=100_000)
        coverage = result["audit_coverage"]
        self.assertEqual(coverage["input_site_rows"], 2)
        self.assertEqual(coverage["input_function_rows"], 1)
        self.assertEqual(coverage["unique_site_records_joined_to_report"], 1)
        self.assertEqual(coverage["site_ids_outside_report_count"], 1)
        self.assertEqual(coverage["in_report_sites_not_candidates"], 0)
        self.assertEqual(coverage["unique_function_records_joined_to_candidates"], 1)
        self.assertEqual(coverage["repeated_candidate_function_joins"], 1)
        self.assertIn("candidate-unresolved-relationships", result["candidates"][0]["categories"])
        evidence = result["packets"][0]["audit_evidence"]["site"]
        self.assertEqual(evidence["vta_candidates"], [{"target": "example.Other"}])
        self.assertEqual(evidence["dispatch_kinds"], ["function_value"])
        self.assertNotEqual(result["relationship_catalog"][identifier("link", "b-shared")]["source"]["resolution"], "established")

    def test_minimum_packet_retains_operation_api_alternatives(self):
        report = make_report()
        report["sites"][0]["operations"] = [
            {"api": "os.Open", "effects": ["file.read"]},
            {"api": "os.OpenFile", "effects": ["file.write"]},
        ]
        report["sites"][0]["large_input_payload"] = "x" * 50_000
        result = build(report, max_packet_bytes=48_000)
        packet = result["packets"][0]
        self.assertEqual(packet["reduction_report"]["strategy"], "progressive")
        self.assertEqual([row["api"] for row in packet["operation"]["operations"]],
                         ["os.Open", "os.OpenFile"])
        self.assertEqual(packet["operation"]["operations"][1]["effects"], ["file.write"])

    def test_reduced_audit_summary_retains_structured_effect_references(self):
        report = make_report()
        report["sites"][0]["large_input_payload"] = "x" * 50_000
        effects = [{"effect": f"structured-{index}", "detail": "y" * 1000} for index in range(60)]
        audit = [{"record_type": "function", "schema_version": 1,
                  "package": "example/pkg", "function": "caller-a", "effects": effects,
                  "uncertain": False, "direct_effects": effects, "inherited_effects": []}]
        result = build(report, audit, max_packet_bytes=48_000)
        packet = result["packets"][0]
        summary = packet["audit_evidence"]["functions"]
        self.assertIn("evidence_store_ref", packet["audit_evidence"])
        self.assertEqual(summary["non_string_effect_count"], len(effects))
        self.assertTrue(summary["non_string_effect_refs"])
        self.assertEqual(result["candidates"][0]["audit_evidence"]["functions"][0]["effects"], effects)
        self.assertEqual(packet["reduction_report"]["audit"]["deferred_function_record_count"], 1)
        self.assertLessEqual(len(canonical_json(packet)), 48_000)

    def test_strict_schema_ids_references_and_limits(self):
        report = make_report()
        malformed = json.loads(json.dumps(report))
        malformed["nodes"][0]["id"] = "node:bad"
        with self.assertRaises(ValueError):
            build(malformed)
        dangling = json.loads(json.dumps(report))
        dangling["reverse"][0]["edges"].append(identifier("link", "absent"))
        with self.assertRaises(ValueError):
            build(dangling)
        malformed_audit = [{"record_type": "site", "schema_version": 1, "id": "site:no"}]
        with self.assertRaises(ValueError):
            build(report, malformed_audit)
        for limit in (0, -1):
            with self.assertRaises(ValueError):
                build(report, max_packet_bytes=limit)

    def test_safe_source_declaration_excerpt_and_symlink_escape(self):
        report = make_report()
        source = b"one\ntwo\nthree\nfour\nfive\nsix\n"
        digest = hashlib.sha256(source).hexdigest()
        with tempfile.TemporaryDirectory() as root, tempfile.TemporaryDirectory() as outside:
            source_path = os.path.join(root, "src.go")
            with open(source_path, "wb") as stream:
                stream.write(source)
            declaration = {"package": "example/pkg", "name": "caller-a", "file": "src.go",
                           "start_line": 2, "end_line": 4}
            model = {"schema_version": 1, "declarations": [declaration],
                     "revision": {"revision": "test-revision"}}
            manifest = {"files": {"src.go": {"sha256": digest}}}
            result = build(report, source_root=root, manifest=manifest, program_model=model,
                           max_packet_bytes=100_000)
            packet = result["packets"][0]
            self.assertTrue(any(item["start_line"] == 2 and item["end_line"] == 4
                                for item in packet["source_excerpts"]))
            self.assertTrue(all(item["sha256"] == digest for item in packet["source_excerpts"]))
            self.assertEqual(result["provenance"]["source_revision"], "test-revision")
            # Nested locations serialize repository-relative and equivalent roots agree.
            self.assertEqual(packet["operation"]["location"]["file"], "src.go")
            self.assertEqual(packet["operation"]["location"]["file_status"], "repository_relative")
            with tempfile.TemporaryDirectory() as second_root:
                with open(os.path.join(second_root, "src.go"), "wb") as stream:
                    stream.write(source)
                second = build(report, source_root=second_root, manifest=manifest, program_model=model,
                               max_packet_bytes=100_000)
                self.assertEqual(canonical_json(result["packets"]), canonical_json(second["packets"]))
            os.symlink(os.path.join(outside, "secret.go"), os.path.join(root, "escape.go"))
            with open(os.path.join(outside, "secret.go"), "wb") as stream:
                stream.write(source)
            manifest["files"]["escape.go"] = {"sha256": digest}
            # An operation outside the manifest remains explicit unavailable evidence.
            report["sites"][0]["location"]["file"] = "escape.go"
            escaped = build(report, source_root=root, manifest=manifest, program_model=model,
                            max_packet_bytes=100_000)
            reasons = [item["reason"] for item in escaped["packets"][0]["unavailable_source"]]
            self.assertIn("resolved source path escapes supplied root", reasons)
            self.assertIsNone(escaped["packets"][0]["operation"]["location"]["file"])
            self.assertEqual(escaped["packets"][0]["operation"]["location"]["line"], 4)
            self.assertEqual(escaped["packets"][0]["operation"]["location"]["file_status"], "unavailable")

    def test_absolute_nested_locations_and_recursive_path_redaction(self):
        report = make_report()
        source = b"one\ntwo\nthree\nfour\nfive\nsix\n"
        digest = hashlib.sha256(source).hexdigest()
        with tempfile.TemporaryDirectory() as root, tempfile.TemporaryDirectory() as root_two:
            for base in (root, root_two):
                with open(os.path.join(base, "src.go"), "wb") as stream:
                    stream.write(source)
            manifest = {"files": {"src.go": {"sha256": digest}}}
            report["sites"][0]["location"]["file"] = os.path.join(root, "src.go")
            report["edges"][0]["source"]["location"]["file"] = os.path.join(root, "src.go")
            audit = [{"record_type": "site", "schema_version": 1, "id": report["sites"][0]["id"],
                      "package": "example/pkg", "function": "operation", "kind": "call",
                      "location": {"file": os.path.join(root, "src.go"), "line": 4}, "uncertain": False}]
            provenance = make_provenance(report)
            provenance["sites"][0]["location"]["file"] = os.path.join(root, "src.go")
            provenance["nodes"][0]["location"]["file"] = os.path.join(root, "src.go")
            provenance["edges"].append({"id": identifier("origin", "second-edge"),
                "from": provenance["nodes"][0]["id"], "to": provenance["nodes"][1]["id"],
                "kind": "copy", "resolution": "established"})
            control_flow = make_control_flow(report)
            control_flow["rows"][0]["branches"] = [{"block": 0, "true": 1, "false": 2,
                "condition_kind": "ssa.If", "location": {"file": os.path.join(root, "src.go"),
                "line": 4, "column": 1}}]
            first = build(report, audit, source_root=root, manifest=manifest,
                          argument_provenance=provenance, control_flow=control_flow, max_packet_bytes=100_000)
            report_two = json.loads(json.dumps(report))
            report_two["sites"][0]["location"]["file"] = os.path.join(root_two, "src.go")
            report_two["edges"][0]["source"]["location"]["file"] = os.path.join(root_two, "src.go")
            audit_two = json.loads(json.dumps(audit))
            audit_two[0]["location"]["file"] = os.path.join(root_two, "src.go")
            provenance_two = json.loads(json.dumps(provenance))
            provenance_two["sites"][0]["location"]["file"] = os.path.join(root_two, "src.go")
            provenance_two["nodes"][0]["location"]["file"] = os.path.join(root_two, "src.go")
            control_flow_two = json.loads(json.dumps(control_flow))
            control_flow_two["rows"][0]["branches"][0]["location"]["file"] = os.path.join(root_two, "src.go")
            second = build(report_two, audit_two, source_root=root_two, manifest=manifest,
                           argument_provenance=provenance_two, control_flow=control_flow_two, max_packet_bytes=100_000)
            encoded = canonical_json(first)
            self.assertNotIn(root.encode(), encoded)
            self.assertEqual(canonical_json(first["packets"]), canonical_json(second["packets"]))
            self.assertEqual(first["argument_provenance"]["sites"][0]["location"]["file"], "src.go")
            self.assertEqual(first["control_flow"]["rows"][0]["branches"][0]["location"]["file"], "src.go")
            self.assertEqual(first["packets"][0]["control_flow"]["rows"][0]["branches"][0]["location"]["file"], "src.go")
            # A path-shaped label is argument data, not a location and is preserved.
            self.assertEqual(first["argument_provenance"]["nodes"][0]["label"], "semantic/path-string")
            outside = json.loads(json.dumps(provenance))
            outside["sites"][0]["location"]["file"] = "/private/not-a-real-root/source.go"
            redacted = build(report, source_root=root, manifest=manifest, argument_provenance=outside,
                             max_packet_bytes=100_000)
            self.assertIsNone(redacted["argument_provenance"]["sites"][0]["location"]["file"])
            self.assertNotIn(b"/private/not-a-real-root", canonical_json(redacted))

    def test_progressive_reduction_preserves_graph_and_exact_budget(self):
        report = make_chain_report()
        expanded = build(report, max_candidates=1, max_packet_bytes=1_000_000)
        reduced = build(report, max_candidates=1, max_packet_bytes=48_000)
        candidate = reduced["candidates"][0]
        packet = reduced["packets"][0]
        encoded = canonical_json(packet)
        self.assertEqual(expanded["candidates"][0]["id"], candidate["id"])
        self.assertGreater(candidate["expanded_packet_bytes"], 48_000)
        self.assertEqual(candidate["packet_status"], "emitted")
        self.assertLessEqual(len(encoded), 48_000)
        self.assertEqual(packet["reduction_report"]["final_serialized_bytes"], len(encoded))
        self.assertEqual(packet["reduction_report"]["original_expanded_bytes"],
                         candidate["expanded_packet_bytes"])
        self.assertEqual(packet["reduction_report"]["strategy"], "progressive")
        self.assertIn("compact_call_chain_segments", packet["reduction_report"]["steps"])
        self.assertTrue(packet["graph"]["connectivity_complete"])
        self.assertTrue(packet["graph"]["relationship_refs_complete"])
        graph_report = packet["reduction_report"]["graph"]
        self.assertEqual(graph_report["included_node_refs"], len(packet["graph"]["node_refs"]))
        self.assertEqual(graph_report["deferred_node_refs"],
                         graph_report["total_node_refs"] - graph_report["included_node_refs"])
        self.assertEqual(graph_report["represented_node_count"], graph_report["total_node_refs"])
        represented = set(packet["graph"]["edge_refs"])
        for segment in packet["graph"]["segments"]:
            represented.update(segment["edge_refs"])
        self.assertEqual(represented, set(candidate["edge_ids"]))
        self.assertEqual(len(reduced["relationship_catalog"]), len(candidate["edge_ids"]))
        catalog_refs = {edge_id for segment in candidate["compact_graph_catalog"]["segments"]
                        for edge_id in segment["edge_refs"]}
        catalog_refs.update(edge["id"] for edge in candidate["compact_graph_catalog"]["direct_edges"])
        self.assertEqual(catalog_refs, set(candidate["edge_ids"]))
        self.assertIn("Graph connectivity is structural", packet["coverage"]["evidence_note"])
        tiny = build(report, max_candidates=1, max_packet_bytes=1)
        self.assertEqual([item["id"] for item in tiny["candidates"]],
                         [item["id"] for item in reduced["candidates"]])
        self.assertEqual(tiny["candidates"][0]["packet_status"], "deferred_over_budget")
        self.assertGreater(tiny["deferred"][0]["minimum_packet_bytes"], 1)
        self.assertEqual(tiny["deferred"][0]["unique_edge_refs"], candidate["edge_ids"])

    def test_source_excerpt_selection_is_independent_and_recoverable(self):
        report = make_report()
        report["sites"][0]["location"]["file"] = "operation.go"
        operation_source = b"one\ntwo\nthree\n" + (b"x" * 50_000) + b"\nfive\n"
        caller_source = b"func CallerA() {}\n"
        with tempfile.TemporaryDirectory() as root:
            for name, content in (("operation.go", operation_source), ("caller.go", caller_source)):
                with open(os.path.join(root, name), "wb") as stream:
                    stream.write(content)
            manifest = {"files": {name: {"sha256": hashlib.sha256(content).hexdigest()}
                                  for name, content in (("operation.go", operation_source),
                                                        ("caller.go", caller_source))}}
            model = {"schema_version": 1, "declarations": [
                {"package": "example/pkg", "name": "operation", "file": "operation.go",
                 "start_line": 4, "end_line": 4},
                {"package": "example/pkg", "name": "caller-a", "file": "caller.go",
                 "start_line": 1, "end_line": 1}],
                "revision": {"revision": "source-selection-test"}}
            result = build(report, source_root=root, manifest=manifest, program_model=model,
                           max_packet_bytes=48_000)
        packet = result["packets"][0]
        self.assertEqual(result["candidates"][0]["packet_status"], "emitted")
        self.assertTrue(any(ref["path"] == "operation.go" and "operation_site" in ref["roles"]
                            for ref in packet["source_refs"]))
        self.assertTrue(any(excerpt["path"] == "caller.go"
                            and "frontier_declaration" in excerpt["roles"]
                            for excerpt in packet["source_excerpts"]))
        self.assertFalse(any(excerpt["path"] == "operation.go" for excerpt in packet["source_excerpts"]))
        self.assertGreater(packet["reduction_report"]["source"]["deferred_excerpt_count"], 0)
        self.assertEqual(packet["reduction_report"]["final_serialized_bytes"], len(canonical_json(packet)))
        self.assertNotIn(root.encode(), canonical_json(result))
        refs = {ref["id"] for ref in result["candidates"][0]["source_refs"]}
        self.assertTrue({ref["id"] for ref in packet["source_refs"]} <= refs)

    def test_packet_status_cap_budget_and_deferred_refs(self):
        report = make_report(two_sites=True)
        capped = build(report, max_candidates=1, max_packet_bytes=100_000)
        statuses = [candidate["packet_status"] for candidate in capped["candidates"]]
        self.assertEqual(statuses.count("emitted"), 1)
        self.assertEqual(statuses.count("deferred_max_candidates"), 1)
        budgeted_report = make_report()
        budgeted_report["reverse"][0]["truncated"]["edges"] = True
        budgeted = build(budgeted_report, max_packet_bytes=1)
        self.assertEqual(budgeted["candidates"][0]["packet_status"], "deferred_over_budget")
        self.assertEqual(budgeted["deferred"][0]["unique_edge_refs"], budgeted["candidates"][0]["edge_ids"])
        self.assertTrue(budgeted["candidates"][0]["graph_status"]["cycle"])
        self.assertTrue(budgeted["candidates"][0]["graph_status"]["truncated"]["edges"])
        self.assertEqual(budgeted["deferred"][0]["graph_status"], budgeted["candidates"][0]["graph_status"])
        self.assertTrue(all(len(canonical_json(packet)) <= 100_000 for packet in capped["packets"]))
        self.assertGreater(budgeted["candidates"][0]["deferred_unique_edge_refs"].__len__(), 0)
        selected_over_budget = build(make_report(two_sites=True), max_candidates=1, max_packet_bytes=1)
        by_id = {item["id"]: item for item in selected_over_budget["candidates"]}
        selected_id = min(by_id)
        self.assertEqual(by_id[selected_id]["packet_status"], "deferred_over_budget")
        self.assertEqual(sum(item["packet_status"] == "deferred_max_candidates"
                             for item in selected_over_budget["candidates"]), 1)
        self.assertIn("packet size does not affect selection",
                      selected_over_budget["limits"]["selection_policy"])

    def test_argument_provenance_validation_summary_and_deferral_recovery(self):
        report = make_report()
        provenance = make_provenance(report)
        full_coverage = json.loads(json.dumps(report["coverage"]))
        result = build(report, max_packet_bytes=100_000, argument_provenance=provenance)
        self.assertEqual(result["argument_provenance"]["schema"], provenance["schema"])
        self.assertIsNone(result["argument_provenance"]["sites"][0]["location"]["file"])
        self.assertEqual(result["coverage"]["report"], full_coverage)
        summary = result["packets"][0]["argument_provenance"]["sites"][0]
        self.assertEqual(summary["roles"][0]["role"], "path")
        # Candidate VTA/function-value evidence must not be promoted into a proven origin path.
        self.assertEqual(summary["edges"][0]["resolution"], "candidate_vta")
        self.assertEqual(summary["calls"][0]["resolution"], "candidate_function_value")
        self.assertNotIn("established", summary["edges"][0]["resolution"])
        self.assertNotIn("established", summary["calls"][0]["resolution"])
        self.assertEqual(summary["origins"][0]["label"], "semantic/path-string")
        packet_size = len(canonical_json(result["packets"][0]))
        exact_fit = build(report, max_packet_bytes=packet_size, argument_provenance=provenance)
        self.assertEqual(exact_fit["candidates"][0]["packet_status"], "emitted")
        self.assertEqual(len(canonical_json(exact_fit["packets"][0])), packet_size)
        deferred = build(report, max_packet_bytes=packet_size - 1, argument_provenance=provenance)
        inventory = deferred["candidates"][0]
        record = deferred["deferred"][0]
        self.assertTrue(inventory["provenance_site_refs"])
        self.assertTrue(record["provenance_value_refs"])
        self.assertTrue(record["provenance_edge_refs"])
        self.assertTrue(record["provenance_call_refs"])
        bad = json.loads(json.dumps(provenance))
        bad["edges"][0]["from"] = "value:" + "0" * 64
        with self.assertRaises(ValueError):
            build(report, argument_provenance=bad)
        wrong_api = json.loads(json.dumps(provenance))
        wrong_api["sites"][0]["api"] = "os.Remove"
        with self.assertRaisesRegex(ValueError, "does not match joined report site"):
            build(report, argument_provenance=wrong_api)
        unresolved = json.loads(json.dumps(provenance))
        unresolved["sites"][0].pop("site_ref")
        unresolved["sites"][0]["location"]["column"] = report["sites"][0]["location"]["column"]
        unresolved["sites"][0]["resolution"] = "unresolved_dynamic"
        unresolved["sites"][0]["site_limitation"] = "report_site_unavailable"
        retained = build(report, argument_provenance=unresolved)
        self.assertEqual(retained["argument_provenance"]["sites"][0]["site_limitation"],
                         "report_site_unavailable")
        self.assertEqual(len(retained["packets"][0]["argument_provenance"]["sites"]), 1)
        unrelated = json.loads(json.dumps(unresolved))
        unrelated["sites"][0]["location"]["line"] += 1
        separate = build(report, argument_provenance=unrelated)
        self.assertEqual(separate["packets"][0]["argument_provenance"]["sites"], [])
        self.assertEqual(separate["candidates"][0]["provenance_site_refs"], [])
        self.assertEqual(len(separate["argument_provenance"]["sites"]), 1)

    def test_control_flow_exact_join_validation_retention_and_recovery(self):
        report = make_report()
        projection = make_control_flow(report)
        result = build(report, control_flow=projection, max_packet_bytes=100_000)
        packet_cf = result["packets"][0]["control_flow"]
        self.assertIn(CONTROL_FLOW_STRUCTURAL_LIMITATION, packet_cf["limitations"])
        # A structurally mapped block is not a path-condition or execution proof.
        self.assertIsNone(packet_cf["rows"][0]["branches"])
        self.assertNotIn("path_condition", packet_cf["rows"][0])
        self.assertEqual(packet_cf["rows"][0]["id"], projection["rows"][0]["id"])
        self.assertEqual(result["control_flow"]["rows"][0]["id"], projection["rows"][0]["id"])

        wrong_api = make_control_flow(report, api="os.Remove")
        with self.assertRaisesRegex(ValueError, "does not match joined report site"):
            build(report, control_flow=wrong_api)
        dangling = make_control_flow(report, site_id=identifier("site", "missing"))
        with self.assertRaisesRegex(ValueError, "dangling control-flow site"):
            build(report, control_flow=dangling)
        duplicate = make_control_flow(report)
        duplicate["rows"].append(dict(duplicate["rows"][0]))
        with self.assertRaisesRegex(ValueError, "duplicate control-flow ID"):
            build(report, control_flow=duplicate)
        bad_resolution = make_control_flow(report)
        bad_resolution["rows"][0]["resolutions"] = ["execution_proven"]
        with self.assertRaisesRegex(ValueError, "invalid control-flow resolution"):
            build(report, control_flow=bad_resolution)
        missing_limitation = make_control_flow(report)
        missing_limitation["limitations"] = []
        with self.assertRaisesRegex(ValueError, "structural-only limitation"):
            build(report, control_flow=missing_limitation)
        missing_site_limitation = make_control_flow(report)
        missing_site_limitation["truncated"]["effect_report_sites"] = True
        with self.assertRaisesRegex(ValueError, "EffectReport limitation"):
            build(report, control_flow=missing_site_limitation)

        # A row at a matching source position but with a different canonical site ID is not joined.
        report_two = make_report(two_sites=True)
        other_site_id = report_two["sites"][1]["id"]
        unmatched = make_control_flow(report_two, site_id=other_site_id, api="os.Open")
        joined = build(report_two, control_flow=unmatched, max_packet_bytes=100_000)
        first_packet = next(packet for packet in joined["packets"]
                            if packet["operation"]["id"] == report_two["sites"][0]["id"])
        self.assertEqual(first_packet["control_flow"]["rows"], [])
        without_cf = build(report_two, max_candidates=1, max_packet_bytes=1)
        with_cf = build(report_two, control_flow=unmatched, max_candidates=1, max_packet_bytes=1)
        self.assertEqual([item["id"] for item in without_cf["candidates"]],
                         [item["id"] for item in with_cf["candidates"]])

        unavailable_report = json.loads(json.dumps(report))
        unavailable_report["sites"][0]["api"] = ""
        unavailable_report["sites"][0]["apis"] = []
        unavailable = make_control_flow(unavailable_report, api="", status="unavailable_api_identity")
        retained_unavailable = build(unavailable_report, control_flow=unavailable)
        self.assertEqual(retained_unavailable["control_flow"]["rows"][0]["api"], "")
        self.assertEqual(retained_unavailable["packets"][0]["control_flow"]["rows"], [])
        with self.assertRaisesRegex(ValueError, "omits an API available"):
            build(report, control_flow=make_control_flow(report, api="", status="unavailable_api_identity"))

        ambiguous = make_control_flow(report, status="ambiguous_instruction")
        deferred = build(report, control_flow=ambiguous, max_packet_bytes=1)
        self.assertEqual(deferred["candidates"][0]["control_flow_row_refs"],
                         [{"id": ambiguous["rows"][0]["id"], "status": "ambiguous_instruction"}])
        self.assertEqual(deferred["candidates"][0]["control_flow_truncated"], {"branches": False})
        self.assertEqual(deferred["deferred"][0]["control_flow_row_refs"],
                         deferred["candidates"][0]["control_flow_row_refs"])
        self.assertEqual(deferred["deferred"][0]["control_flow_truncated"], {"branches": False})
        self.assertEqual(deferred["control_flow"]["rows"][0]["status"], "ambiguous_instruction")

    def test_canonical_component_byte_accounting_and_no_provenance_compatibility(self):
        packet = {"alpha": [1, "é"], "beta": {"x": True}}
        measured = canonical_component_bytes(packet)
        self.assertEqual(measured["total"], len(canonical_json(packet)))
        self.assertEqual(measured["total"], sum(measured["members"].values()) + measured["framing"])
        plain = build(make_report(), max_packet_bytes=100_000)
        self.assertNotIn("argument_provenance", plain["packets"][0])
        self.assertNotIn("argument_provenance", plain)
        self.assertNotIn("control_flow", plain["packets"][0])
        self.assertNotIn("control_flow", plain)
        self.assertEqual(plain["packets"][0]["coverage"]["operation_sites_identified"], 1)

    def test_report_coverage_truncation_and_candidate_id_stability(self):
        report = make_report()
        report["truncated"]["depth"] = True
        report["reverse"][0]["truncated"]["edges"] = True
        first = build(report, max_packet_bytes=100_000)
        second = build(report, max_packet_bytes=100_000)
        self.assertEqual(first["candidates"][0]["id"], second["candidates"][0]["id"])
        self.assertTrue(first["packets"][0]["coverage"]["report_truncated"]["depth"])
        self.assertTrue(first["packets"][0]["graph"]["truncated"]["edges"])
        self.assertEqual(first["packets"][0]["coverage"]["operation_sites_identified"], 1)
        self.assertNotIn("report", first["packets"][0]["coverage"])
        self.assertEqual(first["coverage"]["report"], report["coverage"])
        self.assertEqual(first["coverage"]["report_truncated"], report["truncated"])
        self.assertGreater(first["token_estimate"]["packets"][0]["tokens_heuristic"], 0)
        validate_effect_report(report)


if __name__ == "__main__":
    unittest.main()
