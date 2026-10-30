#!/usr/bin/env python3
"""Import safe, structured observations from Pi JSONL without retaining prompt/output text."""
from __future__ import annotations

import argparse
import hashlib
import json
import pathlib
import re
import shlex
from datetime import datetime


RUN_METADATA_FIELDS = {"run_id", "started_at", "ended_at", "wall_duration_ms", "source_revision", "analyzer_revision",
                       "process_exit_code", "timed_out"}
DECISION_FIELDS = {"decision", "timestamp", "rationale", "scope_adequate", "another_phase_warranted"}
SAFE_SHELL_ASSIGNMENTS = {"GOPROXY": "off", "GOSUMDB": "off", "GOTOOLCHAIN": "local",
                          "GOFLAGS": "-mod=readonly", "GOWORK": "off"}
SHELL_ASSIGNMENT = re.compile(r"^([A-Za-z_][A-Za-z0-9_]*)=(.*)$")


def _safe_ref(path, line, entry):
    ident = entry.get("id")
    return {"input": path.name, "line": line, **({"event_id": ident} if isinstance(ident, str) and ident else {})}


def _canonical_source(root, value):
    if not isinstance(value, str) or not value:
        return None
    candidate = pathlib.Path(value)
    if pathlib.PureWindowsPath(value).is_absolute() or pathlib.PureWindowsPath(value).drive:
        return None
    if not candidate.is_absolute():
        candidate = root / candidate
    try:
        rel = candidate.resolve(strict=False).relative_to(root.resolve()).as_posix()
    except (OSError, ValueError):
        return None
    return rel if rel not in ("", ".") and ".." not in pathlib.PurePosixPath(rel).parts else None


def _safe_usage(usage):
    if not isinstance(usage, dict):
        return {}
    safe = {key: value for key, value in usage.items()
            if key in ("input", "output", "cacheRead", "cacheWrite", "totalTokens", "reasoning")
            and isinstance(value, (int, float)) and not isinstance(value, bool)}
    cost = usage.get("cost")
    if isinstance(cost, dict):
        safe["cost"] = {key: value for key, value in cost.items()
                        if key in ("input", "output", "cacheRead", "cacheWrite", "total")
                        and isinstance(value, (int, float)) and not isinstance(value, bool)}
    return safe


def _has_unquoted_shell_syntax(command):
    quote, escaped = None, False
    for char in command:
        if escaped:
            escaped = False
            continue
        if char == "\\" and quote != "'":
            escaped = True
            continue
        if quote == "'":
            if char == "'":
                quote = None
            continue
        if quote == '"':
            if char == '"':
                quote = None
            elif char in ("$", "`"):
                return True
            continue
        if char in ("'", '"'):
            quote = char
        elif char in "|;&<>$`\n\r#(){}*?[]~":
            return True
    return False


def _parse_simple_shell(command):
    """Return argv for one simple command; shell syntax is never evaluated."""
    if not isinstance(command, str) or _has_unquoted_shell_syntax(command):
        return None
    try:
        argv = shlex.split(command, posix=True)
    except ValueError:
        return None
    if not argv:
        return None
    used_assignments = set()
    while argv:
        match = SHELL_ASSIGNMENT.fullmatch(argv[0])
        if match is None:
            break
        name, value = match.groups()
        if name in used_assignments or SAFE_SHELL_ASSIGNMENTS.get(name) != value:
            return None
        used_assignments.add(name)
        argv.pop(0)
    if not argv or any(SHELL_ASSIGNMENT.fullmatch(arg) for arg in argv):
        return None
    return argv


def _search_paths(argv):
    """Accept only known no-value search flags; never mistake option values for paths."""
    no_value = {"-n", "--line-number", "-i", "--ignore-case", "-w", "--word-regexp",
                "-F", "--fixed-strings", "-E", "--extended-regexp", "-H", "--with-filename",
                "-h", "--no-filename", "-r", "-R", "--recursive", "--hidden", "-s",
                "--no-messages", "-v", "--invert-match", "-c", "--count", "-o",
                "--only-matching", "-q", "--quiet", "-l", "--files-with-matches",
                "--no-heading", "-S", "--smart-case", "-I", "--no-ignore", "-uu", "-uuu"}
    positional, options_done = [], False
    for arg in argv[1:]:
        if not options_done and arg == "--":
            options_done = True
        elif not options_done and arg.startswith("-") and arg != "-":
            if arg not in no_value:
                return None
        else:
            positional.append(arg)
    if not positional:
        return None
    return positional[1:]


def _iso_timestamp(value):
    if not isinstance(value, str) or not value:
        return False
    try:
        datetime.fromisoformat(value.replace("Z", "+00:00"))
        return True
    except ValueError:
        return False


def _validate_run_metadata(metadata):
    if metadata is None:
        return {}
    if not isinstance(metadata, dict) or set(metadata) - RUN_METADATA_FIELDS:
        raise ValueError("run metadata must be an object containing only allowlisted fields")
    safe = {}
    for field, value in metadata.items():
        if field in ("run_id", "source_revision", "analyzer_revision"):
            if not isinstance(value, str) or not value:
                raise ValueError(f"run metadata {field} must be a nonempty string")
            safe[field] = value
        elif field in ("started_at", "ended_at"):
            if not _iso_timestamp(value):
                raise ValueError(f"run metadata {field} must be an ISO timestamp")
            safe[field] = value
        elif field == "wall_duration_ms":
            if not isinstance(value, int) or isinstance(value, bool) or value < 0:
                raise ValueError("run metadata wall_duration_ms must be a nonnegative integer")
            safe[field] = value
        elif field == "process_exit_code":
            if not isinstance(value, int) or isinstance(value, bool):
                raise ValueError("run metadata process_exit_code must be an integer")
            safe[field] = value
        elif field == "timed_out":
            if not isinstance(value, bool):
                raise ValueError("run metadata timed_out must be boolean")
            safe[field] = value
    return safe


def _validate_decision(decision, path):
    if decision is None:
        return None
    if not isinstance(decision, dict) or set(decision) != DECISION_FIELDS:
        raise ValueError("supervisor decision requires exactly decision, timestamp, rationale, scope_adequate, and another_phase_warranted")
    if decision["decision"] not in ("continue", "stop") or not _iso_timestamp(decision["timestamp"]):
        raise ValueError("invalid supervisor decision or timestamp")
    if not isinstance(decision["rationale"], str) or not decision["rationale"].strip():
        raise ValueError("supervisor decision rationale must be nonempty")
    for key in ("scope_adequate", "another_phase_warranted"):
        if not isinstance(decision[key], bool):
            raise ValueError(f"supervisor decision {key} must be boolean")
    return {**decision, "evidence_ref": {"input": path.name, "json_pointer": ""}}


def import_session(input_path, repository_root, *, run_metadata=None, run_metadata_path=None,
                   supervisor_decision=None, supervisor_decision_path=None):
    """Return safe ledger rows and coverage; raw prompts/results are never copied."""
    path, root = pathlib.Path(input_path), pathlib.Path(repository_root).resolve()
    run_facts = _validate_run_metadata(run_metadata)
    run_metadata_path = pathlib.Path(run_metadata_path or "run-metadata.json")
    run_ref = {"input": run_metadata_path.name, "json_pointer": ""}

    decision = _validate_decision(supervisor_decision, pathlib.Path(supervisor_decision_path or "supervisor-decision.json"))
    rows, limitations, calls, seen, result_facts = [], [], {}, {}, {}
    conflicted_calls = set()
    state = {"session_id": None, "worker_id": run_facts.get("run_id"), "settled": False}
    report = {"schema": "reffector.pi-import-report.v1", "input": path.name, "availability": "complete",
              "input_records": 0, "malformed_records": [], "malformed_events": [], "conflicts": [], "outside_paths": [],
              "unclassified_shell_commands": [], "limitations": limitations,
              "session_id": "unavailable", "worker_id": state["worker_id"] or "unavailable",
              "runner_status": {"process_exit_code": run_facts.get("process_exit_code", "unavailable"),
                                "timed_out": run_facts.get("timed_out", "unavailable")},
              "run_id": run_facts.get("run_id", "unavailable"),
              "supervisor_decision": "recorded" if decision else "missing",
              "metadata": {"provider": "unavailable", "model": "unavailable", "thinking": "unavailable",
                           "usage": "unavailable", "worker_status": "unavailable", "timestamps": "unavailable"},
              "investigation_facts": {"wall_duration_ms": run_facts.get("wall_duration_ms", "unavailable"),
                                      "active_duration": "unavailable",
                                      "source_review": "not established by Pi tool-call records",
                                      "candidate_dispositions": "not encoded by Pi runtime events"}}

    def add(event, ref=None, related_refs=()):
        event = dict(event)
        event.setdefault("session_id", state["session_id"] or "unavailable")
        event.setdefault("worker_id", state["worker_id"] or "unavailable")
        refs = [item for item in related_refs if item is not None and item != ref]
        if ref is not None:
            event["evidence_ref"] = ref
        if refs:
            event["evidence_refs"] = refs
        payload = {key: value for key, value in event.items() if key not in ("evidence_ref", "evidence_refs")}
        fingerprint = hashlib.sha256(json.dumps(payload, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
        logical_id = None
        if event.get("type") in ("tool_invocation", "tool_result", "source_read", "source_search", "test_command"):
            logical_id = event.get("tool_call_id")
        elif event.get("type") == "reffector_invocation":
            logical_id = event.get("invocation_id")
        evidence = event.get("evidence_ref")
        if logical_id is None and event.get("type") == "session_metadata":
            logical_id = event.get("response_id")
            if logical_id is None and isinstance(evidence, dict):
                logical_id = evidence.get("event_id")
        key = (event.get("type"), str(logical_id)) if logical_id is not None else None
        if key is not None and key in seen:
            prior, previous_fingerprint = seen[key]
            primary_ref = prior.pop("evidence_ref", None)
            all_refs = prior.setdefault("evidence_refs", [])
            for item in ([primary_ref] if primary_ref is not None else []) + ([ref] if ref is not None else []) + refs:
                if item is not None and item not in all_refs:
                    all_refs.append(item)
            if fingerprint != previous_fingerprint:
                report["availability"] = "partial"
                report["conflicts"].append({"event_id": str(logical_id), "kind": "logical event ID reused with conflicting facts"})
                if "status" in prior:
                    prior["status"] = "unavailable"
                if "exit_status" in prior:
                    prior["exit_status"] = "unavailable"
                if event.get("type") == "reffector_invocation":
                    prior.pop("exit_code", None)
            return prior
        if key is not None:
            seen[key] = (event, fingerprint)
        rows.append(event)
        return event

    def attach_ref(row, ref):
        primary = row.pop("evidence_ref", None)
        refs = row.setdefault("evidence_refs", [])
        for item in ([primary] if primary is not None else []) + ([ref] if ref is not None else []):
            if item not in refs:
                refs.append(item)

    def remember_call(call_id, name, args, ref):
        if call_id in conflicted_calls:
            return None
        current = {"name": name, "args": args if isinstance(args, dict) else {}, "ref": ref, "classification": None}
        previous = calls.get(call_id)
        if previous is not None:
            if previous["name"] != current["name"] or previous["args"] != current["args"]:
                calls.pop(call_id, None)
                conflicted_calls.add(call_id)
                report["conflicts"].append({"event_id": call_id, "kind": "tool_call_id reused with different arguments"})
                report["availability"] = "partial"
                return None
            return previous
        calls[call_id] = current
        return current

    def classify_start(call_id, call, ref):
        name, args = call["name"], call["args"]
        add({"type": "tool_invocation", "tool_call_id": call_id, "tool_name": name}, ref)
        if name == "read":
            source = _canonical_source(root, args.get("path"))
            if source is None:
                report["outside_paths"].append({"evidence_ref": ref, "classification": "source path unavailable/outside repository"})
                call["classification"] = {"kind": "read", "source": None}
            else:
                offset, limit = args.get("offset", 1), args.get("limit")
                call["classification"] = {"kind": "read", "source": source,
                                           "requested_start_line": offset if isinstance(offset, int) and not isinstance(offset, bool) and offset > 0 else None,
                                           "requested_line_count": limit if isinstance(limit, int) and not isinstance(limit, bool) and limit > 0 else None}
            return
        if name != "bash":
            return
        argv = _parse_simple_shell(args.get("command"))
        if argv is None:
            report["unclassified_shell_commands"].append(ref)
            call["classification"] = {"kind": "unclassified"}
            return
        exe = pathlib.Path(argv[0]).name
        # Preserve only the recognized executable label, never arbitrary arguments.
        recorded_argv = [exe]
        if exe in ("reffector", "reffector.exe"):
            call["classification"] = {"kind": "reffector"}
            add({"type": "reffector_invocation", "invocation_id": call_id,
                 "target_revision": run_facts.get("source_revision", "unavailable"),
                 "argv": recorded_argv, "status": "unavailable"}, ref)
        elif exe == "pytest" or (exe in ("go", "python", "python3") and any(
                arg.lower() in ("test", "unittest", "pytest") or arg.lower().endswith("_test.py") for arg in argv[1:])):
            call["classification"] = {"kind": "test"}
            add({"type": "test_command", "tool_call_id": call_id, "command": exe,
                 "argv": recorded_argv, "exit_status": "unavailable"}, ref)
        elif exe in ("rg", "grep"):
            paths = _search_paths(argv)
            if paths is None:
                report["unclassified_shell_commands"].append(ref)
                call["classification"] = {"kind": "unclassified"}
                return
            call["classification"] = {"kind": "search"}
            canonical = []
            for item in paths:
                source_path = _canonical_source(root, item)
                if source_path is None:
                    report["outside_paths"].append({"evidence_ref": ref, "classification": "search scope unavailable/outside repository"})
                else:
                    canonical.append(source_path)
            scope = ",".join(canonical) or "unavailable"
            add({"type": "source_search", "tool_call_id": call_id,
                 "query": "bash " + exe + " invocation", "scope": scope, "status": "unavailable"}, ref)
        else:
            report["unclassified_shell_commands"].append(ref)
            call["classification"] = {"kind": "unclassified"}

    def record_result(call_id, call, result, is_error, ref):
        # A successful result needs a recognizable Pi payload, not just isError=false.
        result_valid = isinstance(result, dict) and (
            isinstance(result.get("structuredContent"), dict) or isinstance(result.get("content"), list))
        if not result_valid:
            report["availability"] = "partial"
            report["malformed_events"].append(ref["line"])
            if is_error is not True:
                is_error = None
        structured = result.get("structuredContent") if isinstance(result, dict) else None
        exit_code = structured.get("exit_code") if isinstance(structured, dict) else None
        exact_exit = isinstance(exit_code, int) and not isinstance(exit_code, bool)
        facts = (is_error if isinstance(is_error, bool) else None, exit_code if exact_exit else None)
        conflict = call_id in result_facts and result_facts[call_id] != facts
        if conflict:
            report["conflicts"].append({"event_id": call_id, "kind": "conflicting tool result observations"})
            report["availability"] = "partial"
            is_error, exit_code, exact_exit = None, None, False
            for prior in rows:
                if prior.get("tool_call_id") == call_id and prior.get("type") == "source_read":
                    prior["status"] = "unavailable"
                    attach_ref(prior, ref)
                elif prior.get("tool_call_id") == call_id and prior.get("type") == "test_command":
                    prior["exit_status"] = "unavailable"
                    attach_ref(prior, ref)
                elif prior.get("tool_call_id") == call_id and prior.get("type") == "source_search":
                    prior["status"] = "unavailable"
                    attach_ref(prior, ref)
                elif prior.get("invocation_id") == call_id and prior.get("type") == "reffector_invocation":
                    prior["status"] = "unavailable"
                    prior.pop("exit_code", None)
                    attach_ref(prior, ref)
        else:
            result_facts[call_id] = facts
        status = "unavailable" if not isinstance(is_error, bool) else ("failed" if is_error else "success")
        add({"type": "tool_result", "tool_call_id": call_id, "tool_name": call.get("name", "unavailable"),
             "status": status}, ref)
        if conflict:
            return
        exact_exit = isinstance(exit_code, int) and not isinstance(exit_code, bool)
        classification = call.get("classification")
        if not isinstance(classification, dict):
            return
        kind = classification.get("kind")
        if kind == "read" and classification.get("source"):
            read_status = status
            event = {"type": "source_read", "tool_call_id": call_id,
                     "source_path": classification["source"], "status": read_status,
                     "requested_start_line": classification.get("requested_start_line"),
                     "requested_line_count": classification.get("requested_line_count")}
            add(event, ref, related_refs=(call.get("ref"),))
        elif kind == "test":
            test_status = str(exit_code) if exact_exit else ("failed (exit status unavailable)" if is_error is True else "unavailable")
            for row in rows:
                if row.get("type") == "test_command" and row.get("tool_call_id") == call_id:
                    row["exit_status"] = test_status
                    attach_ref(row, ref)
        elif kind == "reffector":
            for row in rows:
                if row.get("type") == "reffector_invocation" and row.get("invocation_id") == call_id:
                    row["status"] = ("success" if exit_code == 0 else "failed") if exact_exit else "unavailable"
                    attach_ref(row, ref)
                    if exact_exit:
                        row["exit_code"] = exit_code
        elif kind == "search":
            search_status = ("failed" if is_error is True else
                             "success" if exact_exit and exit_code == 0 else
                             "no_matches" if exact_exit and exit_code == 1 else
                             "failed" if exact_exit else "unavailable")
            for row in rows:
                if row.get("type") == "source_search" and row.get("tool_call_id") == call_id:
                    row["status"] = search_status
                    attach_ref(row, ref)

    def record_assistant_metadata(message, ref):
        if not isinstance(message, dict):
            return
        provider, model = message.get("provider"), message.get("model")
        thinking, timestamp = message.get("thinkingLevel"), message.get("timestamp")
        usage = _safe_usage(message.get("usage"))
        for key, value in (("provider", provider), ("model", model), ("thinking", thinking)):
            if isinstance(value, str) and value:
                report["metadata"][key] = "available"
        if isinstance(message.get("usage"), dict):
            report["metadata"]["usage"] = "available"
        if isinstance(timestamp, (str, int, float)) and not isinstance(timestamp, bool):
            report["metadata"]["timestamps"] = "available"
        row = {"type": "session_metadata", "kind": "assistant_message"}
        response_id = message.get("responseId")
        if isinstance(response_id, str) and response_id:
            row["response_id"] = response_id
        for field, value in (("provider", provider), ("model", model), ("thinkingLevel", thinking), ("timestamp", timestamp)):
            if isinstance(value, (str, int, float)) and not isinstance(value, bool):
                row[field] = value
        if isinstance(message.get("usage"), dict):
            row["usage"] = usage
        add(row, ref)

    try:
        stream = path.open(encoding="utf-8")
    except OSError as error:
        raise ValueError("cannot open Pi transcript") from error
    with stream:
        for line_number, line in enumerate(stream, 1):
            try:
                entry = json.loads(line)
                if not isinstance(entry, dict):
                    raise ValueError("record is not an object")
            except (json.JSONDecodeError, ValueError):
                report["malformed_records"].append(line_number)
                report["availability"] = "partial"
                continue
            report["input_records"] += 1
            ref = _safe_ref(path, line_number, entry)
            kind = entry.get("type")
            if kind == "session":
                session_id = entry.get("id")
                if isinstance(session_id, str) and session_id:
                    state["session_id"] = session_id
                    report["session_id"] = session_id
                timestamp = entry.get("timestamp")
                if isinstance(timestamp, str):
                    report["metadata"]["timestamps"] = "available"
                    add({"type": "session_metadata", "kind": "session_header", "session_id": state["session_id"] or "unavailable",
                         "timestamp": timestamp}, ref)
                continue
            if kind in ("thinking_level_change", "model_change", "usage"):
                if kind == "thinking_level_change":
                    report["metadata"]["thinking"] = "available"
                    add({"type": "session_metadata", "kind": kind, "thinkingLevel": entry.get("thinkingLevel", "unavailable"),
                         "timestamp": entry.get("timestamp")}, ref)
                elif kind == "model_change":
                    report["metadata"]["provider"] = "available" if isinstance(entry.get("provider"), str) else "unavailable"
                    report["metadata"]["model"] = "available" if isinstance(entry.get("modelId"), str) else "unavailable"
                    add({"type": "session_metadata", "kind": kind,
                         **({"provider": entry["provider"]} if isinstance(entry.get("provider"), str) else {}),
                         **({"modelId": entry["modelId"]} if isinstance(entry.get("modelId"), str) else {}),
                         **({"timestamp": entry["timestamp"]} if isinstance(entry.get("timestamp"), str) else {})}, ref)
                else:
                    report["metadata"]["usage"] = "available"
                    add({"type": "session_metadata", "kind": "usage", "usage_kind": entry.get("kind", "unavailable"),
                         "provider": entry.get("provider", "unavailable"), "model": entry.get("model", "unavailable"),
                         "usage": _safe_usage(entry.get("usage")), **({"timestamp": entry["timestamp"]} if isinstance(entry.get("timestamp"), str) else {})}, ref)
                continue
            if kind in ("message_start", "message_end"):
                message = entry.get("message")
                if kind == "message_end" and isinstance(message, dict) and message.get("role") == "assistant":
                    record_assistant_metadata(message, ref)
                continue
            if kind == "agent_start":
                report["metadata"]["worker_status"] = "running observed"
                add({"type": "worker_lifecycle", "status": "started"}, ref)
                continue
            if kind == "agent_end":
                # Agent may continue after this event. Only agent_settled is terminal.
                continue
            if kind == "agent_settled":
                aborted = entry.get("aborted")
                if isinstance(aborted, bool):
                    state["settled"] = True
                    status = "aborted" if aborted else "completed"
                    report["metadata"]["worker_status"] = status + " observed"
                    add({"type": "worker_lifecycle", "status": status}, ref)
                else:
                    report["metadata"]["worker_status"] = "unavailable"
                    report["availability"] = "partial"
                    report["conflicts"].append({"line": line_number, "kind": "agent_settled missing boolean aborted"})
                continue
            if kind == "tool_execution_start":
                call_id, name, args = entry.get("toolCallId"), entry.get("toolName"), entry.get("args", {})
                if isinstance(call_id, str) and call_id and isinstance(name, str) and name and isinstance(args, dict):
                    call = remember_call(call_id, name, args, ref)
                    if call is not None:
                        classify_start(call_id, call, ref)
                else:
                    report["availability"] = "partial"
                    report["malformed_events"].append(line_number)
                continue
            if kind == "tool_execution_end":
                call_id = entry.get("toolCallId")
                if not isinstance(call_id, str) or not call_id:
                    report["availability"] = "partial"
                    report["malformed_events"].append(line_number)
                    continue
                call = calls.get(call_id)
                if call is None:
                    report["availability"] = "partial"
                    report["conflicts"].append({"line": line_number, "kind": "tool result without unique invocation"})
                    name = entry.get("toolName")
                    call = {"name": name if isinstance(name, str) and name else "unavailable"}
                record_result(call_id, call, entry.get("result"), entry.get("isError"), ref)
                continue
            # Persisted session-entry form remains supported for old versions.
            if kind == "message" and isinstance(entry.get("message"), dict):
                message = entry["message"]
                role = message.get("role")
                if role == "assistant":
                    record_assistant_metadata(message, ref)
                    for content in message.get("content", []):
                        if isinstance(content, dict) and content.get("type") == "toolCall":
                            call_id, name = content.get("id"), content.get("name")
                            if isinstance(call_id, str) and isinstance(name, str):
                                call = remember_call(call_id, name, content.get("arguments", {}), ref)
                                if call is not None:
                                    classify_start(call_id, call, ref)
                elif role == "toolResult":
                    call_id = message.get("toolCallId")
                    call = calls.get(call_id) if isinstance(call_id, str) else None
                    if call is not None:
                        record_result(call_id, call, {"structuredContent": message.get("details"),
                                                      "content": message.get("content")},
                                      message.get("isError"), ref)
    if not state["settled"]:
        report["metadata"]["worker_status"] = "unavailable (no agent_settled)"
    if run_facts:
        add({"type": "session_metadata", "kind": "run_metadata", **run_facts}, run_ref)
        report["run_metadata"] = run_facts
        if "wall_duration_ms" in run_facts:
            report["investigation_facts"]["wall_duration_ms"] = run_facts["wall_duration_ms"]
            add({"type": "termination", "reason": "measured runner wall duration",
                 "wall_duration_ms": run_facts["wall_duration_ms"], "active_duration_status": "unavailable"}, run_ref)
        runner_failed = run_facts.get("timed_out") is True or (
            isinstance(run_facts.get("process_exit_code"), int) and run_facts["process_exit_code"] != 0)
        if runner_failed and not state["settled"]:
            status = "failed (runner timeout)" if run_facts.get("timed_out") is True else "failed (runner exit)"
            report["metadata"]["worker_status"] = status
            add({"type": "worker_lifecycle", "status": "failed"}, run_ref)
    if decision is None:
        rows.append({"type": "supervisor_decision", "status": "missing", "session_id": state["session_id"] or "unavailable",
                     "worker_id": state["worker_id"] or "unavailable"})
        limitations.append("Supervisor decision missing; Pi events do not encode supervisor decisions.")
    else:
        rows.append({"type": "supervisor_decision", "status": "recorded", **decision,
                     "session_id": state["session_id"] or "unavailable", "worker_id": state["worker_id"] or "unavailable"})
    if report["malformed_records"]:
        limitations.append("Malformed JSONL records were skipped; transcript coverage is partial.")
    if report["malformed_events"]:
        limitations.append("Malformed Pi event records were skipped or prevented exact derived outcomes; transcript coverage is partial.")
    if report["conflicts"]:
        limitations.append("Conflicting/missing tool-call associations were excluded from derived classifications.")
    if report["metadata"]["worker_status"] == "unavailable":
        limitations.append("Worker lifecycle unavailable: terminal agent_settled was absent or invalid.")
    by_type, by_type_status = {}, {}
    for row in rows:
        event_type = row.get("type", "unavailable")
        by_type[event_type] = by_type.get(event_type, 0) + 1
        if isinstance(row.get("status"), str):
            statuses = by_type_status.setdefault(event_type, {})
            statuses[row["status"]] = statuses.get(row["status"], 0) + 1
    report["observed_ledger_rows_only"] = {
        "basis": "emitted ledger-row observations (including explicit missing-decision record); not completeness or total activity",
        "by_type": by_type,
        "by_type_and_status": by_type_status,
    }
    limitations.extend((
        "Only successful read-tool results establish retrieval; requested ranges are not verified returned extents.",
        "Search output, analyzer output, and packet-builder output are never interpreted as source reads.",
        "Session-entry records do not expose terminal worker status; agent_end alone is nonterminal.",
        "Source retrieval does not establish substantive function review; active duration is unavailable unless independently measured.",
    ))
    return rows, report


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("transcript", type=pathlib.Path)
    parser.add_argument("--repository-root", type=pathlib.Path, required=True)
    parser.add_argument("--ledger", type=pathlib.Path, required=True)
    parser.add_argument("--report", type=pathlib.Path, required=True)
    parser.add_argument("--run-metadata", type=pathlib.Path)
    parser.add_argument("--supervisor-decision", type=pathlib.Path)
    args = parser.parse_args(argv)
    run_metadata = json.loads(args.run_metadata.read_text(encoding="utf-8")) if args.run_metadata else None
    decision = json.loads(args.supervisor_decision.read_text(encoding="utf-8")) if args.supervisor_decision else None
    rows, report = import_session(args.transcript, args.repository_root, run_metadata=run_metadata,
                                  run_metadata_path=args.run_metadata, supervisor_decision=decision,
                                  supervisor_decision_path=args.supervisor_decision)
    args.ledger.write_text("".join(json.dumps(row, sort_keys=True) + "\n" for row in rows), encoding="utf-8")
    args.report.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
