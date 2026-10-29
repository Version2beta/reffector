# Investigation observability implementation

## Analyzer stats and coverage

`-stats` continues to write its JSON summary to stderr. It includes selected root package paths, source modules (module path and version, without local replacement paths), host OS/architecture, `GOOS`/`GOARCH` build-target values (environment override or host fallback), test-selection mode, package-load status, and configured site/depth/edge caps. The summary also includes measured analyzer wall duration. Duration is not added to normal JSON output, which remains deterministic. JSON values are preserved when duration is added to the stats envelope.

Existing function counts describe the selected source-function inventory before output filtering. `source_functions` counts all selected functions, including those with no recognized effects; `no_effects_no_uncertainty` is not an unanalysed category. The JSON explicitly marks the whole-source function denominator `unavailable` and gives the reason: unselected packages, build-excluded files, alternate platforms, and other source variants are not enumerated. No coverage percentage is emitted. Selected package roots are scope metadata, not source coverage.

Effect coverage distinguishes selected-source and dependency operation sites, unresolved sites, filtered and emitted sites, emitted established and candidate edges, and graph truncation flags. Edge counts are explicitly emitted counts and must be interpreted with the truncation metadata and configured caps.

The analyzer does not inspect VCS metadata or source snapshot provenance. `source_revision_status` is therefore `unavailable`, with a reason; source-module identity is not a source revision or content digest. Custom build tags, CGO configuration, and detailed per-package failure/skip records are not currently exposed. On package-loading failure, the command returns diagnostics and no partial stats object; it does not emit fabricated zero counts. The successful load status means the requested roots loaded, not that every possible package variant was analyzed. Environment variables are not copied into stats.

## External investigation JSONL ledger

The deterministic packet builder accepts optional externally supplied events with `--events events.jsonl`, one JSON object per line. The allowlisted event types and fields are:

- `reffector_invocation`: required `invocation_id`, `target_revision`, `argv` (argument vector), and `status`; optional integer `exit_code` and 64-hex-character `binary_sha256`.
- `source_read`: required canonical repository-relative `source_path` and `status` (`success` or `failed`); optional `start_line` and `end_line` together, and optional `reason`. Absolute paths, parent traversal, backslashes, and noncanonical paths are rejected.
- `source_search`: required `query`, `scope`, and `status`; optional nonnegative `matches`.
- `candidate`: required `candidate_id` and `status`.
- `reproduction`: required `candidate_id`, `attempt`, and `outcome`.
- `operation_group`: required `group_id` and `operation_ids` (string array).
- `termination`: required `reason` and `active_duration_status` (`measured` or `unavailable`); optional nonnegative `wall_duration_ms`, and optional nonnegative `active_duration_ms` when status is `measured`.

The validator rejects unknown event types/fields, malformed required values, partial or reversed source ranges, invalid durations, and duplicate termination records. `recorded_counts` summarizes supplied ledger records, including successful/failed read events and distinct successfully read paths, candidate IDs, and operation groups. If no event input is supplied, availability is `not_provided` and recorded counts are `null`, not zero. An explicitly supplied empty ledger has zero recorded events. Counts describe only supplied entries; the output explicitly says ledger completeness is not independently verified.

This is an input ledger, not an observer. The outer runner must provide actual commands, target revisions, reads, searches, candidate updates, reproductions, and termination evidence. `argv` is stored verbatim; callers must omit or redact sensitive argument values. No environment dump is accepted or collected. Wall duration and active duration are separate; active time is explicitly marked unavailable unless measured and supplied. There is no automatic capture of tools, shell commands, tests, or investigator active time.

Packet-builder reads used to create source excerpts remain separate from investigator `source_read` events; packet excerpts and unavailable-source markers describe the builder's own reads. A successful read event does not imply the whole file was reviewed.

## Validation

Focused commands run for this change:

- `go test ./cmd/reffector ./internal/analyzer` — stats metadata, unchanged ordinary JSON schema, and analyzer behavior.
- `python3 -m unittest discover -s tools/effect_investigation -p 'test_*.py'` — event validation and CLI ingestion, recorded counts, duration distinction, and missing-ledger semantics.
- `go build -o /tmp/reffector-observability-cli ./cmd/reffector` — CLI build without creating a repository artifact.
- `git diff --check` — whitespace validation.

These checks validate the local implementation and its input schema. They do not prove that an external runner supplied a complete or accurate event ledger, that all source variants were analyzed, or that a recorded command/reproduction behaved as described.
