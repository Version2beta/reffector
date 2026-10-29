# Rclone `averageLoop` dependency retry

**Disposition:** `REGRESSION_REPRODUCED_PRE_FIX_AND_FIXED; EFFECTREPORT_PRESENT; RELEVANT_STATIC_PATH_FOUND_WITH_A_CANDIDATE_RC_DISPATCH_EDGE; CANONICAL_PACKET_BUILD_INCOMPLETE`.

The retry completed the offline regression and pre-fix Reffector analysis. The EffectReport contains the goroutine launch and reverse relationships through the accounting setup path. The requested full canonical packet was **not** completed: it was gracefully stopped after 60:05 of observed CPU time (60:39 wall time), before producing any output files. No packet size, emitted/deferred candidate status, or canonical packet result is claimed.

## Scope and preserved state

The analyzer input is the pinned pre-fix rclone revision `c1b5756eec76b9f0f7ee4da43fdbcae9cb73a2db`; the fix `c91c4cbbffdf34c62fc91d8bfda7c3d315d415e9` is its direct child. The original rclone checkout remained clean at `b551a5c1688282eb2ee4274dff96e95d91d075f4`. Tests and analysis used disposable snapshots and the isolated module cache. Module verification passed offline; Go commands used the installed `go1.27.2`, `GOPROXY=off`, `GOSUMDB=off`, `GOTOOLCHAIN=local`, and `-mod=readonly`. The source `go.mod`/`go.sum` files were unchanged.

The retry artifacts are under:

`/tmp/rclone-average-loop-preflight/dependency-cache-retry-20261028T205455Z/`

All 2,570 tracked source blobs in each pinned snapshot were verified. The separate pre-fix regression copy differs only in `fs/accounting/stats_groups_test.go`, which matches the fix's test file. The prior report `reports/rclone-average-loop-preflight.md` remains unchanged; it documents the earlier environment-blocked attempt. No Reffector implementation, test, dependency, or other report was changed for this retry. The requested file is the only repository file created by this report work; pre-existing dirty/untracked work was preserved.

## Source and regression result

The source review in the earlier preflight remains applicable: on the relevant sync/copy setup path, `newSyncCopyMove` calls `accounting.Stats`, which can create a stats group; pre-fix `NewStatsGroup` starts `averageLoop` eagerly. The loop uses its own background-derived cancellation context. The stop path is tied to transfer completion, so a zero-transfer group does not reach that transfer stop. `MoveDir` has early-return paths, so this does not apply to every RC move. These source facts establish a plausible lifecycle mechanism, not a measured daemon leak.

Offline tests in the disposable snapshots establish the regression:

- `go test -mod=readonly -count=1 ./fs/accounting` passed on both pre-fix and fix source.
- The fix's `TestNewStatsGroupDoesNotStartAverageLoop`, transplanted test-only into the separate pre-fix copy, failed at its intended assertion with: “NewStatsGroup must not start averageLoop for a zero-transfer group (issue #9567)”.
- The same focused test passed on the fix source.

Raw commands, exits, and output are in `logs/tests/` and the corrected outcome summary is `evidence/test-outcome-and-source-integrity-validated.json`. This is a test of group loop start behavior, not a runtime RC-request or goroutine-count measurement.

## Reffector EffectReport and reverse relationships

Reffector was built offline at `d766ee95bde0041549cedc1bdbc9a1a6d3a0ef29` (binary SHA-256 `7aa4a28f66271865d682547f64f996add3ab94e74b85c51bbd5579ef0e10ef6c`). The analysis ran only on the pinned pre-fix snapshot with `-all -effect concurrency.goroutine -max-sites 50000 -max-edges 200000 -max-depth 100 -json ./...` and exited 0.

The EffectReport is 457,954,303 bytes, SHA-256 `a1569ae03c77facbd421225d2524ba3166f84209a8911c66a107f5dc61ff2440`. It contains 303 selected packages, 215 sites, 4,996 graph nodes, 38,082 graph edges, and 215 reverse records. Its report-level site, edge, and depth truncation flags are false. These bounds describe this configured report; they do not make it a complete program call graph.

The historical operation is present:

- Site ID: `site:e5138d02e412d746677d7de95d593881fbb545a512acb21577ab979fa0993ca1`
- `(*StatsInfo)._startAverageLoop`, `fs/accounting/stats.go:389`
- Effect: `concurrency.goroutine`; API: `ssa.Go`; site marked not uncertain.

I independently extracted and cross-checked this site's reverse record against the EffectReport. It has 4,405 node references (4,404 unique nodes), 36,721 edge references, one origin, maximum depth 22, and no site/edge/depth truncation. The target reverse graph includes cycles/revisits and many candidate edges: 29,732 are marked candidate (`candidate_vta` 29,547; `candidate_function_value` 185), while 6,987 are established and two are synthetic. A `cycle` or `revisit` flag is graph structure, not evidence of a runtime loop or leak.

The reverse relationships include this static path:

`(*Server).handlePost → NewJob → (*Jobs).NewJob → (*Job).run → fs/sync:init#1$1 → rcSyncCopyMove → Sync/CopyDir → runSyncCopyMove → newSyncCopyMove → accounting.Stats → StatsGroup → NewStatsGroup → startAverageLoop → _startAverageLoop`.

The critical qualification is that `(*Job).run → fs/sync:init#1$1` is `candidate_vta` (`candidate=true`), not an established call. The surrounding direct source call edges are marked established in the report, including the path from `NewStatsGroup` to `startAverageLoop` and from `startAverageLoop` to the goroutine site. The `rcSyncCopyMove` branches to `Sync`, `CopyDir`, and `MoveDir` are also reported as established call sites, but the graph does not establish which branch a particular request takes or its runtime conditions. This is useful static evidence of a relevant path; it is not proof that the candidate RC dispatch executes for this case, that a zero-transfer request leaves a loop running, or that a production leak occurred.

Evidence for this extraction and its limits:

- `evidence/analysis-retry/effect-report-summary.json` — report identity, coverage, truncation, target site.
- `evidence/analysis-retry/average-loop-target-reverse-row.json` — target reverse record extracted directly from the report.
- `evidence/analysis-retry/average-loop-target-reverse-graph.json` — the referenced target nodes and edges.
- `evidence/analysis-retry/average-loop-target-reverse-inspection.json` — target identity, edge resolution counts, and selected path edges with IDs and source locations.
- `evidence/analysis-retry/average-loop-reverse-extraction-validation.json` — confirms extracted node/edge ID sets exactly match the target reverse references and all edge endpoints are present.
- `logs/analysis-retry/report-stream-summary-validated.result` — independent streaming scan of the full report exited 0 in 25 seconds.

**Analyzer correctness and effect-detection coverage are separate from packet-builder performance.** For this case, the analyzer emitted the expected goroutine site and reverse relationships within the configured report scope; I found no correctness failure in those records. The dynamic RC handler dispatch remains a candidate relationship and runtime lifecycle behavior remains unproven. The packet build's long runtime does not negate or establish analyzer correctness.

## Separate performance finding: canonical packet build

**Finding `PERF-1`:** the full investigation-packet command did not reach artifact emission within the observed run. The exact command is preserved in `logs/packet-build/command.txt`; it used the full EffectReport, `--max-candidates 100`, and `--max-packet-bytes 48000`.

At termination, the wrapper recorded `elapsed_seconds=3639` (60:39) and `exit_status=143`, the expected shell status after the requested SIGTERM. A `ps` snapshot recorded 60:05.28 of CPU time. The diagnostic `ps` snapshot reported RSS 52,544 KiB; the simultaneous macOS `sample` report measured physical footprint 2.4 GiB, peak 3.5 GiB. These are the recorded metrics from different macOS memory measures. The output directory `evidence/investigation-packets-default/` did not exist; no partial packet artifact was produced. Before termination stdout was empty; stderr contains only the shell's `Terminated: 15` notice after SIGTERM.

The one-second macOS sample captured Python in `_json`'s `encoder_listencode_obj`, key sorting (`list_sort_impl`), and Unicode escaping. That is direct evidence that the sampled instant was JSON serialization, not graph traversal. The output directory was absent, so `main()` had not reached its post-`build()` output-writing phase. The source has multiple serialization points inside `build()`—canonical packet-size measurement/reduction and the full-report fingerprint—so the sample does **not** identify which `canonical_json` call was active or whether packet reduction had finished. No cause beyond this observed serialization path is asserted.

The process was stopped by sending SIGTERM to PID 95517 only; the wrapper wrote the exit record and exited. The command, process/memory snapshot, sample, stdout/stderr, and exit record are preserved under `logs/packet-build/`, especially `stop-diagnostic-context.txt`, `stop-diagnostic-sample-full.txt`, and `result`. No other process was terminated, and no completed artifacts were deleted.

## Separately labeled bounded diagnostic proposal — not run

Do not rerun the full 215-site build. For a diagnostic only, create a **new, clearly labeled derived EffectReport** containing the exact target site, its full reverse record, and all of that record's original referenced nodes and edges (no new relationships). Record the parent report SHA-256 and explicitly mark the reduced scope so the result cannot be mistaken for the canonical full-report packet. Validate its schema and reference closure before invoking the existing CLI with `--max-candidates 1 --max-packet-bytes 48000`, writing to a new output directory such as `evidence/investigation-packets-target-only-diagnostic/`.

This is not a replacement for the canonical full-report result. The target reverse graph still contains 36,721 edges, and `--max-candidates` alone is not a wall-time bound: the builder constructs candidate data for the supplied sites before emission selection. If this diagnostic is later authorized, impose an external short stop (for example, five minutes or 4 GiB physical footprint, whichever comes first), preserve any output, and report interruption as incomplete. Do not alter `effects.json` or infer absent relationships from a reduced-scope result.

## Gate summary

| Gate | Result |
|---|---|
| A — pinned revisions, disposable snapshots, offline module integrity | **Pass** |
| B — source-grounded lifecycle and regression | **Pass for the targeted `NewStatsGroup` behavior; no production leak measurement** |
| C — pre-fix/fix package and regression tests | **Pass; pre-fix targeted assertion fails, fix passes** |
| D — pre-fix Reffector EffectReport | **Pass; report emitted and independently checked** |
| E — target site and reverse-call evidence | **Present; includes a candidate RC dispatch edge and is not runtime proof** |
| F — full canonical investigation packet | **Incomplete; gracefully stopped after 60:05 CPU / 60:39 elapsed, no output files** |

The original preflight report and all pre-existing dirty/untracked work remain untouched. No model/provider calls, benchmarks, subagents, commits, or remote actions were performed.
