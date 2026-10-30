# Reffector investigation workflow integration trial

## Outcome

One bounded provider-backed Pi 1.4 JSON-mode run completed successfully. Run metadata records start `2026-10-29T22:27:44.197Z`, end `2026-10-29T22:29:06.684Z`, and 82,488 ms wall time; the process exited 0 and did not time out. The importer converted its saved event stream into a schema-valid ledger and coverage report, including source reads, searches, Reffector/test invocations, assistant metadata, lifecycle, run duration, and an explicit supervisor stop decision.

This tests the instrumentation and workflow only. It does not establish restored or comparative defect-discovery effectiveness. No defect was confirmed and no comparative campaign was started.

## Pinned inputs and run

- Source snapshot: commit `39a4830e4d04341ff920956c9260a61042097b35`, limited to 20 files under `internal/analyzer`, `internal/effects`, and the Go module files. Every snapshot file hash matched both the saved manifest and the committed Git blob; the snapshot remained read-only and unchanged after the trial.
- Source manifest SHA-256: `a31125bb9a1d008041c22d05a7657bbc4d40c5ee640024fd7c09d0fdf5f455d2`.
- Reffector binary SHA-256: `c9224f2c833e40dfaf2fb1827f9b75850d8bcacdac5df73d62a2fe34af832d32`. Go build metadata identifies the same commit and `vcs.modified=false`.
- Pi invocation used `--mode json --model openai-codex/gpt-6.1-sol --thinking high --no-context-files --no-extensions --no-skills --no-prompt-templates --tools read,bash`, with a separate session directory under `/tmp`. Provider/API-key environment variables were not passed to Pi or its Bash tool. The prompt was saved under `/tmp/reffector-workflow-integration-39a4830-20261029/trial/integration-prompt.md` (SHA-256 `ccf187798f55b01c09bf7ed43701246c9f4344dee0b3dfa94959cd93628487e1`).
- Pi reported `openai-codex`, model `gpt-6.1-sol`, thinking `high`. Thirteen assistant-message metadata rows carried timestamps and reported token/cost usage; the importer retained only allowlisted numeric usage/cost values, not prompts or response text.
- Run ID: `workflow-trial-5d83659823a84259997ccb4e08a19a96`; Pi session ID: `01a189c6-b875-7139-a18d-1bacb80ee240`.
- Raw Pi events, saved session, run metadata, and imported outputs remain under `/tmp/reffector-workflow-integration-39a4830-20261029/trial/`; no transcript or large analyzer output was added to the repository.

The investigator's bounded analyzer command (event lines `175–179`) exited 0:

```sh
GOPROXY=off GOSUMDB=off GOFLAGS=-mod=readonly /tmp/reffector-workflow-integration-39a4830-20261029/bin/reffector -view effects -max-sites 32 -max-edges 64 -max-depth 3 -stats ./internal/analyzer
```

The observed report selected `github.com/Version2beta/reffector/internal/analyzer`, loaded successfully on darwin/arm64, identified 180 source functions and 54 matching operation sites, emitted 32 sites (13 selected-package sites), and reported 63 established and one candidate edge. Site, edge, and depth limits were all reached; tests were excluded and the whole-source function denominator was unavailable. These are static-analysis counts, not execution coverage. The counts came from analyzer/tool output (including the Pi Bash-log result at event lines `242–243`), not from repository source reads. The analyzer itself reported source revision unavailable; the external snapshot manifest and binary build metadata provide the pinned revision evidence.

## Imported observations

The event stream contained 1,689 JSON records, with no malformed records or conflicting call associations. The imported ledger contains 54 rows:

- 12 tool invocations and 12 results;
- two successful Reffector invocations (help and the bounded analysis command; query event lines `175–179`);
- three successful source searches, scoped to `internal/analyzer` and its two test files (event spans `371–375`, `477–480`, and `555–558`);
- four successful repository reads, with requested ranges for `internal/analyzer/analyzer.go` (1–215; event span `403–404`), `internal/analyzer/loading.go` (1–200; `431–432`), `internal/analyzer/selection_test.go` (1–180; `509–510`), and `internal/analyzer/loading_test.go` (1–200; `586–587`);
- two focused `go test` commands, both with exact exit status 0 (event spans `837–840` and `899–902`):
  - `go test -mod=readonly ./internal/analyzer -run '^TestSelectionPatterns$' -count=1 -timeout=45s`
  - `go test -mod=readonly ./internal/analyzer -run '^TestPrivateWorkspaceRebasesPaths$' -count=1 -timeout=45s`
- worker start and completed lifecycle rows, one measured termination row, 15 session-metadata rows, and one recorded supervisor-decision row.

Read rows preserve requested ranges only; they do not claim verified returned extents. The importer retained event-line references and linked each derived read/search/test/Reffector row to its tool-call events. The coverage report describes emitted rows only, not total activity or completeness.

One successful `read` call targeted a Pi-generated Bash output log outside the pinned source root (event lines 242–243; basename `pi-bash-9eadeb681f2567b0.log`). The importer records this as an out-of-root path and does not count it as a source read. This was a scope deviation from the prompt and is retained as a limitation; no log content was copied into the ledger.

The first offline import left commands with quoted regex pipes and safe Go environment prefixes unclassified. The importer was narrowly updated to recognize quoted search metacharacters and only exact allowlisted leading assignments (`GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`, `GOFLAGS=-mod=readonly`, `GOWORK=off`), without evaluating or retaining those prefixes. Real pipelines, substitutions, globs, unknown assignments, and complex commands remain unclassified. The same saved event stream was re-imported; no second provider run was made. The final coverage report has zero unclassified shell commands and one out-of-root read reference.

## Investigator-reported hypotheses and limits

The assistant's final response described two bounded behavior hypotheses, not defect findings:

1. It marked directory-selection behavior accepted within the examined cases: `Exact` suppresses implicit directory recursion while preserving explicit recursive/nonlocal patterns. The focused selection test exited 0. Absolute/parent paths, symlinks, and filesystem errors were not tested.
2. It marked workspace rebasing and cleanup-target binding accepted within the examined cases. The focused private-workspace test exited 0. The `AnalyzeReport` to cleanup-closure relationship remained `candidate_function_value`; the response did not claim that deletion succeeds, and removal-error/failure-path behavior was not probed.

These dispositions are self-reported in the assistant's final response (`events.jsonl:1686`); they are not encoded as candidate rows in the runtime ledger and were not independently validated as source-review conclusions. The importer correctly reports substantive source review and candidate dispositions as not established by Pi tool-call records. Successful source retrieval alone does not prove review. Active duration is unavailable; only runner wall time was measured.

## Supervisor decision

The explicit decision recorded in `supervisor-decision.json` at `2026-10-29T22:35:51Z` is **stop** (`scope_adequate: true`, `another_phase_warranted: false` for this task). No further phase is authorized within this task. Comparative discovery effectiveness remains unassessed and requires a separate decision. The decision is an external supervisor record, not inferred from the Pi transcript.
