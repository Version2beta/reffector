from __future__ import annotations

import hashlib
import pathlib
import tempfile
import sys
import unittest

ROOT = pathlib.Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT))

from tools.archchar.selection import retrieve_evidence_records, select_context  # noqa: E402
from tools.archchar.source_selection import select_source_context  # noqa: E402


class SelectionTests(unittest.TestCase):
    def setUp(self) -> None:
        self.question = {"id": "download-flow", "prompt": "Trace download ranges and waiters."}
        self.target = {"id": "unit", "packages": ["example/x"], "files": ["a.go"]}
        self.source = {"declarations": [
            {"id": "fn:Download", "kind": "function", "name": "Download", "package": "example/x", "file": "a.go", "line_start": 1, "line_end": 8},
            {"id": "fn:Pure", "kind": "function", "name": "Pure", "package": "example/x", "file": "a.go", "line_start": 10, "line_end": 12},
        ]}
        self.evidence = {"revision": {"revision": "abc"}, "source_manifest_sha256": "hash", "coverage": {"truncated": False}, "records": [
            {"ref": "effect-site:download", "record_type": "effect_site", "package": "example/x", "file": "a.go", "source_ref": {"file": "a.go", "line": 3}, "data": {"function": "Download", "effects": ["fs.read"]}},
            {"ref": "call-edge:download", "record_type": "call_edge", "package": "example/x", "file": "a.go", "source_ref": {"file": "a.go", "line": 4}, "data": {"caller_package": "example/x", "callee_package": "example/other", "candidate": True, "caller": {"function": "Download"}}},
            {"ref": "unresolved:download", "record_type": "unresolved_site", "package": "example/x", "file": "a.go", "source_ref": {"file": "a.go", "line": 5}, "data": {"function": "Download", "uncertain": True}},
            {"ref": "effect-site:outside", "record_type": "effect_site", "package": "example/out", "file": "outside.go", "data": {"function": "Else"}},
            {"ref": "effect-site:conflict", "record_type": "effect_site", "package": "example/x", "file": "a.go", "data": {"id": "conflict", "effects": ["fs.read"]}},
            {"ref": "audit-site:conflict", "record_type": "audit_site", "package": "example/x", "file": "a.go", "data": {"id": "conflict", "effects": ["fs.write"]}},
        ]}

    def test_selection_is_deterministic_and_covers_candidates_and_deferred(self) -> None:
        first = select_context(self.question, self.target, self.evidence, self.source, 100000)
        second = select_context(self.question, self.target, self.evidence, self.source, 100000)
        self.assertEqual(first, second)
        refs = {row["ref"] for row in first["selected"]}
        self.assertIn("call-edge:download", refs)
        self.assertIn("unresolved:download", refs)
        self.assertEqual(first["not_inspected_refs"], ["effect-site:outside"])
        self.assertEqual({item["ref"] for item in first["source_declarations"]}, {"fn:Download", "fn:Pure"})
        self.assertEqual(first["coverage"]["truncated"], False)
        conflict = first["contradiction_groups_inspected"]
        self.assertEqual(len(conflict), 1)
        self.assertEqual(set(conflict[0]["refs"]), {"effect-site:conflict", "audit-site:conflict"})
        self.assertTrue({"effect-site:conflict", "audit-site:conflict"}.issubset(refs))

    def test_budget_defers_without_mutating_canonical_index(self) -> None:
        before = repr(self.evidence)
        result = select_context(self.question, self.target, self.evidence, self.source, 1)
        self.assertEqual(self.evidence.__repr__(), before)
        self.assertEqual(result["selected"], [])
        self.assertEqual(result["available_count"], 5)
        self.assertEqual(result["deferred_count"], 5)
        self.assertEqual(sum(result["coverage"]["deferred_by_record_type"].values()), 5)
        self.assertTrue(result["coverage"]["truncated"])
        self.assertIn("not semantic sufficiency", " ".join(result["uncertainty"]))

    def test_missing_indexes_and_invalid_budget_fail_closed(self) -> None:
        with self.assertRaises(ValueError):
            select_context(self.question, self.target, {}, self.source, 10)
        with self.assertRaises(ValueError):
            select_context(self.question, self.target, self.evidence, self.source, 0)

    def test_source_selection_is_bounded_hash_checked_and_question_directed(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            path = root / "a.go"
            raw = ("package x\nfunc DownloadRange() {}\nfunc ExportOne() {}\n"
                   "func ExportTwo() {}\nfunc ExportThree() {}\n").encode()
            path.write_bytes(raw)
            source_model = {"files": [{"path": "a.go", "imports": []}], "declarations": [
                {"id": "download", "package": "example/x", "file": "a.go", "name": "DownloadRange", "kind": "function", "symbol": "DownloadRange", "exported": True, "has_body": True, "start_line": 2, "end_line": 2},
                {"id": "one", "package": "example/x", "file": "a.go", "name": "ExportOne", "kind": "function", "symbol": "ExportOne", "exported": True, "has_body": True, "start_line": 3, "end_line": 3},
                {"id": "two", "package": "example/x", "file": "a.go", "name": "ExportTwo", "kind": "function", "symbol": "ExportTwo", "exported": True, "has_body": True, "start_line": 4, "end_line": 4},
                {"id": "three", "package": "example/x", "file": "a.go", "name": "ExportThree", "kind": "function", "symbol": "ExportThree", "exported": True, "has_body": True, "start_line": 5, "end_line": 5},
            ]}
            manifest = {"a.go": {"sha256": hashlib.sha256(raw).hexdigest()}}
            result = select_source_context({"id": "q", "prompt": "Download range"},
                                           {"id": "unit", "packages": ["example/x"], "files": ["a.go"]},
                                           source_model, manifest, root, 1000)
            refs = {item["ref"] for item in result["selected"]}
            self.assertIn("download", refs)
            self.assertLess(len(refs), 4)
            self.assertTrue(result["retrieval_log"])
            self.assertTrue(result["source_coverage"]["full_source_remains_retrievable"])
            stale = dict(manifest)
            stale["a.go"] = {"sha256": "0" * 64}
            with self.assertRaisesRegex(ValueError, "hash mismatch"):
                select_source_context({"id": "q", "prompt": "Download range"},
                                      {"id": "unit", "packages": ["example/x"], "files": ["a.go"]},
                                      source_model, stale, root, 1000)

    def test_local_evidence_retrieval_is_revision_checked_logged_and_bounded(self) -> None:
        revision = self.evidence["revision"]
        result = retrieve_evidence_records(self.evidence, ["effect-site:download"], revision, 10000)
        self.assertEqual(result["retrieved_refs"], ["effect-site:download"])
        self.assertEqual(result["log"][0]["ref"], "effect-site:download")
        self.assertEqual(result["records"][0]["record"]["function"], "Download")
        with self.assertRaisesRegex(ValueError, "revision"):
            retrieve_evidence_records(self.evidence, ["effect-site:download"], {"revision": "changed"}, 10000)
        with self.assertRaisesRegex(ValueError, "unknown evidence"):
            retrieve_evidence_records(self.evidence, ["outside:ref"], revision, 10000)
        with self.assertRaisesRegex(ValueError, "budget"):
            retrieve_evidence_records(self.evidence, ["effect-site:download"], revision, 1)
        with self.assertRaisesRegex(ValueError, "unique"):
            retrieve_evidence_records(self.evidence, ["effect-site:download", "effect-site:download"], revision, 10000)


if __name__ == "__main__":
    unittest.main()
