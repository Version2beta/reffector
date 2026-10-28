from __future__ import annotations

import json
import os
import pathlib
import sys
import tempfile
import unittest
from types import SimpleNamespace
from unittest.mock import patch

ROOT = pathlib.Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT))

from tools.archchar.core import (  # noqa: E402
    aggregate,
    benchmark,
    benchmark_evidence,
    benchmark_treatment_context,
    canonical_json,
    claim_conflicts,
    claim_score,
    index_evidence,
    investigate,
    prepare,
    request_size,
    safe_command_display,
    sanitize_model_value,
    valid_claim_shape,
    claim_evidence_issues,
    verify,
    write_json,
)


class ArchcharTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = pathlib.Path(self.temp.name)
        self.repo = self.base / "repo"
        self.repo.mkdir()
        (self.repo / "go.mod").write_text("module example.org/x\n\ngo 1.23\n", encoding="utf-8")
        (self.repo / "a.go").write_text(
            'package x\nimport (\n "example.org/x/dep"\n "os"\n)\n'
            'func A() { dep.B() }\nfunc Read() { _, _ = os.ReadFile("input") }\n',
            encoding="utf-8",
        )
        (self.repo / "helper.go").write_text("package x\nfunc Helper() {}\n", encoding="utf-8")
        (self.repo / "extra.go").write_text("package x\n// " + ("extra " * 150) + "\n", encoding="utf-8")
        (self.repo / "a_test.go").write_text(
            'package x\nimport "testing"\nfunc TestA(t *testing.T) {}\n', encoding="utf-8"
        )
        (self.repo / "dep").mkdir()
        (self.repo / "dep" / "dep.go").write_text("package dep\nfunc B() {}\n", encoding="utf-8")
        (self.repo / "other").mkdir()
        (self.repo / "other" / "other.go").write_text("package other\nfunc C() {}\n", encoding="utf-8")
        self.effects = self._effects()
        self.effects_bytes = json.dumps(self.effects, sort_keys=True).encode("utf-8")
        self.audit_records = self._audit()
        self.audit_bytes = b"".join(json.dumps(record, sort_keys=True).encode() + b"\n" for record in self.audit_records)
        self.effects_path = self.base / "effects.json"
        self.effects_path.write_bytes(self.effects_bytes)
        self.audit_path = self.base / "audit.jsonl"
        self.audit_path.write_bytes(self.audit_bytes)
        self.counter_script = self.base / "token-counter.py"
        self.counter_script.write_text(
            "import json, sys\n"
            "payload=json.load(sys.stdin)\n"
            "request=json.dumps(payload['request'], ensure_ascii=False, sort_keys=True, separators=(',', ':'))\n"
            "print(json.dumps({'input_tokens': (len(request)+3)//4, 'token_count_type':'estimate', 'model':'fake-model', 'context_window_tokens':payload['context_window_tokens'], 'max_output_tokens':32000}))\n",
            encoding="utf-8",
        )

    def token_counter_command(self) -> str:
        return json.dumps([sys.executable, str(self.counter_script)])

    def token_policy(self) -> dict[str, int]:
        return {"context_window_tokens": 200000, "output_reserve_tokens": 32000, "safety_margin_tokens": 30000}

    def investigate_run(self, run: pathlib.Path, command: str, **kwargs):
        return investigate(str(run), command, token_counter_command_json=self.token_counter_command(), token_policy=self.token_policy(), **kwargs)

    def aggregate_run(self, run: pathlib.Path, command: str | None = None, **kwargs):
        return aggregate(str(run), command, token_counter_command_json=self.token_counter_command(), token_policy=self.token_policy(), **kwargs)

    def verify_run(self, run: pathlib.Path, command: str | None = None, **kwargs):
        return verify(str(run), command, token_counter_command_json=self.token_counter_command(), token_policy=self.token_policy(), **kwargs)

    def benchmark_run(self, run: pathlib.Path, manifest: pathlib.Path, command: str | None = None, **kwargs):
        return benchmark(str(run), str(manifest), command, token_counter_command_json=self.token_counter_command(), token_policy=self.token_policy(), **kwargs)

    def _effects(self) -> dict:
        a = str(self.repo / "a.go")
        dep = str(self.repo / "dep" / "dep.go")
        other = str(self.repo / "other" / "other.go")
        return {
            "schema_version": 1,
            "groups": [
                {"effect": "fs.read", "sites": ["site:read"]},
                {"effect": "fs.write", "sites": [], "possible_sites": ["site:possible"]},
            ],
            "sites": [
                {
                    "id": "site:read",
                    "package": "example.org/x",
                    "function": "Read",
                    "kind": "call",
                    "location": {"file": a, "line": 7, "column": 34},
                    "effects": ["fs.read"],
                    "possible_effects": [],
                    "operations": [{"api": "os.ReadFile", "effects": ["fs.read"], "possible_effects": [], "uncertain": False}],
                    "uncertain": False,
                },
                {
                    "id": "site:possible",
                    "package": "example.org/x",
                    "function": "A",
                    "kind": "call",
                    "location": {"file": a, "line": 6, "column": 14},
                    "effects": [],
                    "possible_effects": ["fs.write"],
                    "operations": [{"api": "candidate", "effects": [], "possible_effects": ["fs.write"], "uncertain": True}],
                    "uncertain": True,
                },
            ],
            "unresolved_sites": ["site:unknown"],
            "reverse": [{"site_id": "site:read", "origins": ["node:read"], "edges": ["link:A-B"]}],
            "nodes": [
                {"id": "node:A", "package": "example.org/x", "function": "A", "variant": "x.A"},
                {"id": "node:B", "package": "example.org/x/dep", "function": "B", "variant": "dep.B"},
                {"id": "node:read", "package": "example.org/x", "function": "Read", "variant": "x.Read"},
            ],
            "edges": [
                {
                    "id": "link:A-B",
                    "caller": "node:A",
                    "callee": "node:B",
                    "source": {
                        "id": "edge:A-B",
                        "caller": {"package": "example.org/x", "function": "A"},
                        "callee": {"package": "example.org/x/dep", "function": "B"},
                        "location": {"file": a, "line": 6, "column": 14},
                        "kind": "call",
                        "resolution": "candidate_vta",
                    },
                    "candidate": True,
                    "synthetic": False,
                },
                {
                    "id": "link:B-C",
                    "caller": "node:B",
                    "callee": "node:C",
                    "source": {
                        "id": "edge:B-C",
                        "caller": {"package": "example.org/x/dep", "function": "B"},
                        "callee": {"package": "example.org/x/other", "function": "C"},
                        "location": {"file": dep, "line": 2, "column": 16},
                        "kind": "call",
                        "resolution": "established",
                    },
                    "candidate": False,
                    "synthetic": False,
                },
                {
                    "id": "link:synthetic",
                    "caller": "node:A",
                    "callee": "node:read",
                    "source": {"id": "edge:synthetic", "caller": {"package": "example.org/x", "function": "A"}, "callee": {"package": "example.org/x", "function": "Read"}},
                    "candidate": False,
                    "synthetic": True,
                },
            ],
            "coverage": {
                "packages_selected": 3,
                "source_functions": 5,
                "emitted": 5,
                "omitted": 0,
                "packages": ["example.org/x", "example.org/x/dep", "example.org/x/other"],
            },
            "inventory": [],
            "truncated": {"sites": False, "edges": False, "depth": False},
        }

    def _audit(self) -> list[dict]:
        a = str(self.repo / "a.go")
        dep = str(self.repo / "dep" / "dep.go")
        other = str(self.repo / "other" / "other.go")
        return [
            {
                "record_type": "site", "schema_version": 1, "id": "site:read",
                "package": "example.org/x", "function": "Read", "kind": "call",
                "location": {"file": a, "line": 7, "column": 34}, "effects": ["fs.read"],
                "possible_effects": [], "uncertain": False, "vta_candidates": [],
            },
            {
                "record_type": "site", "schema_version": 1, "id": "site:possible",
                "package": "example.org/x", "function": "A", "kind": "call",
                "location": {"file": a, "line": 6, "column": 14}, "effects": [],
                "possible_effects": ["fs.write"], "uncertain": True,
                "vta_candidates": [{"id": "node:B", "effect_propagation_supported": True}],
            },
            {
                "record_type": "function", "schema_version": 1,
                "package": "example.org/x", "function": "Read", "effects": ["fs.read"],
                "possible_effects": [], "uncertain": False,
                "locations": [{"file": a, "line": 7, "column": 6}],
            },
            {
                "record_type": "function", "schema_version": 1,
                "package": "example.org/x/dep", "function": "B", "effects": [],
                "possible_effects": [], "uncertain": False,
                "locations": [{"file": dep, "line": 2, "column": 6}],
            },
            {
                "record_type": "function", "schema_version": 1,
                "package": "example.org/x/other", "function": "C", "effects": [],
                "possible_effects": [], "uncertain": False,
                "locations": [{"file": other, "line": 2, "column": 6}],
            },
        ]

    def args(self, out: pathlib.Path, **overrides) -> SimpleNamespace:
        values = {
            "repo": str(self.repo),
            "out": str(out),
            "reffector": "missing-reffector-for-import",
            "effects": str(self.effects_path),
            "audit": str(self.audit_path),
            "pattern": ["./..."],
            "max_sites": 50000,
            "max_edges": 200000,
            "max_depth": 100,
            "max_context_bytes": 50000,
            "max_retrieval_rounds": 2,
            "max_retrieval_bytes": 20000,
            "token_counter_command_json": self.token_counter_command(),
            "context_window_tokens": 200000,
            "output_reserve_tokens": 32000,
            "safety_margin_tokens": 30000,
        }
        values.update(overrides)
        return SimpleNamespace(**values)

    def prepare_import(self, name: str = "run", **overrides) -> pathlib.Path:
        output = self.base / name
        prepare(self.args(output, **overrides))
        return output

    def _write_model(self, behavior: str, log_path: pathlib.Path | None = None) -> pathlib.Path:
        script = self.base / f"model_{behavior}.py"
        log_argument = repr(str(log_path)) if log_path else "None"
        script.write_text(
            "import json, pathlib, sys\n"
            "request=json.load(sys.stdin)\n"
            f"log_path={log_argument}\n"
            "if log_path:\n"
            " with open(log_path, 'a', encoding='utf-8') as stream: stream.write(json.dumps(request)+'\\n')\n"
            f"behavior={behavior!r}\n"
            "phase=request.get('phase')\n"
            "claims=[]\n"
            "need_sources=[]\n"
            "if behavior == 'retrieval' and phase == 'component' and request.get('component',{}).get('package') == 'example.org/x' and request.get('component',{}).get('source_files') == ['a.go'] and any(item.get('name') == 'A' for item in request.get('program',{}).get('declarations',[])):\n"
            " need_sources=['helper.go']\n"
            "if behavior == 'escape' and phase == 'component' and request.get('component',{}).get('package') == 'example.org/x':\n"
            " need_sources=['../../outside.go','other/other.go']\n"
            "need_evidence=[]\n"
            "if behavior == 'evidence_retrieval' and phase == 'component' and request.get('component',{}).get('package') == 'example.org/x':\n"
            " need_evidence=['call-edge:link:A-B']\n"
            "if behavior == 'evidence_escape' and phase == 'component' and request.get('component',{}).get('package') == 'example.org/x':\n"
            " need_evidence=['call-edge:../../outside']\n"
            "if behavior == 'retrieval' and phase == 'component_followup':\n"
            " claims=[{'id':'x-flow','statement':'A calls the dependency function','category':'established','domain':'relationship','evidence':[{'kind':'source','file':'a.go','line':6,'quote':'func A() { dep.B() }'}]}]\n"
            "if behavior == 'allphase' and phase == 'component' and request.get('component',{}).get('package') == 'example.org/x':\n"
            " claims=[{'id':'component-x','statement':'Package x calls the dependency','category':'possible','domain':'relationship','evidence':[{'kind':'source','file':'a.go','line':6,'quote':'func A() { dep.B() }'}]}]\n"
            "if behavior in ('benchmark','allphase') and phase == 'benchmark':\n"
            " claims=[{'id':'benchmark-fact','statement':'The cache uses downloaders and writeback','category':'established','domain':'architecture','evidence':[{'kind':'source','file':'a.go','line':1,'quote':'package x'}]}]\n"
            "response={'claims':claims,'need_sources':need_sources,'model':'fake-model'}\n"
            "if need_evidence: response['need_evidence']=need_evidence\n"
            "if phase == 'aggregate':\n"
            " children=request.get('children',[])\n"
            " child_ids=[claim['id'] for child in children for claim in child.get('claims',[])]\n"
            " evidence=[]\n"
            " for child in children:\n"
            "  for claim in child.get('claims',[]):\n"
            "   if claim.get('evidence'): evidence=claim['evidence']; break\n"
            "  if evidence: break\n"
            " aggclaims=[] if not child_ids else [{'id':'agg-'+request['aggregate_id'],'statement':'summarized subsystem','category':('established' if behavior=='upgrade' else 'possible'),'domain':'architecture','evidence':evidence,'child_claim_ids':child_ids}]\n"
            " response={'claims':aggclaims,'model':'fake-model'}\n"
            "if phase == 'verify': response={'verdicts':[{'claim_id':request['claim']['id'],'verdict':'supported','reason':'cited lines match'}],'model':'fake-model'}\n"
            "if phase == 'benchmark':\n"
            " response={'claims':claims,'model':'fake-model','usage':{'input_tokens':100,'output_tokens':20}}\n"
            " if request.get('mode') == 'ordinary_source_analysis': response['cost_usd']=0.001\n"
            "print(json.dumps(response))\n",
            encoding="utf-8",
        )
        return script

    def model_command(self, script: pathlib.Path) -> str:
        return json.dumps([sys.executable, str(script)])

    def test_import_preserves_raw_inputs_and_scopes_packets(self) -> None:
        run = self.prepare_import()
        self.assertEqual((run / "raw-effects.json").read_bytes(), self.effects_bytes)
        self.assertEqual((run / "raw-audit.jsonl").read_bytes(), self.audit_bytes)
        metadata = json.loads((run / "metadata.json").read_text())
        self.assertEqual(metadata["status"], "complete")
        self.assertIn("git_available", metadata["revision"])
        inventory = json.loads((run / "inventory.json").read_text())
        self.assertEqual(len(inventory["packages"]), 3)
        self.assertEqual(set(json.loads((run / "source-manifest.json").read_text())["files"]), {
            "a.go", "helper.go", "extra.go", "a_test.go", "dep/dep.go", "other/other.go"
        })
        packets = [json.loads(path.read_text()) for path in (run / "packets").glob("*.json")]
        x_packet = next(packet for packet in packets if packet["package"] == "example.org/x")
        request = x_packet["request"]
        self.assertNotIn("effects", request)
        self.assertEqual({item["path"] for item in request["source"]}, {"a.go", "helper.go", "extra.go", "a_test.go"})
        self.assertEqual({edge["id"] for edge in request["evidence"]["call_relationships"]}, {"link:A-B"})
        self.assertEqual({node["id"] for node in request["evidence"]["nodes"]}, {"node:A", "node:B", "node:read"})
        canonical = json.loads((run / "evidence-index.json").read_text())
        evidence_by_ref = {item["ref"]: item for item in canonical["records"]}
        audit_site = evidence_by_ref["audit-site:site:possible"]["data"]
        self.assertEqual(audit_site["candidate_records"][0]["id"], "node:B")
        reverse = evidence_by_ref["reverse:site:read"]["data"]
        self.assertEqual(reverse["origins"], ["node:read"])
        self.assertEqual(reverse["edges"], ["link:A-B"])
        unresolved_ref = next(ref for ref in request["evidence"]["available_evidence_refs"] if ref.startswith("unresolved-site:"))
        self.assertEqual(evidence_by_ref[unresolved_ref]["data"]["id"], "site:unknown")
        self.assertEqual(request["evidence"]["unresolved_site_records"][0]["id"], "site:unknown")
        self.assertEqual(x_packet["request_bytes"], request_size(request))
        self.assertLessEqual(x_packet["request_bytes"], x_packet["max_context_bytes"])
        global_edges = json.loads((run / "relationships.json").read_text())["effect_site_reverse_edges"]
        self.assertEqual({edge["id"] for edge in global_edges}, {"link:A-B", "link:B-C", "link:synthetic"})
        self.assertTrue(all(edge["scope"].startswith("effect-site reverse") for edge in global_edges))

    def test_token_overflow_partitions_by_declaration_without_dropping_scope(self) -> None:
        counter = self.base / "partition-counter.py"
        counter.write_text(
            "import json, sys\n"
            "payload=json.load(sys.stdin)\n"
            "request=payload['request']\n"
            "tokens=50000 if request.get('unit_kind') == 'package' else 1000\n"
            "print(json.dumps({'input_tokens':tokens,'token_count_type':'exact','model':'fake-model','context_window_tokens':payload['context_window_tokens'],'max_output_tokens':32000}))\n",
            encoding="utf-8",
        )
        run = self.prepare_import(
            "token-partition-run", max_context_bytes=50000,
            token_counter_command_json=json.dumps([sys.executable, str(counter)]),
            context_window_tokens=100000, output_reserve_tokens=32000, safety_margin_tokens=30000,
        )
        packets = [json.loads(path.read_text()) for path in (run / "packets").glob("*.json")]
        x_packets = [packet for packet in packets if packet["package"] == "example.org/x"]
        self.assertTrue(x_packets)
        self.assertTrue(all(packet["unit_kind"] == "symbol_partition" for packet in x_packets))
        self.assertTrue(all(packet["status"] == "pending" for packet in x_packets))
        expected = {
            item["id"] for item in json.loads((run / "program-model.json").read_text())["declarations"]
            if item.get("package") == "example.org/x"
        }
        actual = {
            item["id"] for packet in x_packets for item in packet["request"]["program"]["declarations"]
        }
        self.assertEqual(actual, expected)
        refs = {ref for packet in x_packets for ref in packet["request"]["evidence"]["available_evidence_refs"]}
        self.assertIn("effect-site:site:read", refs)
        self.assertIn("audit-site:site:read", refs)
        self.assertTrue(all(packet["token_budget"]["status"] == "within_budget" for packet in x_packets))
        self.assertTrue((run / "evidence-index.json").exists())

    def test_concurrency_is_a_supported_claim_domain_with_source_evidence(self) -> None:
        claim = {
            "id": "concurrency-uncertainty", "statement": "The source does not establish whether A executes concurrently.",
            "category": "unknown", "domain": "concurrency",
            "evidence": [{"kind": "source", "file": "a.go", "line": 6, "quote": "func A() { dep.B() }"}],
        }
        self.assertTrue(valid_claim_shape(claim))
        manifest = json.loads((self.prepare_import("concurrency-domain-run") / "source-manifest.json").read_text())["files"]
        self.assertEqual(claim_evidence_issues(claim, index_evidence(self.base / "concurrency-domain-run"), self.repo, manifest), [])

    def test_prepare_commands_are_offline_and_audit_gets_no_graph_caps(self) -> None:
        fake_binary = self.base / "fake-reffector"
        fake_binary.write_text("not executed while subprocess is patched\n", encoding="utf-8")
        fake_binary.chmod(0o700)
        output = self.base / "command-run"
        package_data = [
            {"ImportPath": "example.org/x", "Dir": str(self.repo), "GoFiles": ["a.go", "helper.go"], "TestGoFiles": ["a_test.go"], "Imports": ["example.org/x/dep"]},
            {"ImportPath": "example.org/x/dep", "Dir": str(self.repo / "dep"), "GoFiles": ["dep.go"], "Imports": []},
            {"ImportPath": "example.org/x/other", "Dir": str(self.repo / "other"), "GoFiles": ["other.go"], "Imports": []},
        ]
        list_bytes = "\n".join(json.dumps(record) for record in package_data).encode()
        calls = []

        def fake_run(argv, cwd, env, timeout):
            calls.append((argv, env))
            if argv[0] == str(fake_binary):
                stdout = self.audit_bytes if "-audit-unresolved" in argv else self.effects_bytes
            elif argv[:2] == ["go", "version"]:
                stdout = b"go version go1.27.2 darwin/arm64\n"
            elif argv[:2] == ["go", "env"]:
                stdout = b"go1.27.2\ndarwin\narm64\n1\n"
            elif argv[:3] == ["go", "list", "-json"]:
                stdout = list_bytes
            else:
                self.fail(f"unexpected command: {argv}")
            return {"returncode": 0, "stdout": stdout, "stderr": b"", "elapsed_seconds": 0.01, "timed_out": False}

        args = self.args(
            output,
            reffector=str(fake_binary),
            effects=None,
            audit=None,
            pattern=["./...", "./internal/..."],
        )
        with patch.dict(os.environ, {"API_TOKEN": "test-secret-must-not-pass"}):
            with patch("tools.archchar.core.run_process", side_effect=fake_run):
                prepare(args)
        effects_call = next(argv for argv, _ in calls if argv[0] == str(fake_binary) and "-json" in argv)
        audit_call = next(argv for argv, _ in calls if argv[0] == str(fake_binary) and "-audit-unresolved" in argv)
        go_list_call = next(argv for argv, _ in calls if argv[:3] == ["go", "list", "-json"])
        self.assertIn("-max-sites", effects_call)
        self.assertIn("-max-edges", effects_call)
        self.assertIn("-max-depth", effects_call)
        self.assertNotIn("-max-sites", audit_call)
        self.assertNotIn("-max-edges", audit_call)
        self.assertNotIn("-max-depth", audit_call)
        self.assertEqual(effects_call[-2:], ["./...", "./internal/..."])
        self.assertEqual(audit_call[-2:], ["./...", "./internal/..."])
        self.assertEqual(go_list_call[-2:], ["./...", "./internal/..."])
        for _argv, env in calls:
            self.assertEqual(env["GOENV"], "off")
            self.assertEqual(env["GOPROXY"], "off")
            self.assertEqual(env["GOSUMDB"], "off")
            self.assertEqual(env["GOFLAGS"], "-mod=readonly")
            self.assertEqual(env["GOWORK"], "off")
            self.assertNotIn("API_TOKEN", env)
            self.assertTrue(pathlib.Path(env["TMPDIR"]).resolve().is_relative_to(output.resolve()))
        metadata = json.loads((output / "metadata.json").read_text())
        self.assertEqual(metadata["status"], "complete")
        self.assertEqual((output / "raw-effects.json").read_bytes(), self.effects_bytes)
        self.assertEqual((output / "raw-audit.jsonl").read_bytes(), self.audit_bytes)

    def test_invalid_effect_schema_is_preserved_and_marks_run_partial(self) -> None:
        malformed = self.base / "malformed-effects.json"
        malformed.write_bytes(b"{not-json")
        run = self.prepare_import("malformed-run", effects=str(malformed))
        metadata = json.loads((run / "metadata.json").read_text())
        self.assertEqual(metadata["status"], "partial")
        self.assertTrue(any("could not be parsed" in error for error in metadata["errors"]))
        self.assertEqual((run / "raw-effects.json").read_bytes(), b"{not-json")

    def test_malformed_effect_coverage_is_preserved_and_marks_run_partial(self) -> None:
        malformed_effects = self._effects()
        malformed_effects["coverage"] = []
        path = self.base / "malformed-coverage.json"
        raw = json.dumps(malformed_effects, sort_keys=True).encode("utf-8")
        path.write_bytes(raw)
        run = self.prepare_import("malformed-coverage-run", effects=str(path))
        metadata = json.loads((run / "metadata.json").read_text())
        inventory = json.loads((run / "inventory.json").read_text())
        self.assertEqual(metadata["status"], "partial")
        self.assertTrue(any("coverage must be an object" in error for error in metadata["errors"]))
        self.assertEqual((run / "raw-effects.json").read_bytes(), raw)
        self.assertEqual(inventory["coverage"], {})

    def test_imported_evidence_must_match_go_list_package_scope(self) -> None:
        run = self.prepare_import("scope-mismatch", pattern=["./dep"])
        metadata = json.loads((run / "metadata.json").read_text())
        self.assertEqual(metadata["status"], "partial")
        self.assertEqual(metadata["evidence_scope"]["status"], "mismatch")
        self.assertTrue(any("package coverage" in error for error in metadata["errors"]))

    def test_output_inside_analyzed_repo_is_rejected(self) -> None:
        output = self.repo / "archchar-output"
        with self.assertRaisesRegex(Exception, "outside the analyzed repository"):
            prepare(self.args(output))
        self.assertFalse(output.exists())

    def test_over_budget_source_is_retained_and_never_truncated(self) -> None:
        large = self.repo / "large.go"
        large.write_text("package x\n// " + ("x" * 8000) + "\n", encoding="utf-8")
        run = self.prepare_import("budget-run", max_context_bytes=2500)
        manifest = json.loads((run / "source-manifest.json").read_text())["files"]
        self.assertIn("large.go", manifest)
        packets = [json.loads(path.read_text()) for path in (run / "packets").glob("*.json")]
        large_packet = next(packet for packet in packets if packet["source_files"] == ["large.go"])
        self.assertEqual(large_packet["status"], "over_budget")
        self.assertGreater(large_packet["request_bytes"], large_packet["max_context_bytes"])
        self.assertEqual(len(large_packet["request"]["source"][0]["text"]), large.stat().st_size)
        relationships = json.loads((run / "relationships.json").read_text())
        self.assertEqual(len(relationships["effect_site_reverse_edges"]), 3)

    def test_retrieval_is_limited_to_the_same_file_unit_package_and_resumes(self) -> None:
        run = self.prepare_import("retrieval-run", max_context_bytes=13000)
        log = self.base / "model-requests.jsonl"
        command = self.model_command(self._write_model("retrieval", log))
        summary = self.investigate_run(run, command)
        self.assertEqual(summary["status"], "complete")
        requests = [json.loads(line) for line in log.read_text().splitlines()]
        x_requests = [
            request for request in requests
            if request.get("component", {}).get("package") == "example.org/x"
            and request.get("component", {}).get("source_files") == ["a.go"]
            and any(item.get("name") == "A" for item in request.get("program", {}).get("declarations", []))
        ]
        self.assertEqual([request["phase"] for request in x_requests], ["component", "component_followup"])
        self.assertEqual({source["path"] for source in x_requests[0]["source"]}, {"a.go"})
        self.assertEqual(x_requests[0]["allowed_source_package"], "example.org/x")
        self.assertEqual([source["path"] for source in x_requests[1]["additional_sources"]], ["helper.go"])
        component = next(
            json.loads(path.read_text()) for path in (run / "components").glob("*.json")
            if json.loads(path.read_text()).get("retrieved_sources")
        )
        self.assertEqual(component["status"], "complete")
        self.assertEqual(component["retrieved_sources"][0]["path"], "helper.go")
        before = len(requests)
        self.investigate_run(run, command)
        after = len(log.read_text().splitlines())
        self.assertEqual(after, before)

    def test_investigation_stops_when_source_changes(self) -> None:
        run = self.prepare_import("stale-run")
        log = self.base / "stale-requests.jsonl"
        command = self.model_command(self._write_model("retrieval", log))
        (self.repo / "a.go").write_text((self.repo / "a.go").read_text() + "// changed\n", encoding="utf-8")
        summary = self.investigate_run(run, command)
        self.assertEqual(summary["status"], "partial")
        self.assertIn("a.go", summary["source_scope_changed"])
        self.assertFalse(log.exists())

    def test_retrieval_rejects_path_escape_and_unrelated_package(self) -> None:
        run = self.prepare_import("escape-run")
        script = self._write_model("escape")
        summary = self.investigate_run(run, self.model_command(script))
        self.assertEqual(summary["status"], "partial")
        x_result = next(json.loads(path.read_text()) for path in (run / "components").glob("*.json") if json.loads(path.read_text()).get("package") == "example.org/x" and json.loads(path.read_text()).get("unresolved_requests"))
        self.assertEqual(x_result["retrieved_sources"], [])
        self.assertTrue(any("out-of-scope" in item for item in x_result["unresolved_requests"]))

    def test_need_evidence_retrieves_only_allowlisted_canonical_records(self) -> None:
        run = self.prepare_import("evidence-retrieval-run")
        log = self.base / "evidence-requests.jsonl"
        summary = self.investigate_run(run, self.model_command(self._write_model("evidence_retrieval", log)))
        self.assertEqual(summary["status"], "complete")
        requests = [json.loads(line) for line in log.read_text().splitlines()]
        x_requests = [request for request in requests if request.get("component", {}).get("package") == "example.org/x"]
        followup = next(request for request in x_requests if request.get("phase") == "component_followup")
        detail = followup["additional_evidence"][0]
        self.assertEqual(detail["ref"], "call-edge:link:A-B")
        self.assertEqual(detail["record"]["source_edge_id"], "edge:A-B")
        result = next(json.loads(path.read_text()) for path in (run / "components").glob("*.json") if json.loads(path.read_text()).get("package") == "example.org/x")
        self.assertEqual(result["retrieved_evidence_refs"], ["call-edge:link:A-B"])

    def test_need_evidence_rejects_unknown_refs_without_disclosure(self) -> None:
        run = self.prepare_import("evidence-escape-run")
        summary = self.investigate_run(run, self.model_command(self._write_model("evidence_escape")))
        self.assertEqual(summary["status"], "partial")
        result = next(json.loads(path.read_text()) for path in (run / "components").glob("*.json") if json.loads(path.read_text()).get("package") == "example.org/x")
        self.assertEqual(result["retrieved_evidence_refs"], [])
        self.assertTrue(any("out-of-scope evidence" in item for item in result["unresolved_requests"]))

    def test_confidence_verifier_rejects_candidate_effect_and_runtime_claims(self) -> None:
        run = self.prepare_import("verify-run")
        packets = [json.loads(path.read_text()) for path in (run / "packets").glob("*.json")]
        for packet in packets:
            claims = []
            if packet["package"] == "example.org/x":
                claims = [
                    {
                        "id": "good-read",
                        "statement": "The read operation is fs.read",
                        "category": "established",
                        "domain": "effect",
                        "evidence": [
                            {"kind": "source", "file": "a.go", "line": 7, "quote": 'func Read() { _, _ = os.ReadFile("input") }'},
                            {"kind": "reffector", "id": "site:read", "field": "effects"},
                        ],
                    },
                    {
                        "id": "promoted-write",
                        "statement": "This call establishes fs.write",
                        "category": "established",
                        "domain": "effect",
                        "evidence": [{"kind": "reffector", "id": "site:possible", "field": "possible_effects"}],
                    },
                    {
                        "id": "candidate-call",
                        "statement": "A definitely calls B",
                        "category": "established",
                        "domain": "relationship",
                        "evidence": [{"kind": "reffector", "id": "edge:A-B", "field": "resolution"}],
                    },
                    {
                        "id": "runtime-claim",
                        "statement": "The callback executes at runtime and completes",
                        "category": "established",
                        "domain": "runtime",
                        "evidence": [{"kind": "source", "file": "a.go", "line": 6, "quote": "func A() { dep.B() }"}],
                    },
                ]
            write_json(run / "components" / f"{packet['unit_key']}.json", {"unit_id": packet["unit_id"], "package": packet["package"], "status": "complete", "claims": claims})
        self.aggregate_run(run)
        report = self.verify_run(run)
        self.assertEqual(report["status"], "partial")
        details = "\n".join(issue["detail"] for issue in report["issues"])
        self.assertIn("possible fields", details)
        self.assertIn("candidate edges", details)
        self.assertIn("runtime execution", details)
        self.assertTrue(next(item for item in report["claims"] if item["claim_id"] == "good-read")["valid"])

    def test_hierarchy_aggregation_and_conflicts_preserve_children(self) -> None:
        run = self.prepare_import("aggregate-run")
        packets = [json.loads(path.read_text()) for path in (run / "packets").glob("*.json")]
        for packet in packets:
            claims = []
            if packet["package"] in ("example.org/x", "example.org/x/dep"):
                path = "a.go" if packet["package"] == "example.org/x" else "dep/dep.go"
                quote = "package x" if path == "a.go" else "package dep"
                claims = [{"id": "shared", "statement": f"{packet['package']} responsibility", "category": "possible", "domain": "architecture", "evidence": [{"kind": "source", "file": path, "line": 1, "quote": quote}]}]
            write_json(run / "components" / f"{packet['unit_key']}.json", {"unit_id": packet["unit_id"], "package": packet["package"], "status": "complete", "claims": claims})
        structural = self.aggregate_run(run)
        self.assertEqual(structural["status"], "partial")
        self.assertEqual(structural["semantic_aggregation"], "not_run")
        self.assertTrue(structural["conflicts"])
        self.assertIn("example.org/x", structural["hierarchy"]["namespace_groups"])
        self.assertGreaterEqual(len(structural["claims"]), 2)

    def test_model_aggregation_confidence_upgrade_is_detected(self) -> None:
        run = self.prepare_import("upgrade-run")
        packets = [json.loads(path.read_text()) for path in (run / "packets").glob("*.json")]
        for packet in packets:
            claims = []
            if packet["package"] == "example.org/x":
                claims = [{"id": "leaf-possible", "statement": "A possible responsibility", "category": "possible", "domain": "architecture", "evidence": [{"kind": "source", "file": "a.go", "line": 1, "quote": "package x"}]}]
            write_json(run / "components" / f"{packet['unit_key']}.json", {"unit_id": packet["unit_id"], "package": packet["package"], "status": "complete", "claims": claims})
        command = self.model_command(self._write_model("upgrade"))
        result = self.aggregate_run(run, command)
        self.assertEqual(result["semantic_aggregation"], "model")
        report = self.verify_run(run)
        self.assertTrue(any(issue["kind"] == "confidence_upgrade" for issue in report["issues"]))

    def test_benchmark_context_defers_unreferenced_details_but_keeps_claim_evidence(self) -> None:
        run = self.prepare_import("compact-benchmark-run")
        inventory = json.loads((run / "inventory.json").read_text())
        selected = set(json.loads((run / "source-manifest.json").read_text())["files"])
        component_path = next((run / "components").glob("*.json"))
        component = json.loads(component_path.read_text())
        component.update({
            "status": "complete",
            "claims": [{
                "id": "read-claim", "statement": "Read performs a filesystem read.",
                "category": "established", "domain": "effect",
                "evidence": [{"kind": "reffector", "id": "site:read", "field": "effects"}],
            }],
        })
        component_path.write_text(json.dumps(component), encoding="utf-8")

        evidence = benchmark_evidence(run, inventory, selected)
        self.assertNotIn("site_evidence", evidence)
        self.assertNotIn("call_relationships", evidence)
        self.assertGreater(evidence["site_evidence_details_deferred"], 0)
        cited = next(item for item in evidence["referenced_records"] if item["id"] == "site:read")
        self.assertEqual(cited["data"]["effects"], ["fs.read"])
        self.assertEqual(evidence["deferred_evidence_ref_count"], evidence["evidence_index_record_count"] - evidence["referenced_record_count"])
        treatment = benchmark_treatment_context(run, inventory, selected)
        self.assertEqual(treatment["reffector"]["referenced_records"], evidence["referenced_records"])
        self.assertEqual({item["path"] for item in treatment["program"]["files"]}, selected)
        self.assertNotIn("declarations", treatment["program"])
        self.assertEqual(treatment["program"]["declaration_counts"]["total"], len(json.loads((run / "program-model.json").read_text())["declarations"]))

    def test_paired_benchmark_same_inputs_rubric_and_unknown_cost(self) -> None:
        run = self.prepare_import("benchmark-run")
        self.aggregate_run(run)
        manifest = self.base / "benchmark.json"
        manifest.write_text(json.dumps({
            "schema_version": 1,
            "name": "test-pair",
            "questions": [{"id": "q1", "prompt": "Describe the source responsibilities."}],
            "rubric": {"required_facts": [{"id": "fact", "all_terms": ["downloaders", "writeback"]}]},
        }), encoding="utf-8")
        preflight_command = self.model_command(self._write_model("benchmark"))
        self.assertEqual(self.investigate_run(run, preflight_command)["status"], "complete")
        self.assertEqual(self.aggregate_run(run)["status"], "structural_only")
        self.assertEqual(self.verify_run(run)["status"], "complete")
        log = self.base / "benchmark-requests.jsonl"
        command = self.model_command(self._write_model("benchmark", log))
        result = self.benchmark_run(run, manifest, command, max_context_bytes=50000, reasoning_level="low", max_output_tokens=700)
        self.assertEqual(result["status"], "complete")
        self.assertTrue(result["paired_inputs"]["same_source_scope"])
        self.assertEqual(result["paired_inputs"]["question_sha256"], result["question_sha256"])
        requests = [json.loads(line) for line in log.read_text().splitlines()]
        self.assertEqual(len(requests), 2)
        self.assertEqual(requests[0]["questions"], requests[1]["questions"])
        self.assertEqual(requests[0]["source"], requests[1]["source"])
        self.assertEqual(requests[0]["source_preparation"], requests[1]["source_preparation"])
        self.assertIsNone(requests[0]["condition_data"])
        self.assertEqual(requests[0]["source_preparation"]["source_coverage"]["files"], len(requests[0]["source"]))
        self.assertTrue(requests[0]["source_preparation"]["canonical_index"] == "program-structure.json")
        forbidden = {"effect", "effects", "possible_effects", "uncertain", "call_edge", "call-edge", "cross_component_relationships"}
        def keys(value):
            if isinstance(value, dict):
                return set(value) | set().union(*(keys(child) for child in value.values()))
            if isinstance(value, list):
                return set().union(*(keys(child) for child in value), set())
            return set()
        self.assertFalse(forbidden & keys(requests[0]))
        self.assertIsNotNone(requests[1]["condition_data"]["reffector"])
        self.assertNotIn("required_facts", requests[0])
        self.assertTrue(result["scores_blind_then_mapped"]["source_only"]["required_facts"][0]["hit"])
        self.assertEqual(result["scores_blind_then_mapped"]["source_only"]["valid_citation_ratio"], 1.0)
        self.assertFalse(result["scores_blind_then_mapped"]["source_only"]["semantic_accuracy_scored"])
        self.assertIsNotNone(result["metrics"]["source_only"]["cost_usd"])
        self.assertIsNone(result["metrics"]["treatment_final_call"]["cost_usd"])
        self.assertIsNone(result["metrics"]["treatment_investigation_aggregation_verification_and_final"]["cost_usd"])
        self.benchmark_run(run, manifest, command, max_context_bytes=50000, reasoning_level="low", max_output_tokens=700)
        self.assertEqual(len(log.read_text().splitlines()), 2)

    def test_rubric_fact_hits_require_a_matching_configured_citation(self) -> None:
        run = self.prepare_import("citation-run")
        manifest = json.loads((run / "source-manifest.json").read_text())
        rubric = {
            "required_facts": [{
                "id": "fact",
                "all_terms": ["downloaders", "writeback"],
                "citation_options": [{"kind": "source", "file": "a.go", "line": 6}],
            }]
        }
        claim = {
            "id": "fact-claim",
            "statement": "This uses downloaders and writeback.",
            "category": "established",
            "domain": "architecture",
            "evidence": [{"kind": "source", "file": "a.go", "line": 1, "quote": "package x"}],
        }
        evidence_index = index_evidence(run)
        unsupported_citation = claim_score([claim], rubric, evidence_index, self.repo, manifest["files"])
        self.assertFalse(unsupported_citation["required_facts"][0]["hit"])
        claim["evidence"] = [{"kind": "source", "file": "a.go", "line": 6, "quote": "func A() { dep.B() }"}]
        matching_citation = claim_score([claim], rubric, evidence_index, self.repo, manifest["files"])
        self.assertTrue(matching_citation["required_facts"][0]["hit"])

    def test_over_budget_benchmark_reports_lossless_source_partitions(self) -> None:
        run = self.prepare_import("partition-plan-run")
        manifest = self.base / "partition-plan.json"
        manifest.write_text(json.dumps({
            "schema_version": 1,
            "questions": [{"id": "q", "prompt": "Describe source."}],
            "rubric": {"required_facts": [{"id": "f", "all_terms": ["source"]}]},
        }), encoding="utf-8")
        report = benchmark(str(run), str(manifest), None, max_context_bytes=1)
        arm = report["arms"]["source_only"]
        self.assertEqual(arm["status"], "over_budget")
        partitions = arm["source_partitions"]
        expected = set(json.loads((run / "source-manifest.json").read_text())["files"])
        self.assertEqual({item["file"] for item in partitions}, expected)
        self.assertTrue(all(item["request"]["source"][0]["path"] == item["file"] for item in partitions))
        self.assertTrue(all(item["request"]["source_preparation"]["source_relationship_directory"] == partitions[0]["request"]["source_preparation"]["source_relationship_directory"] for item in partitions))
        self.assertTrue(report["call_economics"]["no_generation_performed"])
        self.assertEqual(report["call_economics"]["status"], "dry_run_plan_only")

    def test_one_shot_methodology_is_explicit_and_does_not_require_prepared_claims(self) -> None:
        run = self.prepare_import("one-shot-run")
        manifest = self.base / "one-shot.json"
        manifest.write_text(json.dumps({
            "schema_version": 1, "questions": [{"id": "q", "prompt": "Describe source."}],
            "rubric": {"required_facts": [{"id": "f", "all_terms": ["source"]}]},
        }), encoding="utf-8")
        report = benchmark(str(run), str(manifest), None, methodology="one_shot_paired_v1")
        self.assertEqual(report["methodology"], "one_shot_paired_v1")
        self.assertEqual(report["status"], "pending")
        self.assertEqual(report["call_economics"]["conditional_additional_calls"], {
            "retrieval": "none; complete source is supplied in the paired request",
            "semantic_aggregation": "0 calls; partition claims are concatenated locally and deterministically",
            "model_verification": "none; only deterministic claim scoring is performed",
        })
        self.assertFalse(report["treatment_phase_validation"]["complete"])
        self.assertNotEqual(report["arms"]["source_only"]["request"]["mode"], "ordinary_source_analysis")
        self.assertIsNone(report["arms"]["source_only"]["request"]["condition_data"])
        treatment_data = report["arms"]["reffector_treatment"]["request"]["condition_data"]
        self.assertEqual(set(treatment_data), {"reffector", "selection"})
        self.assertTrue(treatment_data["reffector"]["records_by_file"])
        self.assertEqual(treatment_data["selection"]["unit"]["id"], "rclone-vfs-cache-subsystem")
        self.assertEqual(len(treatment_data["selection"]["per_question"]), 1)
        self.assertEqual(treatment_data["selection"]["semantic_sufficiency"], "not_established")
        self.assertIn("deferred_by_record_type", report["selection_audit"][0]["coverage"])
        selected_refs = {item["ref"] for question in report["selection_audit"] for item in question["selected"]}
        retrieval = treatment_data["selection"]["retrieval"]
        self.assertEqual(retrieval["retrieved_refs"], sorted(selected_refs))
        self.assertTrue(retrieval["log"])
        packed_refs = {record[0] for group in treatment_data["reffector"]["records_by_file"]
                       for kind, records in group.items() if kind in treatment_data["reffector"]["field_order"]
                       for record in records}
        self.assertEqual(selected_refs, packed_refs)
        self.assertEqual(report["selection_audit"][0]["source_declarations"][0]["line_start"], 6)
        shared = report["arms"]["source_only"]["request"]["source_preparation"]
        self.assertEqual(shared["canonical_index"], "program-structure.json")
        self.assertTrue(all(item.get("source_sha256") for item in shared["files"]))
        self.assertTrue(all(isinstance(item.get("declaration_refs"), list) for item in shared["files"]))
        log = self.base / "one-shot-requests.jsonl"
        fake = self.model_command(self._write_model("benchmark", log))
        counter_log = self.base / "whole-fit-counts.log"
        counter = self.base / "whole-fit-counter.py"
        counter.write_text(
            "import json, pathlib, sys\np=json.load(sys.stdin)\n"
            f"pathlib.Path({str(counter_log)!r}).open('a').write('count\\n')\n"
            "print(json.dumps({'input_tokens':100,'token_count_type':'exact','model':'fake-model','context_window_tokens':200000,'max_output_tokens':32000}))\n",
            encoding="utf-8",
        )
        executed = benchmark(str(run), str(manifest), fake, methodology="one_shot_paired_v1",
                             token_counter_command_json=json.dumps([sys.executable, str(counter)]), token_policy=self.token_policy())
        self.assertEqual(executed["status"], "complete")
        self.assertEqual(counter_log.read_text().splitlines(), ["count", "count"])
        requests = [json.loads(line) for line in log.read_text().splitlines()]
        self.assertEqual(len(requests), 2)
        self.assertEqual(requests[0]["source"], requests[1]["source"])
        self.assertEqual(requests[0]["source_preparation"], requests[1]["source_preparation"])
        self.assertIsNone(requests[0]["condition_data"])
        self.assertIsNotNone(requests[1]["condition_data"]["reffector"])

    def test_one_shot_treatment_excludes_unrelated_call_edges(self) -> None:
        run = self.prepare_import("one-shot-edge-scope-run")
        inventory = json.loads((run / "inventory.json").read_text())
        selected_package = inventory["packages"][0]["id"]
        index_path = run / "evidence-index.json"
        index = json.loads(index_path.read_text())
        index["records"].extend([
            {"ref": "call-edge:outside-only", "record_type": "call_edge", "package": "example.org/out-a", "file": "outside.go",
             "data": {"caller_package": "example.org/out-a", "callee_package": "example.org/out-b", "caller": {}, "callee": {}}},
            {"ref": "call-edge:selected-boundary", "record_type": "call_edge", "package": selected_package, "file": None,
             "data": {"caller_package": selected_package, "callee_package": "example.org/outside", "caller": {}, "callee": {}}},
        ])
        index_path.write_text(json.dumps(index), encoding="utf-8")
        treatment = benchmark_treatment_context(run, inventory, set(json.loads((run / "source-manifest.json").read_text())["files"]), one_shot=True)
        packed = treatment["reffector"]
        refs = {record[0] for group in packed["records_by_file"] for kind, records in group.items()
                if kind in packed["field_order"] for record in records}
        self.assertIn("call-edge:selected-boundary", refs)
        self.assertNotIn("call-edge:outside-only", refs)
        canonical_rows = [row for row in index["records"] if row.get("record_type") in packed["field_order"] and (
            row.get("file") in set(json.loads((run / "source-manifest.json").read_text())["files"])
            or (row.get("record_type") == "call_edge" and (row.get("data", {}).get("caller_package") in {p["id"] for p in inventory["packages"]} or row.get("data", {}).get("callee_package") in {p["id"] for p in inventory["packages"]}))) ]
        self.assertEqual(packed["record_counts"], dict(sorted(__import__("collections").Counter(row["record_type"] for row in canonical_rows).items())))
        expected = {row["ref"]: row for row in canonical_rows}
        prefixes = {"effect_site": "effect-site:", "audit_site": "audit-site:", "call_edge": "call-edge:", "unresolved_site": "unresolved-site:"}
        nested_field_names = packed["nested_field_order"]
        string_pool = packed["string_pool"]
        def unpack_nested(value):
            if isinstance(value, list) and len(value) == 2 and value[0] == "@s":
                return string_pool[value[1]]
            if isinstance(value, list) and value and value[0] == "@o":
                entries = value[1:]
                return {nested_field_names[entries[i]]: unpack_nested(entries[i + 1]) for i in range(0, len(entries), 2)}
            if isinstance(value, list) and value and value[0] == "@a":
                return [unpack_nested(child) for child in value[1:]]
            if isinstance(value, list):
                return [unpack_nested(child) for child in value]
            return value
        decoded_constants = packed["constants"]
        decoded = {}
        for group in packed["records_by_file"]:
            for kind in prefixes:
                for ref, package_marker, id_marker, mask, values in group.get(kind, []):
                    fields = packed["field_order"][kind]
                    data = {field: unpack_nested(value) for field, present, value in zip(fields, mask, values) if present == "1"}
                    if id_marker == "=":
                        data["id"] = ref.removeprefix(prefixes[kind])
                    elif id_marker is not None:
                        data["id"] = id_marker
                    if package_marker == "=":
                        data["package"] = group["package"]
                    elif package_marker is not None:
                        data["package"] = package_marker
                    for field in ("source_ref", "location"):
                        value = data.get(field)
                        if isinstance(value, list):
                            if len(value) == 2:
                                data[field] = {"file": group["file"], "line": value[0], "column": value[1]}
                            else:
                                data[field] = {"file": value[0], "line": value[1], "column": value[2]}
                    data.update(decoded_constants.get(kind, {}))
                    decoded[ref] = data
        self.assertEqual(set(decoded), set(expected))
        self.maxDiff = None
        for ref, data in decoded.items():
            self.assertEqual(data, expected[ref]["data"], ref)

    def test_one_shot_low_budget_executes_paired_file_partitions_and_concatenates_claims(self) -> None:
        run = self.prepare_import("one-shot-partitions-run")
        manifest = self.base / "one-shot-partitions.json"
        manifest.write_text(json.dumps({
            "schema_version": 1, "questions": [{"id": "q", "prompt": "Describe source."}],
            "rubric": {"required_facts": [{"id": "f", "all_terms": ["downloaders"]}]},
        }), encoding="utf-8")
        log = self.base / "partition-requests.jsonl"
        events = self.base / "partition-events.log"
        model = self.base / "partition-model.py"
        model.write_text(
            "import json, pathlib, sys\nrequest=json.load(sys.stdin)\n"
            f"pathlib.Path({str(log)!r}).open('a').write(json.dumps(request)+'\\n')\n"
            f"pathlib.Path({str(events)!r}).open('a').write('model\\n')\n"
            "print(json.dumps({'claims':[{'id':request['source_partition']['file'],'statement':'file partition characterized','category':'established','domain':'architecture','evidence':[]}],'model':'fake-model'}))\n",
            encoding="utf-8",
        )
        counter_log = self.base / "partition-counter.log"
        counter = self.base / "partition-counter.py"
        counter.write_text(
            "import json, pathlib, sys\np=json.load(sys.stdin); request=p['request']\n"
            f"pathlib.Path({str(counter_log)!r}).open('a').write('count\\n')\n"
            f"pathlib.Path({str(events)!r}).open('a').write('count\\n')\n"
            "n=150000 if len(request.get('source',[])) > 1 else 100\n"
            "print(json.dumps({'input_tokens':n,'token_count_type':'exact','model':'fake-model','context_window_tokens':200000,'max_output_tokens':32000}))\n",
            encoding="utf-8",
        )
        command = self.model_command(model)
        counter_command = json.dumps([sys.executable, str(counter)])
        result = benchmark(str(run), str(manifest), command, methodology="one_shot_paired_v1",
                           token_counter_command_json=counter_command, token_policy=self.token_policy())
        self.assertEqual(result["status"], "complete", str({key: (value.get("status"), value.get("reason"), value.get("partition_count")) for key, value in result["arms"].items()}))
        requests = [json.loads(line) for line in log.read_text().splitlines()]
        source_calls = [item for item in requests if item["condition_data"] is None]
        treatment_calls = [item for item in requests if item["condition_data"] is not None]
        self.assertEqual(len(source_calls), len(treatment_calls))
        self.assertEqual([item["source_partition"]["file"] for item in source_calls], [item["source_partition"]["file"] for item in treatment_calls])
        self.assertTrue(all(left["source"] == right["source"] and left["questions"] == right["questions"] and left["source_preparation"] == right["source_preparation"] for left, right in zip(source_calls, treatment_calls)))
        self.assertTrue(all(item["mode"] == "paired_source_context" for item in requests))
        expected_files = {item["source_partition"]["file"] for item in source_calls}
        partition_count = source_calls[0]["source_partition"]["total"]
        self.assertEqual(len(counter_log.read_text().splitlines()), 2 + 2 * partition_count)
        event_rows = events.read_text().splitlines()
        self.assertEqual(event_rows[:2 + 2 * partition_count], ["count"] * (2 + 2 * partition_count))
        self.assertEqual(event_rows[2 + 2 * partition_count:], ["model"] * (2 * partition_count))
        self.assertEqual({item["source_partition"]["file"] for item in source_calls}, expected_files)
        self.assertTrue(all(item["source_partition"]["total"] == len(source_calls) for item in requests))
        for item in requests:
            local_file = item["source_partition"]["file"]
            self.assertEqual(item["source_paths"], [local_file])
            self.assertEqual([source["path"] for source in item["source"]], [local_file])
            self.assertEqual(item["source_preparation"]["source_coverage"]["files"], len(json.loads((run / "source-manifest.json").read_text())["files"]))
            self.assertEqual(item["source_preparation"]["partition_coverage"]["file"], local_file)
            self.assertEqual({entry["path"] for entry in item["source_preparation"]["files"]}, set(json.loads((run / "source-manifest.json").read_text())["files"]))
        for label in ("source_only", "reffector_treatment"):
            arm = result["arms"][label]
            self.assertEqual(arm["aggregation"], "deterministic_claim_list_concatenation_with_partition_provenance")
            claims = arm["result"]["claims"]
            self.assertEqual(len(claims), len(source_calls))
            self.assertTrue({claim["partition_provenance"]["file"] for claim in claims}.issubset(expected_files))
            self.assertTrue(all(claim["partition_provenance"]["source_sha256"] for claim in claims))
        self.assertEqual(result["arms"]["source_only"]["partition_count"], len(source_calls))
        economics = result["call_economics"]["final_benchmark_calls"]
        self.assertEqual(economics["planned_generation_calls"], 2 * len(source_calls))
        self.assertEqual(economics["cap_bound_usd"], 8 * len(source_calls))
        self.assertEqual(result["content_category_bytes"]["source_only"]["treatment_only_evidence"], 0)
        self.assertGreater(result["content_category_bytes"]["reffector_treatment"]["treatment_only_evidence"], 0)
        def all_keys(value):
            if isinstance(value, dict):
                return set(value) | set().union(*(all_keys(child) for child in value.values()))
            if isinstance(value, list):
                return set().union(*(all_keys(child) for child in value), set())
            return set()
        self.assertFalse({"effects", "possible_effects", "uncertain", "call_edge", "cross_component_relationships"} & all_keys(source_calls[0]))

    def test_one_shot_partition_generation_failure_skips_peer_arm(self) -> None:
        run = self.prepare_import("one-shot-peer-failure-run")
        manifest = self.base / "one-shot-peer-failure.json"
        manifest.write_text(json.dumps({
            "schema_version": 1, "questions": [{"id": "q", "prompt": "Describe source."}],
            "rubric": {"required_facts": [{"id": "f", "all_terms": ["source"]}]},
        }), encoding="utf-8")
        request_log = self.base / "failed-partition-requests.jsonl"
        model = self.base / "fail-first-partition.py"
        model.write_text(
            "import json, pathlib, sys\n"
            "request=json.load(sys.stdin)\n"
            f"pathlib.Path({str(request_log)!r}).open('a').write(json.dumps(request)+'\\n')\n"
            "model='wrong-model' if request.get('condition_data') is None and request.get('source_partition',{}).get('ordinal') == 1 else 'fake-model'\n"
            "print(json.dumps({'claims':[],'model':model}))\n",
            encoding="utf-8",
        )
        counter = self.base / "failure-counter.py"
        counter.write_text(
            "import json, sys\np=json.load(sys.stdin); n=150000 if len(p['request'].get('source',[])) > 1 else 100\n"
            "print(json.dumps({'input_tokens':n,'token_count_type':'exact','model':'fake-model','context_window_tokens':200000,'max_output_tokens':32000}))\n",
            encoding="utf-8",
        )
        result = benchmark(str(run), str(manifest), json.dumps([sys.executable, str(model)]),
                           methodology="one_shot_paired_v1", token_counter_command_json=json.dumps([sys.executable, str(counter)]),
                           token_policy=self.token_policy())
        calls = [json.loads(line) for line in request_log.read_text().splitlines()]
        self.assertEqual(len(calls), 1)
        self.assertEqual(calls[0]["condition_data"], None)
        self.assertEqual(result["arms"]["source_only"]["status"], "failed")
        self.assertEqual(result["arms"]["reffector_treatment"]["status"], "skipped_after_peer_failure")
        self.assertEqual(result["arms"]["reffector_treatment"]["generation_calls_started"], 0)

    def test_one_shot_pair_preflights_both_arms_before_any_model_process(self) -> None:
        run = self.prepare_import("one-shot-preflight-run")
        manifest = self.base / "one-shot-preflight.json"
        manifest.write_text(json.dumps({
            "schema_version": 1, "questions": [{"id": "q", "prompt": "Describe source."}],
            "rubric": {"required_facts": [{"id": "f", "all_terms": ["source"]}]},
        }), encoding="utf-8")
        model_log = self.base / "model-started.log"
        model = self._write_model("benchmark", model_log)
        counter_log = self.base / "counter.log"
        counter = self.base / "paired-counter.py"
        counter.write_text(
            "import json, pathlib, sys\n"
            f"p=json.load(sys.stdin); pathlib.Path({str(counter_log)!r}).open('a').write('count\\n')\n"
            "t=bool(p['request'].get('condition_data'))\n"
            "n=150000 if t else 1\n"
            "print(json.dumps({'input_tokens':n,'token_count_type':'exact','model':'fake-model','context_window_tokens':200000,'max_output_tokens':32000}))\n",
            encoding="utf-8",
        )
        report = benchmark(
            str(run), str(manifest), self.model_command(model), methodology="one_shot_paired_v1",
            token_counter_command_json=json.dumps([sys.executable, str(counter)]), token_policy=self.token_policy(),
        )
        file_count = report["arms"]["source_only"]["partition_count"]
        self.assertEqual(len(counter_log.read_text().splitlines()), 2 + 2 * file_count)
        self.assertFalse(model_log.exists())
        self.assertEqual(report["arms"]["source_only"]["status"], "preflight_blocked")
        self.assertEqual(report["arms"]["reffector_treatment"]["status"], "preflight_blocked")
        for label in ("source_only", "reffector_treatment"):
            self.assertIn("whole_request_token_budget", report["arms"][label])
            self.assertIn("request", report["arms"][label])
            self.assertIn(label, report["content_category_bytes"])
        economics = report["call_economics"]["final_benchmark_calls"]
        self.assertEqual(economics["failed_generation_calls"], 0)
        self.assertEqual(economics["generation_calls_started"], 0)
        self.assertEqual(economics["planned_generation_calls"], 0)
        self.assertEqual(economics["hypothetical_calls_if_all_matched_partitions_fit"], 2 * file_count)
        self.assertEqual(economics["hypothetical_cap_exposure_usd"], 8 * file_count)
        self.assertEqual(report["call_economics"]["conditional_additional_calls"]["semantic_aggregation"], "0 calls; partition claims are concatenated locally and deterministically")

    def test_no_model_benchmark_is_pending_and_not_scored(self) -> None:
        run = self.prepare_import("pending-run")
        manifest = self.base / "pending.json"
        manifest.write_text(json.dumps({
            "schema_version": 1,
            "name": "pending",
            "questions": [{"id": "q", "prompt": "Describe it."}],
            "rubric": {"required_facts": [{"id": "f", "all_terms": ["cache"]}]},
        }), encoding="utf-8")
        result = benchmark(str(run), str(manifest), None, max_context_bytes=50000)
        self.assertEqual(result["status"], "pending")
        self.assertEqual(result["scores_blind_then_mapped"]["source_only"]["status"], "not_scored")
        log = self.base / "blocked-model-requests.jsonl"
        blocked = benchmark(str(run), str(manifest), self.model_command(self._write_model("benchmark", log)), max_context_bytes=50000)
        self.assertEqual(blocked["status"], "partial")
        self.assertFalse(log.exists())
        self.assertFalse(blocked["treatment_phase_validation"]["complete"])

    def test_fake_model_end_to_end_all_phases(self) -> None:
        run = self.prepare_import("all-phases-run")
        command = self.model_command(self._write_model("allphase"))
        investigation = self.investigate_run(run, command)
        self.assertEqual(investigation["status"], "complete")
        aggregation = self.aggregate_run(run, command)
        self.assertEqual(aggregation["status"], "complete")
        self.assertEqual(aggregation["semantic_aggregation"], "model")
        verification = self.verify_run(run, command)
        self.assertEqual(verification["status"], "complete")
        manifest = self.base / "all-phases-manifest.json"
        manifest.write_text(json.dumps({
            "schema_version": 1,
            "name": "fake-end-to-end",
            "questions": [{"id": "q", "prompt": "Describe the source."}],
            "rubric": {"required_facts": [{"id": "fact", "all_terms": ["downloaders", "writeback"]}]},
        }), encoding="utf-8")
        paired = self.benchmark_run(run, manifest, command, max_context_bytes=50000)
        self.assertEqual(paired["status"], "complete")
        self.assertTrue((run / "investigation-status.json").exists())
        self.assertTrue((run / "aggregate.json").exists())
        self.assertTrue((run / "verification.json").exists())
        self.assertTrue((run / "benchmark.json").exists())

    def test_secret_bearing_command_and_model_fields_are_redacted(self) -> None:
        self.assertEqual(
            safe_command_display(["/tmp/token-helper", "--api-key", "test-secret", "--mode", "safe"]),
            ["<redacted>", "--api-key", "<redacted>", "--mode", "safe"],
        )
        self.assertEqual(
            sanitize_model_value({"api_key": "test-secret", "statement": "password=test-secret"}),
            {"api_key": "<redacted>", "statement": "password=<redacted>"},
        )

    def test_conflict_detector_reports_disagreement(self) -> None:
        claims = [
            {"id": "same", "statement": "A writes", "category": "possible", "domain": "effect"},
            {"id": "same", "statement": "A only reads", "category": "established", "domain": "effect"},
        ]
        self.assertEqual(claim_conflicts(claims)[0]["claim_id"], "same")


if __name__ == "__main__":
    unittest.main()
