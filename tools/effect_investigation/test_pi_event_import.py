import json
import pathlib
import subprocess
import sys
import tempfile
import unittest

sys.path.insert(0, str(pathlib.Path(__file__).parent))

from pi_event_import import import_session
from investigate import validate_event_ledger

FIXTURE = pathlib.Path(__file__).with_name("fixtures") / "pi_session.jsonl"


def event(kind, **fields):
    return {"type": kind, **fields}


class PiEventImportTests(unittest.TestCase):
    def test_actual_pi_event_stream_imports_safe_observations(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / "src.go").write_text("a\nb\nc\nd\n")
            rows, report = import_session(FIXTURE, root)
        ledger = validate_event_ledger(rows)
        reads = ledger["source_reads"]
        self.assertEqual([(row["source_path"], row["status"]) for row in reads],
                         [("src.go", "success"), ("missing.go", "failed")])
        self.assertEqual(reads[0]["requested_start_line"], 3)
        self.assertEqual(reads[0]["requested_line_count"], 2)
        self.assertNotIn("start_line", reads[0])
        self.assertNotIn("end_line", reads[0])
        self.assertTrue(all("evidence_ref" in row for row in reads))
        self.assertEqual(len(ledger["source_searches"]), 1)
        self.assertEqual(ledger["source_searches"][0]["status"], "success")
        self.assertEqual(ledger["test_commands"][0]["exit_status"], "1")
        self.assertEqual(ledger["test_commands"][0]["argv"], ["go"])
        invocation = ledger["reffector_invocations"][0]
        self.assertEqual(invocation["argv"], ["reffector"])
        self.assertEqual((invocation["status"], invocation["exit_code"]), ("success", 0))
        self.assertEqual(report["session_id"], "fixture-session")
        self.assertEqual(report["metadata"]["provider"], "available")
        self.assertEqual(report["metadata"]["model"], "available")
        self.assertEqual(report["metadata"]["thinking"], "available")
        self.assertEqual(report["metadata"]["usage"], "available")
        self.assertEqual(report["metadata"]["worker_status"], "completed observed")
        self.assertEqual([row["status"] for row in ledger["worker_lifecycle"]], ["started", "completed"])
        metadata = next(row for row in ledger["session_metadata"] if row["kind"] == "assistant_message")
        self.assertEqual(metadata["provider"], "fixture-provider")
        self.assertEqual(metadata["usage"]["cost"]["total"], 0.031)
        self.assertEqual(len(report["unclassified_shell_commands"]), 1)
        self.assertEqual(ledger["supervisor_decisions"][0]["status"], "missing")
        serialized = json.dumps(rows)
        for forbidden in ("sanitized assistant content", "source text is deliberately not retained",
                          "src.go:3:needle", "analyzer output not retained", "sanitized test result"):
            self.assertNotIn(forbidden, serialized)
        self.assertEqual(ledger["recorded_counts"]["source_read_events"], 2)
        observed = report["observed_ledger_rows_only"]
        self.assertIn("not completeness", observed["basis"])
        self.assertEqual(observed["by_type"]["source_read"], 2)
        self.assertEqual(observed["by_type_and_status"]["worker_lifecycle"]["completed"], 1)

    def test_duplicate_events_dedupe_refs_and_absent_is_error_stays_unavailable(self):
        start = event("tool_execution_start", toolCallId="r1", toolName="read",
                      args={"path": "src.go", "offset": 4, "limit": 3})
        end = event("tool_execution_end", toolCallId="r1", toolName="read", result={"structuredContent": {} })
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            path = root / "partial.jsonl"
            path.write_text("\n".join(json.dumps(row) for row in (start, end, start, end)) + "\n{bad\n")
            rows, report = import_session(path, root)
        ledger = validate_event_ledger(rows)
        self.assertEqual(len(ledger["source_reads"]), 1)
        self.assertEqual(ledger["source_reads"][0]["status"], "unavailable")
        self.assertEqual(ledger["source_reads"][0]["requested_start_line"], 4)
        self.assertEqual(len(ledger["source_reads"][0]["evidence_refs"]), 3)
        self.assertEqual(report["availability"], "partial")
        self.assertEqual(report["malformed_records"], [5])
        self.assertEqual(report["metadata"]["model"], "unavailable")
        self.assertEqual(report["metadata"]["usage"], "unavailable")
        self.assertEqual(report["metadata"]["timestamps"], "unavailable")
        self.assertEqual(report["runner_status"]["process_exit_code"], "unavailable")
        self.assertEqual(report["runner_status"]["timed_out"], "unavailable")

    def test_success_flag_without_valid_result_payload_does_not_establish_read(self):
        entries = [
            event("tool_execution_start", toolCallId="read", toolName="read", args={"path": "src.go"}),
            event("tool_execution_end", toolCallId="read", toolName="read", isError=False),
        ]
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "missing-result.jsonl"
            path.write_text("\n".join(map(json.dumps, entries)))
            rows, report = import_session(path, directory)
        ledger = validate_event_ledger(rows)
        self.assertEqual(ledger["source_reads"][0]["status"], "unavailable")
        self.assertEqual(ledger["tool_results"][0]["status"], "unavailable")
        self.assertEqual(report["availability"], "partial")
        self.assertEqual(report["malformed_events"], [2])

    def test_content_only_tool_result_establishes_read_without_retaining_content(self):
        entries = [
            event("tool_execution_start", toolCallId="read", toolName="read", args={"path": "src.go"}),
            event("tool_execution_end", toolCallId="read", toolName="read", isError=False,
                  result={"content": [{"type": "text", "text": "private source text"}]}),
        ]
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "content-result.jsonl"
            path.write_text("\n".join(map(json.dumps, entries)))
            rows, report = import_session(path, directory)
        ledger = validate_event_ledger(rows)
        self.assertEqual(ledger["source_reads"][0]["status"], "success")
        self.assertNotIn("private source text", json.dumps(rows))
        self.assertEqual(report["availability"], "complete")

    def test_malformed_tool_start_marks_transcript_partial(self):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "malformed-start.jsonl"
            path.write_text(json.dumps(event("tool_execution_start", toolName="read", args={"path": "src.go"})))
            rows, report = import_session(path, directory)
        self.assertFalse([row for row in rows if row["type"] == "tool_invocation"])
        self.assertEqual(report["availability"], "partial")
        self.assertEqual(report["malformed_events"], [1])

    def test_distinct_identical_reads_keep_multiplicity_but_same_call_dedupes(self):
        entries = []
        for call_id in ("read-a", "read-b"):
            entries.extend((
                event("tool_execution_start", toolCallId=call_id, toolName="read",
                      args={"path": "same.go", "offset": 2, "limit": 3}),
                event("tool_execution_end", toolCallId=call_id, toolName="read", isError=False,
                      result={"structuredContent": {}}),
                event("tool_execution_end", toolCallId=call_id, toolName="read", isError=False,
                      result={"structuredContent": {}}),
            ))
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "reads.jsonl"
            path.write_text("\n".join(map(json.dumps, entries)))
            rows, report = import_session(path, directory)
        reads = validate_event_ledger(rows)["source_reads"]
        self.assertEqual(len(reads), 2)
        self.assertEqual({row["tool_call_id"] for row in reads}, {"read-a", "read-b"})
        self.assertTrue(all(len(row["evidence_refs"]) == 3 for row in reads))
        self.assertEqual(report["availability"], "complete")

    def test_identical_metadata_without_logical_id_preserves_multiplicity(self):
        message = {"role": "assistant", "provider": "p", "model": "m", "thinkingLevel": "low",
                   "timestamp": 1767322801000, "usage": {"input": 2, "output": 1, "totalTokens": 3}, "content": []}
        entries = [event("message_end", message=message), event("message_end", message=message)]
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "metadata.jsonl"
            path.write_text("\n".join(map(json.dumps, entries)))
            rows, _ = import_session(path, directory)
        metadata = [row for row in validate_event_ledger(rows)["session_metadata"] if row["kind"] == "assistant_message"]
        self.assertEqual(len(metadata), 2)
        self.assertNotEqual(metadata[0]["evidence_ref"]["line"], metadata[1]["evidence_ref"]["line"])

    def test_conflicting_tool_call_id_marks_partial_and_does_not_infer_read(self):
        entries = [
            event("tool_execution_start", toolCallId="same", toolName="read", args={"path": "a.go"}),
            event("tool_execution_start", toolCallId="same", toolName="read", args={"path": "b.go"}),
            event("tool_execution_start", toolCallId="same", toolName="read", args={"path": "a.go"}),
            event("tool_execution_end", toolCallId="same", toolName="read", isError=False, result={"structuredContent": {}}),
        ]
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "conflict.jsonl"
            path.write_text("\n".join(map(json.dumps, entries)))
            rows, report = import_session(path, directory)
        self.assertEqual(report["availability"], "partial")
        self.assertTrue(report["conflicts"])
        self.assertFalse([row for row in rows if row["type"] == "source_read"])

    def test_conflicting_duplicate_results_mark_derived_outcome_unavailable(self):
        entries = [
            event("tool_execution_start", toolCallId="r", toolName="read", args={"path": "src.go"}),
            event("tool_execution_end", toolCallId="r", toolName="read", isError=False, result={"structuredContent": {}}),
            event("tool_execution_end", toolCallId="r", toolName="read", isError=True, result={"structuredContent": {}}),
        ]
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "result-conflict.jsonl"
            path.write_text("\n".join(map(json.dumps, entries)))
            rows, report = import_session(path, directory)
        reads = validate_event_ledger(rows)["source_reads"]
        self.assertEqual(len(reads), 1)
        self.assertEqual(reads[0]["status"], "unavailable")
        self.assertEqual(report["availability"], "partial")

    def test_only_simple_bash_search_test_reffector_are_classified(self):
        commands = [
            ("rg", "rg query src.go", 0, False),
            ("no-match", "rg absent src.go", 1, False),
            ("contradictory", "grep absent src.go", 1, True),
            ("test", "go test ./pkg", 1, False),
            ("pytest", "pytest", 0, False),
            ("rg-regex", "rg 'needle|anchor' src.go", 0, False),
            ("test-prefix", "GOPROXY=off GOSUMDB=off go test -mod=readonly ./pkg -run '^TestCase$'", 1, False),
            ("reffector-prefix", "GOPROXY=off GOSUMDB=off GOFLAGS=-mod=readonly reffector -stats .", 0, False),
            ("unsafe-prefix", "API_KEY=prefix-secret reffector -stats .", 0, False),
            ("reffector", "reffector -stats . --api-key=credential -H 'Authorization: Bearer header-secret' "
             "--opaque innocuous-option-secret positional-secret "
             "--header 'Cookie: cookie-secret' --user 'alice:user-secret' -u 'bob:proxy-user-secret' "
             "--authorization flag-auth-secret --auth-token flag-token-secret "
             "'Authorization: Bearer inline-auth-secret' 'Proxy-Authorization: Basic inline-proxy-secret' "
             "'Cookie: inline-cookie-secret' 'Set-Cookie: inline-set-cookie-secret' "
             "'https://alice:url-password-secret@example.test/path'", 0, False),
            ("pipe", "rg query src.go | cat", 0, False),
            ("option-value", "rg --glob src.go needle", 0, False),
            ("unknown", "mystery --opaque value", 0, False),
        ]
        entries = []
        for ident, command, exit_code, is_error in commands:
            entries.extend((
                event("tool_execution_start", toolCallId=ident, toolName="bash", args={"command": command}),
                event("tool_execution_end", toolCallId=ident, toolName="bash", isError=is_error,
                      result={"structuredContent": {"exit_code": exit_code, "output": "secret output"}}),
            ))
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "bash-events.jsonl"
            path.write_text("\n".join(map(json.dumps, entries)))
            rows, report = import_session(path, directory)
        ledger = validate_event_ledger(rows)
        searches = {row["tool_call_id"]: row["status"] for row in ledger["source_searches"]}
        self.assertEqual(searches["rg"], "success")
        self.assertEqual(searches["no-match"], "no_matches")
        self.assertEqual(searches["contradictory"], "failed")
        self.assertEqual(searches["rg-regex"], "success")
        self.assertEqual({row["scope"] for row in ledger["source_searches"]}, {"src.go"})
        commands_by_id = {row["tool_call_id"]: row for row in ledger["test_commands"]}
        self.assertEqual(commands_by_id["test"]["exit_status"], "1")
        self.assertEqual(commands_by_id["pytest"]["exit_status"], "0")
        self.assertEqual(commands_by_id["test-prefix"]["exit_status"], "1")
        invocations_by_id = {row["invocation_id"]: row for row in ledger["reffector_invocations"]}
        prefixed = invocations_by_id[next(row["invocation_id"] for row in ledger["reffector_invocations"]
                                           if row["invocation_id"] == "reffector-prefix")]
        self.assertEqual(prefixed["argv"], ["reffector"])
        self.assertEqual(prefixed["status"], "success")
        invocation_argv = next(row["argv"] for row in ledger["reffector_invocations"]
                               if row["invocation_id"] == "reffector")
        self.assertEqual(invocation_argv, ["reffector"])
        self.assertEqual(ledger["reffector_invocations"][0]["exit_code"], 0)
        self.assertEqual(len(report["unclassified_shell_commands"]), 4)
        self.assertNotIn("prefix-secret", json.dumps(rows))
        self.assertNotIn("secret output", json.dumps(rows))
        serialized = json.dumps(rows)
        for credential in ("credential", "header-secret", "cookie-secret", "user-secret", "proxy-user-secret",
                           "flag-auth-secret", "flag-token-secret", "inline-auth-secret", "inline-proxy-secret",
                           "inline-cookie-secret", "inline-set-cookie-secret", "url-password-secret",
                           "innocuous-option-secret", "positional-secret"):
            self.assertNotIn(credential, serialized)

    def test_canonical_and_outside_paths_packet_builder_is_not_read(self):
        entries = [
            event("tool_execution_start", toolCallId="outside", toolName="read", args={"path": "../private.go"}),
            event("tool_execution_end", toolCallId="outside", toolName="read", isError=False, result={"structuredContent": {}}),
            event("tool_execution_start", toolCallId="packet", toolName="packet_builder", args={"source": "src.go"}),
            event("tool_execution_end", toolCallId="packet", toolName="packet_builder", isError=False,
                  result={"structuredContent": {"excerpt": "src.go: excerpt"}}),
        ]
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "events.jsonl"
            path.write_text("\n".join(map(json.dumps, entries)))
            rows, report = import_session(path, directory)
        self.assertFalse([row for row in rows if row["type"] == "source_read"])
        self.assertEqual(len(report["outside_paths"]), 1)
        self.assertEqual(report["metadata"]["worker_status"], "unavailable (no agent_settled)")
        self.assertNotIn("src.go: excerpt", json.dumps(rows))

    def test_agent_settled_is_terminal_and_aborted_is_not_completion(self):
        entries = [event("agent_start"), event("agent_end", messages=[]), event("agent_settled", aborted=True)]
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "lifecycle.jsonl"
            path.write_text("\n".join(map(json.dumps, entries)))
            rows, report = import_session(path, directory)
        lifecycle = validate_event_ledger(rows)["worker_lifecycle"]
        self.assertEqual([row["status"] for row in lifecycle], ["started", "aborted"])
        self.assertEqual(report["metadata"]["worker_status"], "aborted observed")

    def test_phased_decision_sequence_and_session_identity(self):
        decisions = []
        for phase, decision in ((1, "continue"), (2, "stop")):
            decisions.append({"phase": phase, "decision": decision, "timestamp": f"2026-01-02T03:0{phase}:00Z",
                              "rationale": "coverage reviewed", "scope_adequate": phase == 2,
                              "another_phase_warranted": phase == 1,
                              "coverage_achieved": {"operational_areas": ["rpc"], "source_paths": ["src.go"],
                                                    "tests_and_probes": ["focused test"]},
                              "remaining_leads": []})
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            events = root / "events.jsonl"
            events.write_text(json.dumps(event("agent_start")) + "\n" +
                              json.dumps(event("agent_settled", aborted=False)) + "\n" +
                              json.dumps(event("agent_settled", aborted=False)) + "\n")
            rows, report = import_session(events, root, run_metadata={"session_id": "rpc-session"},
                                          supervisor_decisions=decisions)
        ledger = validate_event_ledger(rows)
        self.assertEqual([row["phase"] for row in ledger["supervisor_decisions"] if row["status"] == "recorded"], [1, 2])
        self.assertEqual(report["session_id"], "rpc-session")
        self.assertEqual(len([row for row in rows if row.get("type") == "worker_lifecycle"]), 3)

    def test_rpc_supervised_continue_then_stop_records_checkpoint_evidence(self):
        decisions = []
        for phase, choice in ((1, "continue"), (2, "stop")):
            decisions.append({"phase": phase, "decision": choice, "timestamp": f"2026-01-02T03:0{phase}:00Z",
                              "rationale": "reviewed", "scope_adequate": choice == "stop",
                              "another_phase_warranted": choice == "continue",
                              "coverage_achieved": {"operational_areas": ["rpc"], "source_paths": ["src/file.py"],
                                                    "tests_and_probes": ["test"]}, "remaining_leads": []})
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            events, decisions_path = root / "events.jsonl", root / "decisions.jsonl"
            events.write_text("\n".join(json.dumps(event("agent_settled", aborted=False)) for _ in range(2)))
            decisions_path.write_text("\n".join(map(json.dumps, decisions)))
            rows, report = import_session(events, root, supervisor_decisions=decisions,
                supervisor_decisions_path=decisions_path,
                run_metadata={"execution_mode": "rpc-supervised", "termination_reason": "supervisor_stop",
                              "settled_checkpoints": 2, "final_decision": "stop", "process_exit_code": 0,
                              "session_id": "rpc-identity"})
        ledger = validate_event_ledger(rows)
        recorded = ledger["supervisor_decisions"]
        self.assertEqual([item["evidence_ref"]["line"] for item in recorded], [1, 2])
        self.assertEqual([item["decision"] for item in recorded], ["continue", "stop"])
        self.assertEqual(recorded[0]["session_id"], "rpc-identity")
        self.assertEqual(report["session_id"], "rpc-identity")
        self.assertEqual([item["status"] for item in ledger["worker_lifecycle"]], ["settled", "settled", "completed"])

    def test_rpc_completion_requires_matching_counts_and_unaborted_settlements(self):
        decision = {"phase": 1, "decision": "stop", "timestamp": "2026-01-02T03:01:00Z",
                    "rationale": "reviewed", "scope_adequate": True, "another_phase_warranted": False,
                    "coverage_achieved": {"operational_areas": [], "source_paths": [], "tests_and_probes": []},
                    "remaining_leads": []}
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            path = root / "events.jsonl"
            metadata = {"execution_mode": "rpc-supervised", "session_id": "s", "termination_reason": "supervisor_stop",
                        "settled_checkpoints": 2, "final_decision": "stop", "process_exit_code": 0}
            path.write_text(json.dumps(event("agent_settled", aborted=False)))
            rows, report = import_session(path, root, run_metadata=metadata, supervisor_decisions=[decision])
            ledger = validate_event_ledger(rows)
            self.assertEqual(ledger["worker_lifecycle"][-1]["status"], "incomplete")
            self.assertEqual(report["availability"], "partial")
            self.assertIn("mismatch", " ".join(report["limitations"]))

            metadata["settled_checkpoints"] = 1
            path.write_text(json.dumps(event("agent_settled", aborted=True)))
            rows, report = import_session(path, root, run_metadata=metadata, supervisor_decisions=[decision])
            lifecycle = validate_event_ledger(rows)["worker_lifecycle"]
            self.assertEqual(lifecycle[-1]["status"], "incomplete")
            self.assertTrue(lifecycle[0]["aborted"])
            self.assertEqual(report["availability"], "partial")

    def test_rpc_rejects_unphased_decisions_and_marks_continue_without_stop_missing(self):
        legacy = {"decision": "continue", "timestamp": "2026-01-02T03:01:00Z", "rationale": "more",
                  "scope_adequate": False, "another_phase_warranted": True}
        continue_decision = {"phase": 1, **legacy,
                             "coverage_achieved": {"operational_areas": [], "source_paths": [], "tests_and_probes": []},
                             "remaining_leads": []}
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            path = root / "events.jsonl"
            path.write_text(json.dumps(event("agent_settled", aborted=False)))
            metadata = {"execution_mode": "rpc-supervised", "session_id": "s", "termination_reason": "external_deadline",
                        "settled_checkpoints": 1, "final_decision": "missing"}
            with self.assertRaisesRegex(ValueError, "explicitly phased"):
                import_session(path, root, run_metadata=metadata, supervisor_decisions=[legacy])
            rows, report = import_session(path, root, run_metadata=metadata, supervisor_decisions=[continue_decision])
            decisions = validate_event_ledger(rows)["supervisor_decisions"]
            self.assertEqual([row["status"] for row in decisions], ["recorded", "missing"])
            self.assertEqual(report["supervisor_decision"], "incomplete (missing final stop)")

    def test_physical_decision_line_refs_are_preserved(self):
        decision = {"phase": 1, "decision": "stop", "timestamp": "2026-01-02T03:01:00Z",
                    "rationale": "reviewed", "scope_adequate": True, "another_phase_warranted": False,
                    "coverage_achieved": {"operational_areas": [], "source_paths": [], "tests_and_probes": []},
                    "remaining_leads": []}
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            path = root / "events.jsonl"
            path.write_text("{}")
            rows, _ = import_session(path, root, supervisor_decisions=[decision],
                                     supervisor_decision_line_numbers=[4])
        row = validate_event_ledger(rows)["supervisor_decisions"][0]
        self.assertEqual(row["evidence_ref"]["line"], 4)

    def test_rpc_deadline_missing_decision_and_early_pi_exit_are_not_completion(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            path = root / "events.jsonl"
            path.write_text(json.dumps(event("agent_settled", aborted=False)))
            metadata = {"execution_mode": "rpc-supervised", "session_id": "s", "termination_reason": "external_deadline",
                        "settled_checkpoints": 1, "final_decision": "missing"}
            rows, report = import_session(path, root, run_metadata=metadata)
            ledger = validate_event_ledger(rows)
            self.assertEqual(ledger["supervisor_decisions"][0]["status"], "missing")
            self.assertEqual(ledger["worker_lifecycle"][-1]["status"], "timed_out")
            self.assertEqual(report["supervisor_decision"], "incomplete (missing)")
            path.write_text(json.dumps(event("agent_settled", aborted=False)))
            metadata.update(termination_reason="pi_exit", process_exit_code=0, settled_checkpoints=1)
            rows, report = import_session(path, root, run_metadata=metadata)
            self.assertEqual(validate_event_ledger(rows)["worker_lifecycle"][-1]["status"], "failed")
            self.assertNotEqual(report["metadata"]["worker_status"], "completed observed")

    def test_phased_decisions_reject_sequence_and_noncanonical_paths(self):
        base = {"decision": "stop", "timestamp": "2026-01-02T03:01:00Z", "rationale": "done",
                "scope_adequate": True, "another_phase_warranted": False,
                "coverage_achieved": {"operational_areas": [], "source_paths": ["../outside.go"], "tests_and_probes": []},
                "remaining_leads": []}
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "events.jsonl"
            path.write_text("{}\\n")
            with self.assertRaises(ValueError):
                import_session(path, directory, supervisor_decisions=[{"phase": 2, **base}])
            for invalid_path in ("../outside.go", "src\\\\file.go"):
                invalid = dict(base, coverage_achieved={"operational_areas": [], "source_paths": [invalid_path], "tests_and_probes": []})
                with self.assertRaises(ValueError):
                    import_session(path, directory, supervisor_decisions=[{"phase": 1, **invalid}])
            with self.assertRaises(ValueError):
                import_session(path, directory, supervisor_decisions=[{"phase": 1, **{key: value for key, value in base.items() if key not in ("coverage_achieved", "remaining_leads")}}])

    def test_run_metadata_and_explicit_supervisor_decision(self):
        run = {"run_id": "run-7", "started_at": "2026-01-02T03:00:00Z", "ended_at": "2026-01-02T03:02:00Z",
               "wall_duration_ms": 120000, "source_revision": "abc", "analyzer_revision": "def",
               "process_exit_code": 7, "timed_out": False}
        decision = {"decision": "continue", "timestamp": "2026-01-02T03:03:00Z", "rationale": "survey remains",
                    "scope_adequate": False, "another_phase_warranted": True}
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "events.jsonl"
            path.write_text(json.dumps(event("session", id="session-7", version=3, timestamp="2026-01-02T03:00:00Z", cwd="/private")))
            rows, report = import_session(path, directory, run_metadata=run, supervisor_decision=decision,
                                          supervisor_decision_path="decision.json")
        ledger = validate_event_ledger(rows)
        recorded = ledger["supervisor_decisions"][0]
        self.assertEqual(recorded["decision"], "continue")
        self.assertEqual(recorded["evidence_ref"], {"input": "decision.json", "json_pointer": ""})
        self.assertEqual(report["session_id"], "session-7")
        self.assertEqual(report["investigation_facts"]["wall_duration_ms"], 120000)
        self.assertEqual(report["worker_id"], "run-7")
        run_row = next(row for row in ledger["session_metadata"] if row["kind"] == "run_metadata")
        self.assertEqual(run_row["worker_id"], "run-7")
        self.assertEqual(run_row["evidence_ref"], {"input": "run-metadata.json", "json_pointer": ""})
        termination = ledger["termination"]
        self.assertEqual(termination["wall_duration_ms"], 120000)
        self.assertEqual(termination["active_duration_status"], "unavailable")
        self.assertEqual(termination["evidence_ref"]["json_pointer"], "")
        self.assertEqual(report["metadata"]["worker_status"], "failed (runner exit)")
        self.assertEqual([row["status"] for row in ledger["worker_lifecycle"]], ["failed"])
        self.assertNotIn("/private", json.dumps(rows))
        for invalid in (dict(decision, scope_adequate="false"), {**decision, "extra": "forbidden"}):
            with self.assertRaises(ValueError):
                import_session(path, directory, supervisor_decision=invalid)
        with self.assertRaises(ValueError):
            import_session(path, directory, run_metadata={"run_id": "r", "environment": {}})

    def test_runner_timeout_without_agent_settled_records_failure_not_completion(self):
        entries = [event("agent_start"), event("agent_end", messages=[])]
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "events.jsonl"
            path.write_text("\n".join(map(json.dumps, entries)))
            rows, report = import_session(path, directory, run_metadata={"timed_out": True})
        ledger = validate_event_ledger(rows)
        self.assertEqual([row["status"] for row in ledger["worker_lifecycle"]], ["started", "failed"])
        self.assertEqual(report["metadata"]["worker_status"], "failed (runner timeout)")

    def test_persisted_session_entry_format_remains_supported(self):
        entries = [
            event("session", id="legacy-session", version=3, timestamp="2026-01-02T03:00:00Z", cwd="/sanitized"),
            event("message", id="m1", parentId=None, timestamp="2026-01-02T03:00:01Z",
                  message={"role": "assistant", "provider": "p", "model": "m", "thinkingLevel": "low",
                           "timestamp": 1767322801000, "usage": {"input": 2, "output": 1, "totalTokens": 3}, "content": []}),
        ]
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "session.jsonl"
            path.write_text("\n".join(map(json.dumps, entries)))
            rows, report = import_session(path, directory)
        validate_event_ledger(rows)
        self.assertEqual(report["session_id"], "legacy-session")
        self.assertEqual(report["metadata"]["model"], "available")
        self.assertEqual(report["metadata"]["worker_status"], "unavailable (no agent_settled)")

    def test_cli_writes_ledger_report_and_explicit_decision(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            ledger, report, decision = root / "events.jsonl", root / "coverage.json", root / "decision.json"
            run_metadata = root / "run.json"
            decision.write_text(json.dumps({"decision": "stop", "timestamp": "2026-01-02T03:00:00Z",
                                            "rationale": "scope adequate", "scope_adequate": True,
                                            "another_phase_warranted": False}))
            run_metadata.write_text(json.dumps({"run_id": "r1", "wall_duration_ms": 1200}))
            script = pathlib.Path(__file__).with_name("pi_event_import.py")
            process = subprocess.run([sys.executable, str(script), str(FIXTURE), "--repository-root", str(root),
                                      "--ledger", str(ledger), "--report", str(report), "--run-metadata", str(run_metadata),
                                      "--supervisor-decision", str(decision)],
                                     capture_output=True, text=True, check=False)
            self.assertEqual(process.returncode, 0, process.stderr)
            imported = [json.loads(line) for line in ledger.read_text().splitlines()]
            coverage = json.loads(report.read_text())
            self.assertEqual(coverage["supervisor_decision"], "recorded")
            self.assertEqual(coverage["run_metadata"]["run_id"], "r1")
            self.assertEqual(validate_event_ledger(imported)["supervisor_decisions"][0]["decision"], "stop")


if __name__ == "__main__":
    unittest.main()
