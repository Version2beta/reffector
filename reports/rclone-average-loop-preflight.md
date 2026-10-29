# Rclone average-loop historical preflight

**Disposition: `ENVIRONMENT_BLOCKED`.** The source path supports the suspected zero-transfer lifecycle defect, but the offline environment lacked dependencies needed both to execute the regression test and to produce a Reffector EffectReport. Therefore this preflight cannot determine whether Reffector provides useful evidence for this case. No packet or runtime leak measurement was obtained.

## Scope and preserved state

The target was the pinned pre-fix rclone revision `c1b5756eec76b9f0f7ee4da43fdbcae9cb73a2db`; the proposed fix `c91c4cbbffdf34c62fc91d8bfda7c3d315d415e9` is its direct child. The actual rclone checkout was clean at descendant `b551a5c1688282eb2ee4274dff96e95d91d075f4` and remained clean. Testing and analysis used disposable archives under `/tmp/rclone-average-loop-preflight/`. The fix's regression test was ported unchanged, test-only, into the separate disposable pre-fix test copy. No compatibility workaround or dependency/version adjustment was made. No repository production code or dependency files were changed.

## Source-grounded lifecycle observations

These are source observations from the pre-fix revision, not a runtime reproduction:

1. The RC HTTP handler resolves the requested path and calls `jobs.NewJob(ctx, call.Fn, in)` (`fs/rc/rcserver/rcserver.go:273-300`). `fs/rc/jobs/job.go:217-227, 249-309` assigns a default `job/<id>` stats-group name and puts it in the job context. The registered RC `sync`, `copy`, and `move` handlers call `Sync`, `CopyDir`, or `MoveDir` (`fs/sync/rc.go:9-19, 47-57`).
2. `Sync` and `CopyDir` run `runSyncCopyMove`, which constructs `newSyncCopyMove` before `do.run()` (`fs/sync/sync.go:1352-1389`); file-based `MoveDir` fallback reaches the same path (`:1403-1431`). Setup calls `accounting.Stats(ctx)` while constructing its check/upload/rename queues (`:184-193`). `Stats` resolves the group's `StatsInfo`, creating it through `NewStatsGroup` when absent (`fs/accounting/stats_groups.go:280-309`). On these paths the group is created before transfer work. A same-filesystem move no-op or successful server-side `DirMove` returns early and does not establish this stats-group path; the report does not claim every RC move reaches it.
3. Pre-fix `NewStatsGroup` unconditionally calls `stats.startAverageLoop()` (`fs/accounting/stats_groups.go:303-309`). `_startAverageLoop` creates a cancellable context from `context.Background()` and launches `go s.averageLoop(ctx)` (`fs/accounting/stats.go:382-390`); the loop exits on cancellation (`:351-377`). This loop is not derived from the job or sync context.
4. `NewTransfer` and `NewTransferRemoteSize` also start the loop on demand (`fs/accounting/stats.go:821-840`). `Transfer.Done` calls `DoneTransferring` for non-check transfers (`fs/accounting/transfer.go:96-132`), whose last-transfer path stops the loop (`fs/accounting/stats.go:842-856`). With no transfer, that stop path is not reached. The no-transfer sync exit logs “There was nothing to transfer” and cancels its own contexts (`fs/sync/sync.go:1030-1038`); that cancellation does not cancel the loop's separately created background context. `Job.run` finishes the job by calling its RC function and recording the result (`fs/rc/jobs/job.go:107-115`), with no stats-group shutdown in that function.

This establishes a plausible, source-supported lifecycle leak mechanism for a zero-transfer RC job. It does **not** establish a measured daemon goroutine count, memory growth, scheduling behavior, or an independently reproduced production incident. The fix commit's regression test checks only group-loop start/stop state, not the full RC request path.

## Gate results

| Gate | Result | Evidence |
|---|---|---|
| A — pinned revision and isolation | **Pass** | Fix is the pre-fix commit's direct child. Archive/tree and key-file hashes are recorded in the preflight evidence directory. The working rclone checkout remained clean. |
| B — independent source path | **Source-supported; runtime unconfirmed** | The RC job, sync setup, eager start, detached loop context, and transfer-only stop path are present in the pinned source as cited above. |
| C — regression test pre/fix | **Blocked** | The pre-fix accounting package, transplanted regression, fix regression, and fix accounting package commands all failed during module setup with `GOPROXY=off`; none reached test assertions. The test outcome is unknown, not a pass or a reproduced failure. |
| D — Reffector EffectReport | **Blocked** | Reffector built offline from current Reffector HEAD `d766ee95bde0041549cedc1bdbc9a1a6d3a0ef29`. The documented analysis command against the exact pre-fix snapshot exited 1 because required rclone modules were unavailable locally (`cloud.google.com/go/auth@v0.20.0: module lookup disabled by GOPROXY=off`). No EffectReport was emitted. |
| E — canonical packet and evidence mapping | **Not run; blocked by D** | Without an EffectReport, there are no canonical site/relationship IDs, resolution labels, packet source references, coverage/truncation values, or 48,000-byte packet result to inspect. No evidence was hand-authored to fill the gap. |

The `concurrency.goroutine` label, if emitted, would denote a recognized goroutine launch, not proof that the goroutine completes or leaks. Any candidate relationship would also remain a candidate. This run produced neither such a label nor candidate/established relationships, so no analyzer claim about the relevant path is warranted.

## Commands and validation

Environment: Go `go1.27.2 darwin/arm64`; `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local`. Test commands additionally used `-mod=readonly -count=1`.

- Reffector build: `go build -mod=readonly -o /tmp/rclone-average-loop-preflight/reffector ./cmd/reffector` — **passed**.
- Analysis attempted only on `source-analysis-pre-fix`: `reffector -all -effect concurrency.goroutine -max-sites 50000 -max-edges 200000 -max-depth 100 -json ./...` — **blocked**, exit 1.
- Tests attempted in disposable copies: `go test -mod=readonly -count=1 ./fs/accounting` on pre-fix and fix, and `go test -mod=readonly -count=1 ./fs/accounting -run '^TestNewStatsGroupDoesNotStartAverageLoop$'` on the patched pre-fix copy and fix copy — **all blocked during dependency setup**. Go reported `module lookup disabled by GOPROXY=off`; for example, `cloud.google.com/go/auth@v0.20.0` was missing during analysis, and `github.com/stretchr/testify` and other modules were missing for package tests. No network fallback was used. `go.mod`/`go.sum` hashes in the disposable copies were unchanged.
- No packet builder, benchmark, model/provider, or subagent was run.

Full logs, exact source snapshots, the test-only patch, commands, hashes, and the failed empty stdout capture are under `/tmp/rclone-average-loop-preflight/`. `evidence/effects.json` is zero bytes and must not be treated as an EffectReport.

## Next step

Rerun the same pinned-source regression and analysis gates in an environment whose local module cache already contains the required rclone dependencies, keeping network access disabled. Only if the report includes the goroutine site and an untruncated reverse path to `NewStatsGroup` should the canonical 48,000-byte packet be built and inspected. Until then, Reffector's usefulness for this defect remains undetermined; this is not readiness for a controlled model evaluation.
