import hashlib
import json
import tempfile
import unittest
from types import SimpleNamespace
from pathlib import Path

from model_investigation import (InvestigationError, aggregate_findings, invoke_isolated, pi_json_adapter,
                                 verify_static_claims,
                                 prepare_model_packet, retrieve_source_excerpt,
                                 run_and_persist)


def canonical_id(prefix, value):
    return prefix + ":" + hashlib.sha256(value.encode()).hexdigest()


def source_reference(path, content, start_line=1, end_line=1):
    ref = {"path": path, "sha256": hashlib.sha256(content).hexdigest(),
           "start_line": start_line, "end_line": end_line}
    material = json.dumps(ref, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()
    return {"id": "source:" + hashlib.sha256(material).hexdigest(), **ref}


class IsolatedBoundaryTests(unittest.TestCase):
    def setUp(self):
        self.candidate_id = canonical_id("candidate", "candidate:abc")
        self.site_id = canonical_id("site", "site:def")
        self.packet = {"candidate_id": self.candidate_id,
                       "operation": {"id": self.site_id, "api": "os.RemoveAll"},
                       "source_refs": [], "source_excerpts": []}

    def response(self, **updates):
        value = {"candidate_id": self.candidate_id, "site_id": self.site_id, "hypothesis": None,
                 "claims": [], "alternatives": [], "limitations": ["static only"],
                 "verification_plan": ["inspect caller"]}
        value.update(updates)
        return json.dumps(value)

    def test_one_packet_success_and_traceable_evidence(self):
        seen = []
        result = invoke_isolated(self.packet, lambda request: (seen.append(json.loads(request)) or self.response()))
        self.assertEqual(len(seen), 1)
        self.assertEqual(set(seen[0]), {"candidate_id", "packet"})
        self.assertEqual(result["verified_claims"], [])
        self.assertEqual(result["hypothesis_status"], "unresolved")
        self.assertEqual(result["runtime_status"], "not-confirmed")
        self.assertIsNone(result["hypothesis"])
        unsupported = invoke_isolated(self.packet, lambda _request: self.response(
            hypothesis="a free-form concern", claims=[]))
        self.assertEqual(unsupported["hypothesis_status"], "unsupported")

    def graph_packet(self, *, candidate=True):
        packet = json.loads(json.dumps(self.packet))
        caller_id, callee_id = canonical_id("node", "caller"), canonical_id("node", "callee")
        edge_id = canonical_id("link", "edge")
        packet["graph"] = {
            "edge_refs": [edge_id],
            "nodes": [{"id": caller_id}, {"id": callee_id}],
            "edges": [{"id": edge_id, "caller": caller_id,
                       "callee": callee_id, "kind": "call",
                       "resolution": "candidate_vta" if candidate else "established",
                       "candidate": candidate, "synthetic": False}],
        }
        return packet, edge_id

    def source_packet(self, root, *, start_line=1, end_line=1):
        content = b"package cache\nfunc cleanup() {}\n"
        source = Path(root) / "pkg" / "cache.go"
        source.parent.mkdir(parents=True, exist_ok=True)
        source.write_bytes(content)
        ref = source_reference("pkg/cache.go", content, start_line, end_line)
        packet = json.loads(json.dumps(self.packet))
        packet["source_refs"] = [ref]
        packet["source_excerpts"] = [{"id": ref["id"], "path": "forged.go", "text": "stale"}]
        return packet, ref

    def test_static_claims_bind_typed_canonical_evidence_without_promoting_it(self):
        with tempfile.TemporaryDirectory() as directory:
            packet, ref = self.source_packet(directory)
            packet, edge_id = self.graph_packet()
            packet["source_refs"] = [ref]
            claims = [
                {"kind": "operation", "evidence_id": self.site_id, "site_id": self.site_id,
                 "api": "os.RemoveAll", "role": "context"},
                {"kind": "relationship", "evidence_id": edge_id, "site_id": self.site_id,
                 "api": "os.RemoveAll", "role": "supporting"},
                {"kind": "source", "evidence_id": ref["id"], "site_id": self.site_id,
                 "api": "os.RemoveAll", "role": "supporting"},
            ]
            verified = verify_static_claims(claims, packet, repository_root=directory)
            self.assertEqual([item["verification_outcome"] for item in verified],
                             ["identity-bound"] * 3)
            relation = verified[1]["evidence"]
            self.assertEqual(relation["resolution"], "candidate_vta")
            self.assertEqual(relation["caller_id"], packet["graph"]["edges"][0]["caller"])
            self.assertEqual(relation["callee_id"], packet["graph"]["edges"][0]["callee"])
            self.assertTrue(relation["candidate"])
            self.assertFalse(relation["execution_established"])
            source = verified[2]["evidence"]
            self.assertEqual((source["path"], source["sha256"], source["start_line"], source["end_line"]),
                             (ref["path"], ref["sha256"], 1, 1))
            self.assertEqual(source["text"], "1: package cache")

    def test_static_claims_reject_wrong_type_site_api_edge_and_source_path(self):
        """Reject arbitrary-ID lookup, wrong site/API context, forged edge IDs and path-only sources."""
        with tempfile.TemporaryDirectory() as directory:
            packet, ref = self.source_packet(directory)
            packet, edge_id = self.graph_packet()
            packet["source_refs"] = [ref]
            good = {"kind": "relationship", "evidence_id": edge_id, "site_id": self.site_id,
                    "api": "os.RemoveAll", "role": "supporting"}
            adversarial = [
                dict(good, resolution="established"),  # model cannot supply a resolution
                dict(good, site_id=canonical_id("site", "other")),
                dict(good, api="os.ReadFile"),
                dict(good, evidence_id=canonical_id("link", "invented")),
                dict(good, kind="source"),  # relationship ID is not source evidence
                {"kind": "source", "evidence_id": ref["id"], "site_id": self.site_id,
                 "api": "os.RemoveAll", "role": "supporting", "path": "forged.go"},
                {"kind": "source", "evidence_id": canonical_id("source", "invented"),
                 "site_id": self.site_id, "api": "os.RemoveAll", "role": "supporting"},
            ]
            for claim in adversarial:
                with self.subTest(claim=claim), self.assertRaises(InvestigationError):
                    verify_static_claims([claim], packet, repository_root=directory)

    def test_model_cannot_promote_candidate_or_claim_runtime_confirmation(self):
        """Distinguish evidence-bound candidate facts from model-supplied resolution/status labels."""
        packet, edge_id = self.graph_packet()
        claim = {"kind": "relationship", "evidence_id": edge_id, "site_id": self.site_id,
                 "api": "os.RemoveAll", "role": "supporting", "resolution": "established"}
        with self.assertRaises(InvestigationError):
            invoke_isolated(packet, lambda _request: self.response(claims=[claim]))
        # Free-form prose remains a hypothesis, never a verified/runtime result.
        raw = self.response(hypothesis="This relation is established and executed.",
                            claims=[{key: value for key, value in claim.items() if key != "resolution"}])
        result = invoke_isolated(packet, lambda _request: raw)
        self.assertEqual(result["hypothesis_status"], "plausible-unverified")
        self.assertEqual(result["runtime_status"], "not-confirmed")
        with self.assertRaises(InvestigationError):
            invoke_isolated(packet, lambda _request: self.response(hypothesis_status="runtime-confirmed"))

    def test_aggregation_only_groups_persisted_identical_verified_claims(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            packet, ref = self.source_packet(root)
            claim = {"kind": "source", "evidence_id": ref["id"], "site_id": self.site_id,
                     "api": "os.RemoveAll", "role": "supporting"}
            artifact_dirs = []
            for index in range(2):
                candidate_id = canonical_id("candidate", f"candidate:{index}")
                packet_for_run = dict(packet, candidate_id=candidate_id)
                output = root / f"run-{index}"
                run_and_persist(packet_for_run,
                                lambda _request, cid=candidate_id: self.response(
                                    candidate_id=cid, hypothesis="cache cleanup may be incomplete",
                                    claims=[claim]),
                                output, repository_root=root)
                artifact_dirs.append(output)
            groups = aggregate_findings(artifact_dirs, repository_root=root)
            self.assertEqual(len(groups), 1)
            group = groups[0]
            self.assertEqual(group["finding_ids"], sorted(
                canonical_id("candidate", f"candidate:{index}") for index in range(2)))
            self.assertTrue(group["provisional"])
            self.assertEqual(set(group["findings"][0]),
                             {"candidate_id", "hypothesis", "hypothesis_status", "verified_claims"})
            self.assertEqual(group["findings"][0]["hypothesis_status"], "plausible-unverified")
            self.assertEqual(group["findings"][0]["verified_claims"][0]["evidence_id"], ref["id"])

    def test_persistence_rejects_adversarial_claims_before_aggregation(self):
        """Prove invoke→persist rejects wrong API/edge, fake source, promotions, and opaque IDs."""
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            packet, ref = self.source_packet(root)
            packet, edge_id = self.graph_packet()
            packet["source_refs"] = [ref]
            good_relation = {"kind": "relationship", "evidence_id": edge_id,
                             "site_id": self.site_id, "api": "os.RemoveAll", "role": "supporting"}
            attempts = [
                self.response(hypothesis="possible issue", claims=[dict(good_relation, api="os.ReadFile")]),
                self.response(hypothesis="possible issue", claims=[dict(good_relation,
                    evidence_id=canonical_id("link", "invented"))]),
                self.response(hypothesis="possible issue", claims=[dict(good_relation,
                    resolution="established")]),
                self.response(hypothesis="possible issue", claims=[{
                    "kind": "source", "evidence_id": canonical_id("source", "invented"),
                    "site_id": self.site_id, "api": "os.RemoveAll", "role": "supporting"}]),
                self.response(hypothesis="possible issue", claims=[{
                    "kind": "source", "evidence_id": ref["id"], "site_id": self.site_id,
                    "api": "os.ReadFile", "role": "supporting"}]),
                self.response(hypothesis="possible issue", status="runtime-confirmed"),
                self.response(hypothesis="possible issue", resource_id="made-up-resource",
                              obligation_id="made-up-obligation"),
            ]
            failed_dirs = []
            for index, raw in enumerate(attempts):
                output = root / f"adversarial-{index}"
                with self.subTest(index=index), self.assertRaisesRegex(
                        InvestigationError, "failure artifacts persisted"):
                    run_and_persist(packet, lambda _request, value=raw: value, output,
                                    repository_root=root)
                failed_dirs.append(output)
                manifest = json.loads((output / "manifest.json").read_text())
                self.assertEqual(manifest["outcome"], "error")
                self.assertFalse((output / "result.json").exists())
            with self.assertRaisesRegex(InvestigationError, "successful version-2 invocation"):
                aggregate_findings([failed_dirs[0]], repository_root=root)

    def test_aggregation_rejects_artifact_mismatch(self):
        """Reject a partial or stale result whose bytes do not match the success manifest."""
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "run"
            run_and_persist(self.packet, lambda _request: self.response(), output)
            (output / "result.json").write_text("{}\\n")
            with self.assertRaisesRegex(InvestigationError, "result hash"):
                aggregate_findings([output])

    def test_aggregation_rejects_unverified_legacy_identity_labels(self):
        """Distinguish persisted evidence resolution from trusting caller-supplied IDs/statuses."""
        findings = [
            {"finding_id": "finding:a", "site_id": self.site_id, "effect_id": "delete",
             "resource_id": "resource:same", "obligation_id": "obligation:a",
             "evidence_ids": ["invented:matching"], "status": "plausible-unverified",
             "supported_caller_segments": []},
            {"finding_id": "finding:b", "site_id": self.site_id, "effect_id": "delete",
             "resource_id": "resource:same", "obligation_id": "obligation:b",
             "evidence_ids": ["invented:matching"], "status": "runtime-confirmed",
             "supported_caller_segments": []},
        ]
        with self.assertRaises(InvestigationError):
            aggregate_findings(findings)

    def test_aggregation_keeps_distinct_obligations_and_operations_separate(self):
        """Same site/API/text is insufficient when canonical source spans or sites differ."""
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            packet_one, ref_one = self.source_packet(root, start_line=1, end_line=1)
            packet_two, ref_two = self.source_packet(root, start_line=2, end_line=2)
            # Different authenticated ranges stand for distinct evidence-backed concerns.
            artifacts = []
            for index, (packet, ref) in enumerate(((packet_one, ref_one), (packet_two, ref_two))):
                candidate_id = canonical_id("candidate", f"obligation:{index}")
                packet = dict(packet, candidate_id=candidate_id)
                claim = {"kind": "source", "evidence_id": ref["id"], "site_id": self.site_id,
                         "api": "os.RemoveAll", "role": "supporting"}
                output = root / f"obligation-{index}"
                run_and_persist(packet, lambda _request, cid=candidate_id, c=claim: self.response(
                    candidate_id=cid, hypothesis="cache cleanup may be incomplete", claims=[c]),
                    output, repository_root=root)
                artifacts.append(output)
            self.assertEqual(aggregate_findings(artifacts, repository_root=root), [])

            other_site = canonical_id("site", "other operation")
            packet_other = dict(packet_one, candidate_id=canonical_id("candidate", "other operation"),
                                operation={"id": other_site, "api": "os.Remove"})
            claim_other = {"kind": "source", "evidence_id": ref_one["id"], "site_id": other_site,
                           "api": "os.Remove", "role": "supporting"}
            other_dir = root / "other-operation"
            run_and_persist(packet_other, lambda _request: self.response(
                candidate_id=packet_other["candidate_id"], site_id=other_site,
                hypothesis="cache cleanup may be incomplete", claims=[claim_other]),
                other_dir, repository_root=root)
            self.assertEqual(aggregate_findings([artifacts[0], other_dir], repository_root=root), [])

            source = root / "pkg" / "cache.go"
            source.write_bytes(b"modified after investigation\n")
            with self.assertRaisesRegex(InvestigationError, "digest"):
                aggregate_findings(artifacts, repository_root=root)

    def test_malformed_wrong_id_and_unbound_citation_fail_closed(self):
        """Reject recursively discoverable but wrong-kind IDs and unknown schema fields."""
        fabricated = {"kind": "source", "evidence_id": canonical_id("source", "invented"),
                      "site_id": self.site_id, "api": "os.RemoveAll", "role": "supporting"}
        for raw in ("not-json", self.response(site_id=canonical_id("site", "wrong")),
                    self.response(claims=[fabricated]), self.response(extra="ignored"),
                    self.response(resource_id="invented", obligation_id="invented")):
            with self.subTest(raw=raw), self.assertRaises(InvestigationError):
                invoke_isolated(self.packet, lambda _request: raw)

    def test_confined_retrieval_checks_digest_and_returns_traceable_excerpt(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / "pkg" / "sample.go"
            source.parent.mkdir()
            content = b"package pkg\nfunc target() {}\n"
            source.write_bytes(content)
            reference = source_reference("pkg/sample.go", content, 2, 2)
            excerpt = retrieve_source_excerpt(root, reference)
            self.assertEqual(excerpt["id"], reference["id"])
            self.assertEqual(excerpt["text"], "2: func target() {}")
            reference = source_reference("pkg/sample.go", content, 1, 2)
            excerpt = retrieve_source_excerpt(root, reference)
            self.assertEqual(excerpt["text"], "1: package pkg\n2: func target() {}")
            bad_reference = source_reference("pkg/sample.go", b"different content", 1, 2)
            with self.assertRaisesRegex(InvestigationError, "digest"):
                retrieve_source_excerpt(root, bad_reference)

    def test_preparation_reads_only_referenced_sources_and_drops_cached_excerpt(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / "main.go"
            content = b"package main\nfunc target() {}\n"
            source.write_bytes(content)
            reference = source_reference("main.go", content, 2, 2)
            packet = dict(self.packet, source_refs=[reference],
                          source_excerpts=[{"id": reference["id"], "text": "stale or untrusted"}])
            prepared = prepare_model_packet(packet, root)
            self.assertEqual(prepared["source_excerpts"][0]["text"], "2: func target() {}")
            self.assertEqual(prepared["source_refs"], packet["source_refs"])
            self.assertEqual(packet["source_excerpts"][0]["text"], "stale or untrusted")
            packet["source_refs"] = [dict(packet["source_refs"][0], path="../outside.go")]
            with self.assertRaises(InvestigationError):
                prepare_model_packet(packet, root)

    def test_retrieval_rejects_unrelated_paths_and_symlinks(self):
        with tempfile.TemporaryDirectory() as directory, tempfile.TemporaryDirectory() as outside:
            root, external = Path(directory), Path(outside) / "secret.go"
            external.write_text("secret")
            reference = source_reference("../secret.go", b"secret")
            with self.assertRaisesRegex(InvestigationError, "repository-relative|escapes"):
                retrieve_source_excerpt(root, reference)
            (root / "linked.go").symlink_to(external)
            reference = source_reference("linked.go", b"secret")
            with self.assertRaisesRegex(InvestigationError, "outside repository|symlinked"):
                retrieve_source_excerpt(root, reference)

    def test_pi_adapter_pins_fresh_disabled_session_and_checks_metadata(self):
        output = json.dumps({"type": "message_end", "message": {
            "role": "assistant", "provider": "openai-codex", "model": "gpt-6-luna",
            "content": [{"type": "text", "text": self.response()}]}}) + "\n"
        seen = []

        def runner(command, **kwargs):
            seen.append((command, kwargs))
            return SimpleNamespace(returncode=0, stdout=output, stderr="")

        result = invoke_isolated(self.packet, lambda request: pi_json_adapter(request, runner=runner))
        self.assertIsNone(result["hypothesis"])
        command, kwargs = seen[0]
        self.assertEqual(command[command.index("--provider") + 1], "openai-codex")
        self.assertEqual(command[command.index("--model") + 1], "gpt-6-luna")
        system_prompt = command[command.index("--system-prompt") + 1]
        for contract in ("exactly these keys: candidate_id, site_id, hypothesis, claims",
                         "packet.operation.id", "claims must be an array of objects",
                         "Do not supply status, resolution, endpoints, paths, digests, or runtime confirmation.",
                         "no markdown or surrounding text"):
            self.assertIn(contract, system_prompt)
        for flag in ("--no-session", "--no-tools", "--no-mcp", "--no-extensions", "--no-context-files"):
            self.assertIn(flag, command)
        self.assertEqual(kwargs["timeout"], 120)
        self.assertEqual(kwargs["check"], False)

        wrong = output.replace("openai-codex", "openai")
        with self.assertRaisesRegex(InvestigationError, "event stream exceeds"):
            pi_json_adapter("{}", runner=lambda *_args, **_kwargs:
                            SimpleNamespace(returncode=0, stdout=output + " " * 256_001, stderr=""))
        with self.assertRaisesRegex(InvestigationError, "metadata mismatch"):
            pi_json_adapter("{}", runner=lambda *_args, **_kwargs:
                            SimpleNamespace(returncode=0, stdout=wrong, stderr=""))

        # A complete assistant answer does not override a failed CLI exit status.
        nonzero = SimpleNamespace(returncode=7, stdout=output, stderr="provider wrapper failed")
        with self.assertRaisesRegex(InvestigationError, "nonzero status 7"):
            pi_json_adapter("{}", runner=lambda *_args, **_kwargs: nonzero)

    def test_driver_persists_traceable_request_result_and_provider_events(self):
        with tempfile.TemporaryDirectory() as directory:
            output = json.dumps({"type": "message_end", "message": {
                "role": "assistant", "provider": "openai-codex", "model": "gpt-6-luna",
                "content": [{"type": "text", "text": self.response()}]}}) + "\n"

            def runner(command, **kwargs):
                return SimpleNamespace(returncode=0, stdout=output, stderr="")

            def adapter(request, *, event_sink=None):
                return pi_json_adapter(request, runner=runner, event_sink=event_sink)

            root = Path(directory) / "run"
            result = run_and_persist(self.packet, adapter, root,
                                     invocation_setup={"thinking": "low"},
                                     provider_adapter=True)
            self.assertIsNone(result["hypothesis"])
            manifest = json.loads((root / "manifest.json").read_text())
            self.assertEqual(manifest["provider"], "openai-codex")
            self.assertEqual(manifest["model"], "gpt-6-luna")
            self.assertEqual(manifest["outcome"], "success")
            self.assertFalse((root / "error.json").exists())
            self.assertEqual(json.loads((root / "request.json").read_text())["candidate_id"],
                             self.candidate_id)
            self.assertEqual(json.loads((root / "result.json").read_text()), result)
            self.assertIn('"type": "message_end"', (root / "provider-events.jsonl").read_text())
            self.assertNotIn(str(root), (root / "manifest.json").read_text())

    def test_driver_persists_adapter_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / "failed-run"
            with self.assertRaisesRegex(InvestigationError, "failure artifacts persisted"):
                run_and_persist(self.packet, lambda _request: "not-json", root)
            self.assertTrue((root / "error.json").exists())
            self.assertFalse((root / "result.json").exists())
            self.assertTrue((root / "manifest.json").exists())
            self.assertEqual(json.loads((root / "manifest.json").read_text())["outcome"],
                             "error")

    def test_driver_persists_exact_provider_exit_diagnostic(self):
        with tempfile.TemporaryDirectory() as directory:
            output = json.dumps({"type": "message_end", "message": {
                "role": "assistant", "provider": "openai-codex", "model": "gpt-6-luna",
                "content": [{"type": "text", "text": self.response()}]}}) + "\\n"
            def adapter(_request, *, event_sink=None):
                if event_sink is not None:
                    event_sink(output)
                raise InvestigationError("Pi provider invocation returned nonzero status 7")
            root = Path(directory) / "provider-failure"
            with self.assertRaisesRegex(InvestigationError, "failure artifacts persisted"):
                run_and_persist(self.packet, adapter, root, provider_adapter=True)
            error = json.loads((root / "error.json").read_text())
            self.assertEqual(error["error"],
                             "adapter invocation failed: Pi provider invocation returned nonzero status 7")
            self.assertTrue((root / "provider-events.jsonl").exists())

    def test_reused_artifact_directory_is_refused_and_failed_run_has_no_stale_success(self):
        """Distinguish immutable per-run paths from stale success/error artifact reuse."""
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / "run"
            first = run_and_persist(self.packet, lambda _request: self.response(), output)
            original_manifest = (output / "manifest.json").read_bytes()
            called = []
            with self.assertRaisesRegex(InvestigationError, "directory already exists"):
                run_and_persist(self.packet, lambda _request: called.append(True) or "not-json", output)
            self.assertEqual(called, [])
            self.assertEqual((output / "manifest.json").read_bytes(), original_manifest)
            self.assertEqual(json.loads((output / "result.json").read_text()), first)
            failed = Path(directory) / "new-failed-run"
            with self.assertRaisesRegex(InvestigationError, "failure artifacts persisted"):
                run_and_persist(self.packet, lambda _request: "not-json", failed)
            self.assertEqual(json.loads((failed / "manifest.json").read_text())["outcome"], "error")
            self.assertFalse((failed / "result.json").exists())

    def test_request_and_response_limits_fail_closed(self):
        huge = dict(self.packet, payload="x" * 50_000)
        with self.assertRaisesRegex(InvestigationError, "request exceeds"):
            invoke_isolated(huge, lambda _request: self.response())
        with self.assertRaisesRegex(InvestigationError, "response missing or exceeds"):
            invoke_isolated(self.packet, lambda _request: " " * 16_001)


if __name__ == "__main__":
    unittest.main()
