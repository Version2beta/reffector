#!/usr/bin/env python3
"""Bounded, eight-hour, sequential fresh-Pi supervisor; Python stdlib only."""
from __future__ import annotations

import argparse
import datetime as dt
import fcntl
import json
import os
from pathlib import Path
import signal
import subprocess
import sys

HERE = Path(__file__).resolve().parent
DEFAULT_PROVIDER = "openai-codex"


def utc_now() -> dt.datetime:
    return dt.datetime.now(dt.timezone.utc)


def stamp() -> str:
    return utc_now().isoformat(timespec="seconds")


def atomic_json(path: Path, value: dict) -> None:
    tmp = path.with_suffix(path.suffix + ".tmp")
    with tmp.open("w", encoding="utf8") as f:
        json.dump(value, f, indent=2, sort_keys=True)
        f.write("\n")
        f.flush()
        os.fsync(f.fileno())
    os.replace(tmp, path)


def audit(line: str) -> None:
    with (HERE / "run.log").open("a", encoding="utf8") as f:
        f.write(f"{stamp()} {line}\n")


def check_model_events(path: Path, provider: str, model: str) -> tuple[bool, str]:
    """Verify reported provider/model on completed assistant messages."""
    seen = 0
    settled = False
    errors = []
    try:
        with path.open(encoding="utf8") as f:
            for lineno, line in enumerate(f, 1):
                if not line.strip():
                    continue
                event = json.loads(line)
                if event.get("type") == "agent_settled":
                    settled = not event.get("aborted", False)
                if event.get("type") == "message_end":
                    msg = event.get("message") or {}
                    if msg.get("role") != "assistant":
                        continue
                    seen += 1
                    if msg.get("provider") != provider or msg.get("model") != model:
                        return False, (f"assistant message {seen} provider/model not verified: "
                                       f"provider={msg.get('provider')!r}, model={msg.get('model')!r}")
                    if msg.get("stopReason") in ("error", "aborted"):
                        errors.append(f"assistant message {seen} stopReason={msg['stopReason']}")
                if event.get("type") == "auto_retry_end" and not event.get("success", False):
                    errors.append("provider automatic retry failed")
    except (OSError, ValueError, TypeError) as exc:
        return False, f"cannot verify Pi JSON event stream: {exc}"
    if not seen or not settled:
        return False, f"missing verified assistant result or agent_settled (messages={seen}, settled={settled})"
    if errors:
        return False, "; ".join(errors)
    return True, f"verified {seen} assistant messages provider={provider} model={model}"


def terminate_group(proc: subprocess.Popen) -> None:
    if proc.poll() is not None:
        return
    try:
        os.killpg(proc.pid, signal.SIGTERM)
    except ProcessLookupError:
        return
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except ProcessLookupError:
            pass
        proc.wait()


def validate_repo(repo: Path) -> None:
    if not repo.is_dir() or not (repo / "go.mod").is_file() or not (repo / "internal" / "analyzer").is_dir():
        raise ValueError("expected existing Reffector repository with go.mod and internal/analyzer")
    if not (HERE / "TASK.md").is_file() or not (HERE / "state.md").is_file():
        raise ValueError("TASK.md and state.md must be beside ralph.py")


def main(argv: list[str] | None = None) -> int:
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--repo", required=True, type=Path)
    p.add_argument("--provider", default=DEFAULT_PROVIDER, help=f"Pi provider (default: {DEFAULT_PROVIDER})")
    p.add_argument("--model", required=True, help="Exact installed model ID for the selected provider")
    p.add_argument("--hours", type=float, default=8, help="Total wall-clock hours from first launch (maximum 8)")
    p.add_argument("--iterations", type=int, default=32, help="Cumulative iteration ceiling")
    p.add_argument("--minutes", type=float, default=25, help="Maximum wall-clock minutes per iteration")
    p.add_argument("--max-no-progress", type=int, default=3, help="Stop after this many consecutive unchanged handoffs")
    p.add_argument("--thinking", choices=["off", "minimal", "low", "medium", "high", "xhigh"], default="medium")
    p.add_argument("--pi", default="pi", help="Installed Pi executable")
    p.add_argument("--dry-run", action="store_true", help="Check inputs, print config; make no Pi calls")
    a = p.parse_args(argv)
    repo = a.repo.expanduser().resolve()
    try:
        validate_repo(repo)
    except ValueError as exc:
        p.error(str(exc))
    if not (0 < a.hours <= 8 and 1 <= a.iterations <= 128 and 0 < a.minutes <= 90 and 1 <= a.max_no_progress <= 20):
        p.error("hours must be (0,8], iterations 1..128, minutes (0,90], max-no-progress 1..20")
    if not a.provider.strip() or any(c.isspace() for c in a.provider):
        p.error("invalid provider identifier")
    if not a.model.strip() or "/" in a.model and not a.model.startswith(a.provider + "/"):
        p.error(f"provide the exact installed {a.provider} model ID, not another provider's model")
    if a.model.startswith(a.provider + "/"):
        a.model = a.model[len(a.provider) + 1:]
    if not a.model or any(c.isspace() for c in a.model):
        p.error("invalid model identifier")

    cmd_prefix = [a.pi, "--provider", a.provider, "--model", a.model,
                  "--thinking", a.thinking, "--no-extensions", "--no-skills",
                  "--no-context-files", "--no-mcp", "--no-session",
                  "--mode", "json", "--print"]
    if a.dry_run:
        print(f"DRY RUN: repo={repo} provider={a.provider} model={a.model} "
              f"hours={a.hours} iterations={a.iterations} minutes={a.minutes}")
        print("Pi launch:", " ".join(cmd_prefix), "<prompt containing TASK.md and state.md>")
        return 0

    lock_path = HERE / ".runner.lock"
    with lock_path.open("a+") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            print("Another supervisor already owns this loop directory.", file=sys.stderr)
            return 2
        record = HERE / "run_state.json"
        if record.exists():
            state = json.loads(record.read_text(encoding="utf8"))
            if state.get("repo") != str(repo) or state.get("provider") != a.provider or state.get("model") != a.model:
                print("Existing run uses different repository/provider/model; refusing to alter it.", file=sys.stderr)
                return 2
            # An existing deadline is never extended by changing --hours.
            if a.iterations != state["iterations_max"]:
                print("Existing run uses a different iteration cap; refusing reset.", file=sys.stderr)
                return 2
        else:
            if any((HERE / m).exists() for m in ("DONE", "BLOCKED", "TIMEBOX_EXPIRED")):
                print("Stop marker exists; inspect it before starting a new run.", file=sys.stderr)
                return 2
            deadline = utc_now() + dt.timedelta(hours=a.hours)
            state = {"repo":str(repo), "provider":a.provider, "model":a.model,
                     "started_at":stamp(), "deadline_at":deadline.isoformat(),
                     "iterations_max":a.iterations, "attempts":0, "stagnant_iterations":0}
            atomic_json(record, state)
        deadline = dt.datetime.fromisoformat(state["deadline_at"])
        logs = HERE / "logs"
        logs.mkdir(exist_ok=True)
        audit(f"SUPERVISOR_START attempts={state['attempts']} deadline={state['deadline_at']}")
        while state["attempts"] < state["iterations_max"]:
            if any((HERE / marker).exists() for marker in ("DONE", "BLOCKED", "TIMEBOX_EXPIRED")):
                audit("STOP marker present")
                print("DONE/BLOCKED/TIMEBOX_EXPIRED marker exists; stopping.")
                return 0
            remaining = (deadline - utc_now()).total_seconds()
            if remaining <= 0:
                (HERE / "TIMEBOX_EXPIRED").write_text(f"Eight-hour wall clock expired at {stamp()}; review state and logs.\n")
                audit("STOP total deadline expired")
                print("Eight-hour deadline reached; not marking work complete.")
                return 0
            state["attempts"] += 1
            atomic_json(record, state)  # consume attempt before launching, including crash/restart
            n = state["attempts"]
            base = f"iteration-{dt.datetime.now().strftime('%Y%m%d-%H%M%S')}-{n:03d}"
            events_path = logs / f"{base}.jsonl"
            stderr_path = logs / f"{base}.stderr.log"
            # Pi treats a --print argument starting with @ as a single file path.
            # Embed both files explicitly, refreshing state on every iteration.
            task_text = (HERE / "TASK.md").read_text(encoding="utf8")
            state_text = (HERE / "state.md").read_text(encoding="utf8")
            instruction = (
                f"You are iteration {n} of {state['iterations_max']}.\n"
                f"Loop directory: {HERE}\nReffector repo: {repo}\n"
                f"Selected provider: {a.provider}; model: {a.model}.\n"
                "Do only one bounded step, update state.md on disk, and stop. "
                "Do not spawn another supervisor.\n\n"
                f"=== TASK.md ===\n{task_text}\n=== END TASK.md ===\n\n"
                f"=== state.md ===\n{state_text}\n=== END state.md ===\n"
            )
            cmd = [*cmd_prefix, instruction]
            state_before = (HERE / "state.md").read_bytes()
            timeout_s = min(a.minutes * 60, remaining)
            audit(f"ITERATION_START n={n} timeout_s={timeout_s:.1f} events={events_path.name}")
            print(f"{stamp()} iteration {n} (max {timeout_s/60:.1f} min) → {events_path.name}", flush=True)
            try:
                with events_path.open("wb") as events, stderr_path.open("wb") as err:
                    proc = subprocess.Popen(cmd, cwd=repo, stdout=events, stderr=err, start_new_session=True)
                    try:
                        rc = proc.wait(timeout=timeout_s)
                    except subprocess.TimeoutExpired:
                        terminate_group(proc)
                        rc = 124
                    except KeyboardInterrupt:
                        terminate_group(proc)
                        audit(f"STOP interrupted iteration={n}")
                        print("Interrupted; logs and state preserved.", file=sys.stderr)
                        return 130
            except OSError as exc:
                audit(f"STOP failed to launch Pi: {exc}")
                print(f"Cannot launch Pi: {exc}", file=sys.stderr)
                return 2
            okay, summary = check_model_events(events_path, a.provider, a.model)
            audit(f"ITERATION_END n={n} rc={rc} provider_check={okay} detail={summary}")
            if rc == 0 and okay:
                if (HERE / "state.md").read_bytes() == state_before and not any(
                        (HERE / marker).exists() for marker in ("DONE", "BLOCKED")):
                    state["stagnant_iterations"] = state.get("stagnant_iterations", 0) + 1
                    audit(f"NO_PROGRESS n={n} consecutive={state['stagnant_iterations']}")
                else:
                    state["stagnant_iterations"] = 0
                atomic_json(record, state)
                if state["stagnant_iterations"] >= a.max_no_progress:
                    msg = f"No state.md handoff change for {state['stagnant_iterations']} iterations; stopping for review."
                    (HERE / "BLOCKED").write_text(msg + "\n", encoding="utf8")
                    audit(f"STOP {msg}")
                    print(msg, file=sys.stderr)
                    return 2
            if rc != 0 or not okay:
                problem = f"Iteration {n} failed (rc={rc}; provider verification={summary}). Review {events_path.name} and {stderr_path.name}."
                (HERE / "BLOCKED").write_text(problem + "\n", encoding="utf8")
                print(problem, file=sys.stderr)
                return rc if rc != 0 else 2
        audit("STOP cumulative iteration ceiling reached")
        print("Iteration ceiling reached; review work. No completion asserted.")
        return 0


if __name__ == "__main__":
    sys.exit(main())
