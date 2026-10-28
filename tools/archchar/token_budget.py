"""Pre-generation token-count enforcement for model requests."""
from __future__ import annotations

import json
import os
import pathlib
import re
import subprocess
import time
from typing import Any

class TokenBudgetError(Exception):
    """Raised when token counting is unavailable or the request cannot fit."""


def _json_bytes(value: Any) -> bytes:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"), allow_nan=False).encode("utf-8")


def _safe_argv(argv: list[str]) -> list[str]:
    result: list[str] = []
    redact_next = False
    for arg in argv:
        if redact_next:
            result.append("<redacted>")
            redact_next = False
            continue
        if re.search(r"(?i)(api[-_]?key|token|secret|password|authorization|credential|private[-_]?key|bearer)", arg):
            if "=" in arg:
                result.append(arg.split("=", 1)[0] + "=<redacted>")
            elif arg.startswith("-"):
                result.append(arg)
                redact_next = True
            else:
                result.append("<redacted>")
            continue
        result.append(arg)
    return result


def _safe_text(value: str) -> str:
    value = re.sub(r"(?i)(bearer\s+)[A-Za-z0-9._~+/-]+=*", r"\1<redacted>", value)
    value = re.sub(r"\bsk-[A-Za-z0-9_-]{16,}\b", "<redacted>", value)
    return re.sub(r"(?i)((?:api[-_]?key|token|secret|password|authorization|credential|private[-_]?key)\s*[=:]\s*)[^\s,;]+", r"\1<redacted>", value)


def run_token_counter(
    command_json: str | list[str] | None,
    request: dict[str, Any],
    cwd: pathlib.Path,
    policy: dict[str, int] | None,
    timeout: int = 60,
) -> dict[str, Any]:
    if command_json is None:
        raise TokenBudgetError("model generation requires an explicit token-count command")
    try:
        argv = json.loads(command_json) if isinstance(command_json, str) else command_json
    except ValueError as error:
        raise TokenBudgetError("token-count command must be JSON argv") from error
    if not isinstance(argv, list) or not argv or not all(isinstance(item, str) and item for item in argv):
        raise TokenBudgetError("token-count command must be a nonempty JSON argv array")
    if not isinstance(policy, dict):
        raise TokenBudgetError("token budget policy must declare context, output, and safety limits")
    required = ("context_window_tokens", "output_reserve_tokens", "safety_margin_tokens")
    if any(not isinstance(policy.get(field), int) or isinstance(policy.get(field), bool) or policy[field] <= 0 for field in required):
        raise TokenBudgetError("token budget policy limits must be positive integers")
    payload = {
        "protocol": "reffector-archchar/token-count/v1",
        "request": request,
        "context_window_tokens": policy["context_window_tokens"],
        "output_reserve_tokens": policy["output_reserve_tokens"],
        "safety_margin_tokens": policy["safety_margin_tokens"],
    }
    allowed_environment = ("PATH", "HOME", "TMPDIR", "TMP", "TEMP", "LANG", "LC_ALL", "ANTHROPIC_API_KEY")
    env = {name: os.environ[name] for name in allowed_environment if name in os.environ}
    started = time.monotonic()
    try:
        process = subprocess.run(
            argv,
            cwd=cwd,
            env=env,
            input=_json_bytes(payload).decode("utf-8"),
            text=True,
            capture_output=True,
            timeout=timeout,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as error:
        raise TokenBudgetError(f"token-count command did not complete: {type(error).__name__}") from error
    if process.returncode != 0:
        detail = _safe_text(process.stderr[:1000])
        raise TokenBudgetError(f"token-count command failed ({process.returncode}): {detail}")
    try:
        result = json.loads(process.stdout)
    except ValueError as error:
        raise TokenBudgetError("token-count command did not return one JSON object") from error
    if not isinstance(result, dict):
        raise TokenBudgetError("token-count result must be a JSON object")
    input_tokens = result.get("input_tokens")
    if not isinstance(input_tokens, int) or isinstance(input_tokens, bool) or input_tokens < 0:
        raise TokenBudgetError("token-count result requires a nonnegative integer input_tokens")
    token_count_type = result.get("token_count_type")
    if token_count_type not in {"exact", "estimate"}:
        raise TokenBudgetError("token-count result must identify exact or estimated counting")
    model = result.get("model")
    if not isinstance(model, str) or not model:
        raise TokenBudgetError("token-count result must identify the model")
    runtime_context = result.get("context_window_tokens")
    if not isinstance(runtime_context, int) or isinstance(runtime_context, bool) or runtime_context <= 0:
        raise TokenBudgetError("token-count result must report the runtime context window")
    runtime_output = result.get("max_output_tokens")
    if not isinstance(runtime_output, int) or isinstance(runtime_output, bool) or runtime_output <= 0:
        raise TokenBudgetError("token-count result must report the runtime maximum output limit")
    if runtime_output < policy["output_reserve_tokens"]:
        raise TokenBudgetError("token-counter output limit is below the reserved output capacity")
    total = input_tokens + policy["output_reserve_tokens"] + policy["safety_margin_tokens"]
    return {
        "status": "within_budget" if total <= runtime_context else "over_budget",
        "token_count_type": token_count_type,
        "input_tokens": input_tokens,
        "model": model,
        "runtime_context_window_tokens": runtime_context,
        "runtime_max_output_tokens": runtime_output,
        "output_reserved_tokens": policy["output_reserve_tokens"],
        "safety_margin_tokens": policy["safety_margin_tokens"],
        "total_reserved_tokens": total,
        "declared_context_window_tokens": policy["context_window_tokens"],
        "context_window_tokens": runtime_context,
        "counter_elapsed_seconds": time.monotonic() - started,
        "counter_command": _safe_argv(argv),
    }


def require_within_budget(budget: dict[str, Any]) -> None:
    if budget.get("status") != "within_budget":
        raise TokenBudgetError(
            f"request exceeds token budget: input {budget.get('input_tokens')} + output reserve "
            f"{budget.get('output_reserved_tokens')} + safety margin {budget.get('safety_margin_tokens')} "
            f"= {budget.get('total_reserved_tokens')} > context {budget.get('context_window_tokens')}"
        )
