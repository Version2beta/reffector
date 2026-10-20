# Case B candidate ledger

## 1. Local.Save — explored; likely reject purity extraction
[R] fs-survey-valid.json: independent direct sites at local.go:128 mkdir, :170 file sync, :180 rename, cleanup closure :148 remove; platform helper fsyncDir local_unix.go:19.
[F] Save owns a live temporary file, copy length check, unsupported-sync policy, close-before-rename, directory durability, post-rename permissions, deferred cleanup and permanent-error classification. An externally executable plan risks splitting one resource lifetime and implies stale filesystem observations are safe. No nontrivial pure decision yet identified. Inspect tests and callers before final decision.

## 2. UpgradeRepo — explored; likely reject pure config preparation
[R] fs-survey-valid.json reports MkdirTemp :61, backup WriteFile :75, cleanup Remove :100/:101.
[F] upgrade_repo.go: version eligibility, raw backup, conditional remove based on atomic-replace capability, config version update, restore-on-failure are one recovery protocol. cfg.Version=2 extraction alone has negligible payoff; moving capability checks to a detached plan could obscure backend ownership. Tests not yet inspected.

## 3. Prune decision/collection — unresolved next lead
[I] prune.go PlanPrune already separates planning and execution; examine decidePackAction before proposing another layer. This is an independent source lead, not a filesystem site attributed by Reffector.

Preflight: HEAD dd09b6bd01af220b1d5d95c0d4d7d472e05e05c4; clean. Tool/skill hashes match prompt. First analyzer invocation rejected max-edges 0 (exit 2, no load); corrected positive limit loads full required scope successfully (exit 0). All loads offline/read-only. No source changes or tests run.

## 3 update — strongest conditional recommendation
[R] prune-list.json reports shared Repository.List -> backend.Backend.List boundary repository.go:963, site:cb4d488e0c3add7444053d993ca299849465a2f6afb55666531c107fa0850912, with decidePackAction caller prune.go:399. Interface dispatch unresolved; do not invent a concrete Local.List edge.
[F/I] prune.go:409-470 embeds indexed-pack safety check, compression rules and remove/keep/candidate/small-candidate classification in a backend listing callback alongside statistics, progress and index-map mutation. Small candidate helper could classify one indexed pack from value inputs synchronously, leaving listing, stats, diagnostics and mutation in the existing callback. No new inventory snapshot, no delayed executable command plan. Existing PlanPrune/Execute split already owns consequential deletion order. Proposed benefit: small table tests for precedence instead of creating repository data. Not implemented or verified.
[F] cmd/restic/cmd_prune.go:174-183 rejects no-lock except dry run and holds exclusive lock across planning/execution; this does not imply external library callers hold it.
[I] prune tests inspected: integration-style TestPrune, TestPruneSmall and TestPruneMaxUnusedDuplicate exist. No direct classifier tests identified in those files; test source inspection is not test execution.

## 4. Lock refresh protocol — rejected
[R] remove-boundaries.json reports separate RemoveUnpacked sites lock_file.go:321, :351, :357; refresh and refreshStaleLock share adoptReplacementLock, while cleanup branches are independent sites.
[F] lock_file.go:325-361 deliberately checks old lock existence both before and after replacement creation and a wait. Moving observations into a precomputed plan would obscure their temporal role. Keep protocol ownership/order together; no independent policy extraction demonstrated.

## 1 update
[R] save-rename.json depth 2 has one origin, no edges, no truncation; static absence of concrete callers is not evidence Save is unused (interface dispatch).
[F] local_internal_test.go TestNoSpacePermanent injects temp creation ENOSPC and checks permanent-error identity. local_test.go invokes generic backend suite, not inspected in depth. Keep Save lifecycle intact.

## 2 update
[F] upgrade_repo_test.go has success and induced new-upload plus restore-upload failure test; preserved backup path checked nonempty then removed. No assertion read of backup contents in inspected test. Extraction of version assignment remains rejected as trivial and obscuring recovery ownership.

## Final refinement and counterevidence
[F] backend/backend.go:50-58 guarantees List callbacks run in the invoking goroutine, at most once per file on failure; repository.go:963 parses IDs then forwards size. A synchronous value-only classifier needs no new concurrency or resource ownership.
[R] list-callers.json removes function-root restriction: one shared List site, 12 nodes/13 edges, depth truncated but no site/edge truncation. PlanPrune :136 -> decidePackAction :399 -> Repository.List :963 is explicit. RepairIndex, Checker.Packs, Init, resolveBlobsForPacks and searchKey also share this site; this is not a reason to move prune policy into Repository.List.
[I] cmd/restic/cmd_prune_integration_test.go TestPruneWithDamagedRepository checks ErrPacksMissing; TestEdgeCaseRepos includes missing-unused-data, mixed-packs and duplicates. Counterevidence reduces claims of missing coverage: recommendation is cheaper direct rule tests, not absent existing safety tests.
Selected smallest scope: extract only indexed-pack compression/action choice at prune.go:442-469; keep size-mismatch guard :409-415, unindexed handling :400-407, statistics, progress and map deletion in callback. Helper returns keep/remove/repack-candidate/small-candidate plus mustCompress, using packInfo, observed packSize, captured version, targetPackSize and relevant option fields; no repository/printer/callback invocation. Missing-pack handling and global repack limits stay in decidePackAction. Benefit remains a proposal, not verified. Reject if action-result plumbing duplicates policy or makes guard precedence harder to see.
No other lead examined beyond listed areas. Repository remains clean at final provenance capture; no tests executed, no app run, no edits.
