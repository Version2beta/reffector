# Phase 3 quickstart

`python3 run.py` launches five independent Pi CLI sessions concurrently, each with an explicit prompt, fresh persistent session storage, and raw JSONL/stderr capture. Do not rerun in this directory: the launcher refuses existing run metadata. Copy this runner to a NEW artifact directory first; it generates five standalone `A/prompt.md` through `E/prompt.md` files.

Each per-case `launch.json` records the complete runnable Pi argv, cwd, safe environment overrides, deadlines and PID. For a manual run, use that argv from its cwd, redirect stdout/stderr into new artifact files, and update `timing.json` to the new start/deadlines. Do not use --continue/--resume/--fork. Preserve existing artifacts.

The launcher copies the installed binary byte-for-byte, records build provenance and source/skill fingerprints, verifies repository pins/cleanliness, and reads no earlier reports. Go loading is offline; Pi model inference still uses its configured provider. Requested model is openai-codex/gpt-6.1-sol, reasoning high. Actual model and effective reasoning must be checked in session records and worker shell output.

Read-only investigation is a prompt constraint, NOT an OS sandbox: bash can write. Source invariants are checked before/after. Extensions/MCP and automatic context discovery are disabled. Global instructions and (for Caddy) its AGENTS.md are appended explicitly; only Phase 2 Reffector skill is loaded. No cross-case findings are supplied.

Five-minute cap includes tool/runtime/report writing. Final minute is reserved by prompt; this JSON-mode CLI cannot inject deadline steering. Notes and all streamed events survive interruption. `completion.json` distinguishes process exit, assistant completion and report presence; a zero process exit alone is not proof of successful analysis. No automated follow-up investigation is performed on timeout.
