"""Per-invocation immutable inventory versus dynamic packet selection metadata."""
import copy
import pathlib
import tempfile
import unittest
from unittest.mock import patch

import investigate as inv
from fixtures import immediate_reduction_report as baseline
from fixtures import original_packet_reducer as original
from test_reducer_equivalence import deterministic_cases, source_case
from test_investigate import make_chain_report


def immediate_build(report, **options):
    # Other builder functions are unchanged; the new private snapshot is unused
    # by the frozen immediate metadata implementation.
    with patch.object(inv, "_reduction_report", baseline._reduction_report):
        return inv.build(report, **options)


def original_build(report, **options):
    with patch.object(original, "_reduction_report", baseline._reduction_report), \
         patch.object(inv, "_refresh_reduction_report", original._refresh_reduction_report), \
         patch.object(inv, "_progressively_reduce_packet", original._progressively_reduce_packet):
        return inv.build(report, **options)


def capture_reduction(report, options):
    packets, calls = [], []
    metadata, reduce = inv._reduction_report, inv._progressively_reduce_packet
    def observe_metadata(packet, state):
        if state["strategy"] == "progressive" and not packets:
            packets.append(copy.deepcopy((packet, state)))
        return metadata(packet, state)
    def observe_reduce(*args):
        if not calls:
            calls.append(copy.deepcopy(args))
        return reduce(*args)
    with patch.object(inv, "_reduction_report", observe_metadata), \
         patch.object(inv, "_progressively_reduce_packet", observe_reduce):
        inv.build(report, **options)
    return packets[0], calls[0]


class MetadataEquivalenceTests(unittest.TestCase):
    def assert_three_way_equal(self, report, options):
        untouched = copy.deepcopy((report, options))
        expected = inv.canonical_json(immediate_build(report, **options))
        self.assertEqual(inv.canonical_json(original_build(report, **options)), expected)
        result = inv.build(report, **options)
        self.assertEqual(inv.canonical_json(result), expected)
        self.assertEqual((report, options), untouched)
        for packet in result["packets"]:
            size = len(inv.canonical_json(packet))
            self.assertLessEqual(size, options["max_packet_bytes"])
            self.assertEqual(size, packet["reduction_report"]["final_serialized_bytes"])

    def test_complete_results_match_immediate_and_original_oracles(self):
        for name, report, options in deterministic_cases():
            with self.subTest(name=name):
                self.assert_three_way_equal(report, options)
        with tempfile.TemporaryDirectory() as temporary:
            root = pathlib.Path(temporary)
            report, options = source_case(root)
            # Force progressive reduction while retaining real, verified source.
            report["sites"][0]["large_input_payload"] = "x" * 50_000
            minimum = immediate_build(report, max_packet_bytes=1, **options)["candidates"][0]["minimum_packet_bytes"]
            for budget in (1_000_000, minimum-1, minimum, minimum+2_000):
                with self.subTest(source_budget=budget):
                    self.assert_three_way_equal(report, {**options, "max_packet_bytes": budget})
            (root / "src.go").unlink()
            self.assert_three_way_equal(report, {**options, "max_packet_bytes": minimum+2_000})

    def test_selected_refs_excerpts_and_unavailable_counts_are_never_cached(self):
        with tempfile.TemporaryDirectory() as temporary:
            report, options = source_case(pathlib.Path(temporary))
            report["sites"][0]["large_input_payload"] = "x" * 50_000
            (packet, state), _ = capture_reduction(report, {**options, "max_packet_bytes": 20_000})
        inventory = state["source_ref_ids_all"]
        self.assertIsInstance(inventory, frozenset)
        untouched_inventory = inventory.copy()
        def check():
            self.assertEqual(inv._reduction_report(packet, state), baseline._reduction_report(packet, state))
            self.assertIs(state["source_ref_ids_all"], inventory)
            self.assertEqual(inventory, untouched_inventory)
        check()
        saved = copy.deepcopy((packet["source_refs"], packet["source_excerpts"]))
        item = state["source_items"][-1]
        packet["source_refs"].extend([copy.deepcopy(item["ref"])] * 2)
        packet["source_excerpts"].extend([copy.deepcopy(item["excerpt"])] * 2)
        check()  # Duplicates count as one selected identity, not two.
        packet["source_refs"] = []
        packet["source_excerpts"] = []
        check()  # Removal must immediately change included/deferred counts.
        packet["source_refs"], packet["source_excerpts"] = saved
        check()  # Rollback restores the original membership without touching inventory.
        packet["source_refs"].append({"id": "source:unrelated"})
        check()  # Preserve the original intersection and deferred-count rules.
        state["unavailable_source"].append({"reason": "missing"})
        check()

    def test_populated_and_empty_inventory_snapshots_are_not_rescanned(self):
        reports = [(make_chain_report(), {"max_packet_bytes": 48_000})]
        with tempfile.TemporaryDirectory() as temporary:
            report, options = source_case(pathlib.Path(temporary))
            report["sites"][0]["large_input_payload"] = "x" * 50_000
            reports.append((report, {**options, "max_packet_bytes": 20_000}))
            for report, options in reports:
                (packet, state), _ = capture_reduction(report, options)
                expected = baseline._reduction_report(packet, state)
                count = len(state["source_items"])
                class LengthOnly:
                    def __len__(self):
                        return count
                    def __iter__(self):
                        raise AssertionError("immutable source inventory was rescanned")
                state["source_items"] = LengthOnly()
                for _ in range(3):
                    self.assertEqual(inv._reduction_report(packet, state), expected)

    def test_new_reductions_snapshot_changed_source_items_and_duplicate_id_rules(self):
        with tempfile.TemporaryDirectory() as temporary:
            report, options = source_case(pathlib.Path(temporary))
            report["sites"][0]["large_input_payload"] = "x" * 50_000
            (_, _), args = capture_reduction(report, {**options, "max_packet_bytes": 20_000})
            source_items = args[4]
            changed = copy.deepcopy(source_items[0])
            changed["ref"]["id"] = "source:changed"
            # IDs for the full excerpt universe deliberately come from ref.id,
            # not excerpt.id. That existing rule must not change during reuse.
            changed["excerpt"]["id"] = "source:distinct-excerpt"
            variants = (source_items, source_items + [copy.deepcopy(source_items[0])],
                        source_items + [changed], source_items[1:], [])
            previous = None
            for items in variants:
                run_args = list(copy.deepcopy(args))
                run_args[4] = copy.deepcopy(items)
                with patch.object(inv, "_reduction_report", baseline._reduction_report):
                    expected = inv._progressively_reduce_packet(*run_args)
                with patch.object(original, "_reduction_report", baseline._reduction_report):
                    historical = original._progressively_reduce_packet(*run_args)
                self.assertEqual(inv.canonical_json(expected), inv.canonical_json(historical))
                snapshots = []
                metadata = inv._reduction_report
                def observe(packet, state):
                    snapshot = state["source_ref_ids_all"]
                    if not snapshots:
                        snapshots.append(snapshot)
                    self.assertIs(snapshot, snapshots[0])
                    self.assertEqual(snapshot, frozenset(item["ref"]["id"] for item in items))
                    return metadata(packet, state)
                with patch.object(inv, "_reduction_report", observe):
                    actual = inv._progressively_reduce_packet(*run_args)
                self.assertEqual(inv.canonical_json(actual), inv.canonical_json(expected))
                self.assertIsNot(snapshots[0], previous)
                previous = snapshots[0]


if __name__ == "__main__":
    unittest.main()
