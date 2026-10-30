# Effect-directed rclone defect discovery

## Result

One issue is worth an independent Test 2: `fs/operations.RemoveExisting` treats **every**
`Fs.NewObject` error as “destination absent” and returns success with a no-op cleanup. This
behavior was reproduced against rclone's real local backend using a permission-denied lookup.
The helper's own comment promises rollback protection, and its interface contract says absence is
reported as `fs.ErrorObjectNotFound`; however, an end-to-end overwrite/data-loss consequence in a
server-side-copy backend was **not** reproduced. I classify this as a **plausible defect with
demonstrated error swallowing**, not as confirmed data loss.

A separate job-listener retention path remains a lower-confidence, source-only lead. No fixes were
made. The rclone checkout and Reffector source were not modified; this report is the only intended
repository change.

## Scope and evidence boundary

- Reffector: `feat/effect-directed-discovery` at
  `e0e62a296170881822b08bafd50c1f0b81104ce7`. The analysis binary was built from this commit
  (`go1.27.2`, `vcs.modified=false`, SHA-256
  `3c43f7fa177933e344124a191773e76015c41722c8045a6b9eca5abc4e1c1ad4`).
- rclone: clean worktree, branch `fix-parallel-reader-deadlock`, revision
  `b551a5c1688282eb2ee4274dff96e95d91d075f4`. Its disposable source snapshot was checked against
  2,808 tracked blobs (archive SHA-256
  `3887c77203cffef370ecb7b2530f123591d2fbd1c365f31cd76c069d68ecbb13`). The local
  `origin/fix-parallel-reader-deadlock` tracking ref is one commit ahead of this snapshot; that
  commit does not change `RemoveExisting`.
- Analysis and probes used the snapshot under `/tmp/reffector-bug-discovery/`, with `GOPROXY=off`,
  `GOSUMDB=off`, `GOTOOLCHAIN=local`, `GOWORK=off`, and `GOFLAGS=-mod=readonly`. No dependency
  download or remote mutation was performed.
- Static Reffector paths are leads, not proof of runtime execution. The prior PR #10172 pre-fix
  smoke at `81e7372f6fbedfa80592e9a564593d63a8dc32f3` is a separate known-case exercise and is not
  evidence for this finding.

## Effect and uncertainty inventory

The initial bounded query selected seven packages: `fs/accounting`, `fs/sync`, `fs/operations`,
`fs/rc/jobs`, `fs/rc/rcserver`, `backend/local`, and `fs/chunkedreader`.

- Reffector reported 805 source functions, 579 selected direct operation sites and 207 dependency
  operation sites across 24 effect groups. The selected sites include filesystem
  read/write/create/remove/rename, channel and synchronization operations, goroutine/task/wait/timer
  activity, resource acquire/release, network connection and process effects. One possible
  operation site was reported. The site inventory was not truncated; the initial reverse-depth view
  was truncated and was followed by bounded site queries.
- A separate unresolved audit emitted 2,534 uncertainty records: 2,488 unresolved-only and 46 with
  known effects plus uncertainty. Its leading reasons were unrecognized standard-library calls
  (1,079), interface dispatch (745), callbacks (235), reflection (225), function values (220),
  missing bodies (23), argument conditions (4), and state conditions (3). The direct inventory
  summary reports 2,489 unresolved sites; that differs by one from the audit's unresolved-only
  count, so these view-specific counts are not combined as a single deduplicated total.
- The audit identifies the `f.NewObject` call in `RemoveExisting`
  (`fs/operations/operations.go:1249`) as unresolved interface dispatch, with no candidate target
  supplied. It also leaves the `Move` function value unresolved; at the cleanup `Object.Remove`
  call, `fs.remove` is a possible effect and `backend/local.(*Object).Remove` is one VTA candidate.
  The focused `-site` query on the audit-only `NewObject` call returned no site/path; that is a
  limitation of this graph view, not evidence that there are no callers. Source inspection found
  four direct backend callers: OneDrive, Dropbox, Yandex, and SugarSync.
- Other scoped queries returned one candidate VTA edge from `fs/cache.Get` to `jobs.OnFinish`,
  followed by an established edge from `jobs.OnFinish` to `(*Job).OnFinish`; two candidate edges
  appeared around multithread-copy writer close; and the sync deleter query had no candidate-only
  edges. These remain candidate relationships, not established runtime paths. The local
  `(*Fs).PutStream -> (*Fs).Put -> (*Object).Update` path was established in the selected graph.

Raw inventory, audit records, query outputs, and command result files are under
`/tmp/reffector-bug-discovery/analysis/`.

## Candidate 1 — `RemoveExisting` suppresses lookup failures

**Observed behavior:** reproduced. **Impact:** plausible loss of rollback protection; backend-specific
overwrite consequences remain unverified.

`fs/types.go:31-37` specifies that a missing object is reported as `ErrorObjectNotFound`. The local
backend implements this distinction: `backend/local/local.go:622-630` maps `os.IsNotExist` to
`ErrorObjectNotFound`, maps permission failures to `fs.ErrorPermissionDenied`, and returns other
lookup errors. But `fs/operations/operations.go:1249-1251` returns a no-op cleanup and `nil` for
*any* `NewObject` error. The helper's comment at lines 1240-1247 says it moves an existing object
aside so it can be restored if the operation fails. Existing `TestRemoveExisting`
(`fs/operations/operations_test.go:2348`) covers a not-found lookup and normal existing-object
cleanup, but not a lookup failure other than not-found.

OneDrive and Dropbox call the helper and defer its cleanup before continuing their server-side copy
(`backend/onedrive/onedrive.go:1797-1801`, `backend/dropbox/dropbox.go:1348-1352`); Yandex and
SugarSync have the same call pattern. If a backend lookup fails for a reason other than “not found”,
the helper currently allows the copy to continue without moving the old destination aside. If that
backend's copy can replace the destination and then fail, the deferred no-op cannot restore the old
object. That final backend behavior is a hypothesis, not a tested fact here.

**Reproduction:** `/tmp/reffector-bug-discovery/reproductions/remove-existing-local-permission.go`
creates a temporary local remote, makes a parent directory inaccessible, and calls the real
`local.NewFs` and `operations.RemoveExisting`. The successful retry output is:

```text
lookup_error=permission denied is_permission_denied=true
remove_existing_returned_nil_error=true cleanup_is_nil=false target_absent_after_unlock=true
```

This proves that a real `ErrorPermissionDenied` lookup is reported as success and yields a non-nil
no-op cleanup. It does not modify a destination object or demonstrate data loss. A fake-interface
probe (`/tmp/reffector-bug-discovery/reproductions/remove-existing-lookup-error.go`) independently
showed the same error suppression. The initial real-backend probe attempt panicked because the
temporary harness passed a nil config map to `local.NewFs`; the harness was corrected to pass
`configmap.Simple{}` and rerun successfully. Both attempt logs are preserved.

**Independent Test 2 recommendation:** add a focused test around `RemoveExisting` with a fake `fs.Fs`
whose `NewObject` returns a non-`ErrorObjectNotFound` error. Verify the helper returns that error
(and does not signal “absent” with a no-op cleanup). For impact, separately exercise one
server-side-copy caller with a controlled fake backend that makes destination lookup fail while the
subsequent copy path is observable; verify it does not proceed without the preservation step. Keep
those assertions distinct from any implementation fix.

## Lower-confidence lead — finished job listeners retain closures

`fs/rc/jobs/job.go:74-80` invokes listeners at finish but does not clear the listener slice;
`OnFinish` appends to the slice at lines 94-104 and returns a cancellation function. At
`fs/cache/cache.go:187-195`, `Get` registers an `Unpin(f)` closure and discards that cancellation
function. The focused graph reported `cache.Get -> OnFinish` only as `candidate_vta`, not an
established edge. This suggests a finished job may retain a callback (and captured backend value)
until the job itself expires, but the retention duration, runtime registration path, and material
impact were not measured. It is a lead, not a finding; no reproduction was attempted.

## Deprioritized paths

- `backend/local.(*Fs).DirMove` maps every `os.Lstat` error other than not-exist to
  `fs.ErrorDirExists` (`backend/local/local.go:1184-1186`). The high-level wrapper falls back on
  that sentinel (`fs/operations/operations.go:2503-2512`). This is suspicious error classification,
  but the examined fallback does not establish a destructive or materially different outcome; no
  reproduction was attempted, so it was not promoted.
- Writer-close and sync-deleter concurrency sites were inspected as leads. Reffector reported
  candidate-only close relationships in the former and no candidate-only relationships in the
  latter; this static evidence and source review did not establish a hang or missed cleanup. No
  package test suite was run.

## Novelty screen and limitations

Three unauthenticated, read-only GitHub issue searches were made: exact `RemoveExisting` (0 results),
`"server side copy" permission` (28 broad results), and `ErrorObjectNotFound` (26 results). The
latter surfaced related but different reports, including rclone #10100/#10109 on lookup failure
being mapped to 404 and #9530/#9531 on treating an unconfirmed remote lookup as absence. These are
related failure classes, not evidence that this exact helper path is already fixed or reported.
Search results are not exhaustive, so **global novelty is unverified**. Search responses and query
strings are in `/tmp/reffector-bug-discovery/novelty/`.

No rclone package tests or real service integration tests were run. The only runtime evidence is the
bounded local/fake probe above. No source fix or repository test file, commit, staging, or pull
request was created; the two external Go probes remain under `/tmp`.
