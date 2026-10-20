# Case B — Side-effect isolation

## Outcome and provenance

**Recommend considering one small, private indexed-pack classifier in prune planning. Reject broader plan/executor refactors of filesystem saving, upgrade recovery and lock refresh.** This is a read-only engineering proposal, not a defect finding or verified improvement. Nothing was implemented or tested.

Repository: `/Users/rob/Development/reffector/experiments/restic`, HEAD `dd09b6bd01af220b1d5d95c0d4d7d472e05e05c4`; initial and subsequent status checks were clean. Exact analyzer scope: `./internal/repository/... ./internal/backend/local/...`. All writes were confined to this case’s artifact directory.

Frozen executable: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector`, SHA256 `ad9f0ac67b2d512880197e24e4b6be7077553497750380a4af7bc6c8c138ff97`. `go version -m` reports revision `5553a1d13011bf638148661f6a23bf561c3eef5a`, `vcs.modified=true`, module `v0.0.0-20261016233039-5553a1d13011+dirty`; this is the supplied dirty Phase 1/2 build, not a clean-revision build. Go is `go1.27.2 darwin/arm64`, matching the binary compiler. Actual `-help` was inspected and saved.

Read the complete explicit Reffector skill, version `phase2-1`, hash `ea99f629183bfafaf5764bc9eee86902f5e574a39a2a55ebdf41284146173297`. Loaded context: default Pi and supplied global AGENTS instructions, user case contract, explicit skill only; read/bash/write tools (parallel wrapper used), no extensions/MCP or additional discovered context. Selected Pi identity: provider `openai-codex`, model `gpt-6.1-sol`, reasoning `high`, session `01a15a5c-bb67-7286-8a83-2f2ab065a9d2`. These environment values do not independently attest the backend model. Exact session-file path and provenance outputs are in `provenance.txt`.

## Evidence boundary

Every Go/Reffector load used `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly`. Default darwin/arm64 selection, no test variant loading. **Five analyzer queries loaded the full required scope successfully.** An earlier invocation rejected `-max-edges 0` before loading (exit 2); the corrected positive limit succeeded. Literal binary command lines, limits, execution order and exit status are in [commands.md](commands.md); raw JSON/stderr are retained alongside it.

[R] Coverage: six selected packages, 446 source functions; 51 identified operation sites comprise 38 selected-package and 13 dependency sites; 1,021 unresolved sites identified. Filesystem survey emitted all 33 matching sites with no site/edge truncation; depth intentionally limited to zero. Remove-boundary query emitted nine matches with depth truncation. Some API-substring matches arise from interface type names, not a removal operation: `ParallelRemove`’s `Connections` call is one such match. Categories and counts are not safety findings.

[R] Rename expansion has one site and no caller edges or truncation; interface callers are not thereby absent. Root-filtered prune listing expansion has one site, two nodes and one edge, untruncated within that filter. Removing the root filter yields one shared site, 12 nodes/13 edges, depth truncated at two, no site/edge truncation. Interface targets remain unresolved; no concrete edge from repository listing to `Local.List` is established. Dependencies appearing in output are not selected-package coverage. Empty matches or effect-free unresolved calls cannot establish purity.

## Prioritized candidates

### 1. Indexed-pack action classification — smallest worthwhile proposal

[R] `prune-list.json` and `list-callers.json` establish `PlanPrune` (`prune.go:136`) → `decidePackAction` (`:399`) → `Repository.List` → `backend.(Backend).List` (`repository.go:963`), site `site:cb4d488e0c3add7444053d993ca299849465a2f6afb55666531c107fa0850912`. RepairIndex, Checker.Packs, Init and other callers share that direct boundary; they do not share prune policy.

[I/F] `prune.go:386–574` mixes backend inventory, size consistency, compression/action choices, statistics, progress, and destructive mutation of the remaining-index map. However, `PlanPrune` and `PrunePlan.Execute` already separate planning from consequential writes/deletions. The useful residual seam is only **`:442–469`, indexed-pack compression/action selection**.

Before: the listing callback both decides remove/keep/repack-candidate/small-candidate and applies those decisions to counters and collections. Proposed after: a private value-only helper reads `packInfo`, observed size, captured repository version, target size and relevant option fields, returning one of those four choices plus `mustCompress`. It receives no repository/printer and invokes no option callback. The existing callback retains unindexed-pack handling (`:400–407`), the size guard (`:409–415`), statistics, diagnostics, collection/map mutation and progress. Global candidate sorting, minimum small-pack count, repack budgets, missing-pack checks and execution ordering remain unchanged.

[F] Backend List’s contract (`backend.go:50–58`) calls its callback in the invoking goroutine. Classification stays synchronous inside that callback: no new inventory snapshot, stale preflight, asynchronous command queue or authority boundary. The CLI holds its exclusive lock across planning/execution (`cmd_prune.go:174–183,219–246`); library callers’ locking is not established here. Extraction cannot make existing plans safe against arbitrary concurrent mutation.

**Independent correctness obligations:** preserve branch precedence—unused packs are removed before cacheable-only rules; compression depends on version/type/options; mixed packs remain candidates; size equality meets the keep threshold. Preserve size-guard failure before statistics mutation, error identity, map deletion timing, cancellation propagation, and Execute’s repack/index-before-removal protocol. The helper alone proves none of those orchestration guarantees.

Expected payoff is cheap table tests of interacting rules without assembling repository data. Tradeoff: four result cases and result plumbing may add more indirection than clarity. Reject the proposal if it duplicates predicates, scatters safety checks, or cannot materially simplify direct rule tests. No measured benefit is claimed.

### 2. Local.Save — reject detached filesystem plan

[R/F] `local.go:170` file Sync and `:180` Rename (site `site:9a8cbcbfba99c06fc0ee41fa4c19d4b53c29a2e634c66fca321f022a51a01a3c`) belong to one temporary-file lifetime: copy/length check, unsupported-sync policy, close-before-rename, directory sync, permissions and deferred cleanup (`:148`). Splitting observations from execution obscures resource ownership and temporal guarantees; filename computation alone offers negligible payoff. Unix and Windows helper policies differ. Keep this protocol together.

### 3. UpgradeRepo — reject trivial pure transformation

[R/F] `upgrade_repo.go:61,75,100–101` creates a backup and cleans it after success. Conditional backend removal (`:38`) and recovery removal (`:91`) are independent unresolved sites; `UpgradeRepo :81` calls `upgradeRepository`. Extracting `cfg.Version=2` adds little, while separating capability decisions or backup ownership could obscure recovery. Keep orchestration intact.

### 4. Lock refresh — reject temporal split

[R/F] `lock_file.go:321` adopts replacement ownership; `:351` and `:357` independently clean replacement locks on different failure paths. `refresh` and `refreshStaleLock` share adoption. The stale-refresh protocol deliberately observes old-lock existence before and after replacement creation plus a wait (`:325–361`). A detached plan would hide why both observations matter.

## Validation, ledger and remaining uncertainty

Existing test source inspected, **not executed**: `TestPrune`, `TestPruneSmall`, `TestPruneMaxUnusedDuplicate`; CLI `TestPruneWithDamagedRepository` asserts `ErrPacksMissing`, and `TestEdgeCaseRepos` includes missing-unused-data, mixed packs and duplicates. This is counterevidence to any claim of absent safety coverage. `TestNoSpacePermanent` checks Save’s ENOSPC error identity; local tests invoke a generic backend suite. Upgrade tests cover success and induced upload/restore failure.

Proposed validation: classifier decision-table tests, focused existing prune tests, then orchestration checks for size mismatch, partial listing failure/cancellation, unchanged statistics, and no deletion after planning failure. These are ideas, not coverage or passing results.

Evidence ledger: **[R]** raw analyzer reports; **[F]** prompted follow-up in local.go, local_unix.go/local_windows.go, upgrade_repo.go/tests, lock_file.go, repository.go and backend.go; **[I]** initial prune.go inspection and related prune/CLI test and lock-contract source searches. `candidates.md` records evolving decisions. Only cited portions of larger files were inspected. Source correctness, runtime behavior and cross-platform execution remain unproven. Other repository mechanisms were not deeply assessed. Tokens/cost unavailable. No app, tests, network, source edits, commits or historical diffs were produced or changed.

Completion: investigation and report completed within the supplied wall-clock budget; precise elapsed time is recorded in `completion.json`.
