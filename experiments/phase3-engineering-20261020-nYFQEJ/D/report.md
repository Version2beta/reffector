# Case D — Local replacement test design

## Outcome and provenance

**Recommend three focused test families around `(*Local).Save`; no implementation, reproduced defect, or new coverage is claimed.** The important distinction is failure before publication versus failure after publication. Keep persistence, temporary cleanup, retry policy, and repository rollback as separate responsibilities.

Repository `/Users/rob/Development/reffector/experiments/restic`, HEAD `dd09b6bd01af220b1d5d95c0d4d7d472e05e05c4`, was clean initially and at the final provenance check. Required scope `./internal/repository/... ./internal/backend/local/...` loaded successfully in all five analyzer runs (exit 0). No tests or application were executed; no source/worktree changes were made.

Verified binary SHA256 `ad9f0ac67b2d512880197e24e4b6be7077553497750380a4af7bc6c8c138ff97`. `go version -m` reports revision `5553a1d13011bf638148661f6a23bf561c3eef5a`, `vcs.modified=true`, module `v0.0.0-20261016233039-5553a1d13011+dirty`: this is the frozen dirty-source build, not a clean revision. Build/runtime toolchain: Go 1.27.2, darwin/arm64, CGO enabled. Verified full phase2-1 skill SHA256 `ea99f629183bfafaf5764bc9eee86902f5e574a39a2a55ebdf41284146173297`; read it completely and checked actual `-help`.

Selected Pi identity: provider `openai-codex`, model `gpt-6.1-sol`, reasoning `high`, session `01a15a5c-bb81-744c-949b-afe3617780bb`; session file `D/sessions/2026-10-20T17-29-49-185Z_01a15a5c-bb81-744c-949b-afe3617780bb.jsonl`. These identify the selected Pi model, not an independently attested backend. Context: default Pi and supplied global AGENTS.md instructions, explicit Reffector skill only; read/bash/write tools with parallel-call wrapper, no extensions/MCP/discovered repository guidance (AGENTS search returned none). Direct execution as requested. Raw provenance is in `provenance.txt`.

## Boundary and evidence ledger

- **[R] Publication:** `os.Rename`, `internal/backend/local/local.go:180`, `site:9a8cbcbfba99c06fc0ee41fa4c19d4b53c29a2e634c66fca321f022a51a01a3c`.
- **[R] Preparation:** `os.(*File).Sync`, `local.go:170`, `site:a29a7efc1de634a42ea73efe8372b58709101ea6bead1680668c8b1684ef9311`.
- **[R] Cleanup:** deferred `os.Remove`, `local.go:148`, `site:364f599c4b50e48542fbb2181ce43b4751c464d4159aef9219001da5b12ed301`.
- **[R] Commit attempt:** `fsyncDir` → `os.(*File).Sync`, `local_unix.go:19`, `site:0321b124e784f64538e3a8d8cabf18bb01f4fe0a492f580e56ac6b3d4e5f1fcb`.
- **[R] Unresolved client:** `backend.(Backend).Save`, `repository/repository.go:526`, `site:ab027af724decdd52ce0f1c0bcfc79e498fa9ced756b45fe428ebe157a862aa6`, reason `interface_dispatch`. Public and internal `SaveUnpacked` share `saveUnpacked`; this does **not** establish a concrete edge to Local.

**[F]** `local.go:107–200` copies and validates length, syncs/closes the temporary, renames, syncs the directory, then adjusts permissions. On error its defer attempts close/removal of the **temporary pathname**, not the destination. Thus a directory-sync error can coexist with complete new bytes at the destination; failure does not promise rollback. Cleanup errors are ignored, so unconditional cleanup under removal failure is not an established guarantee.

**[R/F]** Focused graphs show Save invoking its cleanup closure, `fsyncDir`, and `setFileReadonly`. The file and directory syncs are independent direct sites, not duplicated calls to consolidate. Rename's three effect categories describe one site. No reverse caller edge into Local.Save was reported, despite depth 3; interface clients remain unresolved.

**[F]** `backend/retry/backend_retry.go:165–192` rewinds before each attempt and deliberately skips Remove for atomic-replace backends. Local advertises that property at `local.go:87–92`. `backend/backend.go:13–18,96–97` establishes retry/permanent-error and atomic-replacement contracts. Atomic publication is not crash-durability proof.

## Existing tests versus proposals

**Existing, inspected but not run:** local `TestBackend` registers the generic suite. `backend/test/tests.go:506` checks successful save/readback and reader ownership; `:622` checks an inaccurate length produces an error, then explicitly deletes possible leftovers. It does not assert old-object preservation or an empty temporary set. Local `TestNoSpacePermanent` (`local_internal_test.go:17`) checks wrapped ENOSPC at temporary creation. Retry `TestBackendSaveRetry` and `TestBackendSaveRetryAtomic` (`backend_retry_test.go:19,57`) check rewind and no-Remove with mocks, not real replacement state.

**Proposed smallest useful set** (use tiny distinct old/new ConfigFile bytes and real temporary directories):

1. **`TestSaveFailurePreservesDestination`**, table: a reader fails after a prefix; a reader reaches EOF with incorrect advertised length. Seed old bytes first; call raw Local.Save. Assert the expected error, exact old bytes, no `config-tmp-*`, and no changes to an unrelated sentinel. Repeat with destination initially absent: no published file. This adds state assertions missing from the generic error-only test; it needs no new production seam.
2. **`TestSavePublicationBoundary`**, success and injected directory-sync EIO rows. On success, verify complete replacement and no temporary. On EIO, verify error identity, complete **new** bytes still present, no temporary, and untouched sentinel. Observe destination bytes inside the injected callback to establish that publication preceded the failure. A future minimal private directory-sync hook, defaulting to `fsyncDir` and restored with `t.Cleanup`, is needed: none exists now. Serialize hook tests, following the existing `tempFile` pattern; do not add a general filesystem abstraction. This is a proposed seam, not an implemented API.
3. **`TestLocalReplacementRetry`**, real Local wrapped by Retry plus an observing wrapper: first-attempt reader failure, first-attempt post-publication directory-sync EIO, and permanent tempfile ENOSPC. Inspect state before attempt two: old bytes for the first row, new bytes for the second, no temporaries in either. Require two attempts, rewound full new contents, and zero Remove calls. ENOSPC must stop after one attempt, preserve old bytes, and retain error identity; use the existing tempfile seam. Bound runtime with context and the existing fast-retry helper. The observer must delegate actual filesystem work, not replace Local with a success mock.

These tests add observation, not change responsibilities: Local owns temporary resources/publication; Retry owns rewind/retry decisions. They cannot prove power-loss durability, cross-platform atomicity, or behavior when cleanup itself fails. File-sync/close/rename failure injection remains untested by this minimal set.

## Priorities, alternatives, and falsifiers

1. **Selected:** real replacement-state tests above; highest payoff at the advertised atomic boundary.
2. **Deferred:** `repository/upgrade_repo.go:33–104` rollback/backup behavior. Existing `TestUpgradeRepoV2Failure` checks upload/reupload errors and backup path. Repository rollback is a separate obligation, not evidence that Local should restore old bytes after publication.
3. **Rejected:** another generic short-upload error test, mock-only no-Remove test, or creation-only ENOSPC classification test; these duplicate inspected checks without strengthening preservation evidence.
4. **Deferred:** Delete/listing and platform fallback matrix. Unix intentionally tolerates selected sync errors; Windows directory sync/readonly handling differs (`local_windows.go`). Do not demand uniform behavior.

Falsifiers: equivalent state assertions found elsewhere would reduce value; an approved contract requiring rollback on every Save error would change expected outcomes; an authorized filesystem fault harness could remove the need for a hook. No such evidence was established. No concurrent writers, crash harness, or Windows runtime was evaluated.

## Reproduction and limits

All commands below ran from the repository cwd with **`GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly`**, no dependency repair/downloads. JSON stdout and stderr were saved as paired artifacts `survey`, `rename`, `save-effects`, `save-unresolved`, and `backend-save-boundary`.

```sh
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect fs. -max-depth 0 -max-sites 24 -max-edges 50 -stats -json ./internal/repository/... ./internal/backend/local/...
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -site site:9a8cbcbfba99c06fc0ee41fa4c19d4b53c29a2e634c66fca321f022a51a01a3c -max-depth 3 -max-sites 4 -max-edges 40 -stats -json ./internal/repository/... ./internal/backend/local/...
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect fs. -package /internal/backend/local -function Save -max-depth 2 -max-sites 12 -max-edges 30 -stats -json ./internal/repository/... ./internal/backend/local/...
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -reason interface_dispatch -function Save -max-depth 1 -max-sites 10 -max-edges 25 -stats -json ./internal/repository/... ./internal/backend/local/...
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -reason interface_dispatch -operation '.Save' -package /internal/repository -max-depth 2 -max-sites 8 -max-edges 30 -stats -json ./internal/repository/... ./internal/backend/local/...
```

**[R]** Six selected packages, 446 source functions; 51 operation sites identified, 38 selected/13 dependency; 1,021 unresolved sites. Survey emitted 24/33 matches (sites/depth truncated). Rename emitted 1/1 and Save filesystem query 7/7, no reported truncation. Broad unresolved Save emitted 10/50, dependency-dominated, sites/depth truncated; refined query emitted 8/8, depth truncated. No edge-budget truncation. Filters constrain caller context; lack of reported uncertainty at an OS API does not establish correctness. Tests were inspected through source, not analyzer `-tests` loading.

**[I]** Generic suite reflection/registration was independently checked in `backend/test/suite.go:41–96`; other inspected files are cited above, plus `backend/rewind_reader.go`. Five analyzer calls; exact remaining tool operations are in the session transcript. Token/cost totals unavailable. Candidate evolution is retained in `candidates.md`. Investigation and report complete at 226.6 seconds after the recorded launch, before the 240-second investigation and 300-second report deadlines. Final repository status remained clean.
