## Caddy

Revision verified: `0e392c89baa50f558bc24440482c5d57fff4499f`. Reffector succeeded; report retained at `/tmp/reffector-caddy.EcnjG1`.

### 1. Reload shutdown performs completion work before requests finish

**Evidence:** `(*App).stop`, `modules/caddyhttp/app.go:723`, launches shutdown goroutines but waits for completion only when `exiting` is true. During reload, it proceeds to stop hooks and `Metrics.shutdown` while requests can still be active. Furthermore, `startedShutdown.Done()` precedes the actual shutdown operation—it acknowledges scheduling, not listener closure.

This contradicts `(*Server).RegisterOnStop`’s documented “shut down completely” contract (`modules/caddyhttp/server.go:1298`). It also stops metrics export before draining requests finish updating metrics.

**Improvement:** Keep reload nonblocking, but move completion hooks and exporter shutdown into the tracked drain-completion operation. Replace the scheduling acknowledgment with an acknowledgment of actual listener shutdown.

**Benefit:** Resource teardown and final metric collection acquire an explicit, verifiable ordering.

**Preserve:** Nonblocking reloads, pending-response delivery, finite grace periods, and unlimited draining when grace is zero. Define what completion means after a shutdown timeout; returning from `Shutdown` does not necessarily mean every handler stopped.

**Evidence boundary:** `app_test.go:34` exercises pending responses across reloads, but does not check hook/exporter ordering. Metrics lifecycle tests (`metrics_test.go:526`) use disabled/no-op export. No tests were run; effects on third-party stop-hook consumers remain unverified.

### 2. Failed listener-type validation loses ownership of an open socket

**Evidence:** `(*App).Start`, `modules/caddyhttp/app.go:571–585`, calls `listenAddr.Listen`, then returns if the result is not a `net.Listener`, without closing it. Registration in `srv.listeners` happens later.

Datagram networks return `net.PacketConn` (`listeners.go:125`); `listenReusable`, `listen_unix.go:160`, opens and registers these sockets. `(*App).Validate` (`app.go:426`) does not reject such a network/protocol combination.

**Consequential effect:** An unsuccessful startup can leave an opened datagram socket and listener-usage accounting behind. `Cleanup` cannot recover an object that was never registered with the server.

**Improvement:** Close the acquired object on type rejection, preserving the primary error. Reject known incompatible built-in network/protocol combinations before binding where practical.

**Benefit:** Failed startup reliably releases acquired resources.

**Preserve:** Plugin-defined networks, valid HTTP/3 configurations, and existing error context. Avoid a restrictive network-name allowlist; plugin return types need runtime validation.

**Evidence boundary:** Listener tests cover explicit QUIC cleanup (`listeners_test.go:716`), not this rejection path. This is source-supported, not runtime-reproduced.

### Limitations and metrics

Reffector reported 360 functions, all uncertain. It marked `App.Start` with direct `net.listen`, while `Stop` and `Cleanup` had empty effect lists despite consequential shutdown operations. Interface calls, goroutines, shared-state mutation, and exporter behavior therefore required source inspection.

Both repositories remain unchanged. No tests or application code were executed; no external services were contacted. Search counts include `rg`/`find` source or guidance searches, including failed searches; report queries are excluded. File counts cover distinct source/test files opened for inspection. Runner telemetry was unavailable.

```json
{
  "repository": "caddy",
  "revision": "0e392c89baa50f558bc24440482c5d57fff4499f",
  "scope": ["./modules/caddyhttp"],
  "configuration": "reffector",
  "start_utc": "2026-10-20T02:59:36Z",
  "end_utc": "2026-10-20T03:00:47Z",
  "elapsed_seconds": 71,
  "reffector_seconds": 1.63,
  "search_commands": 10,
  "source_files_inspected": 9,
  "tool_calls": null,
  "input_tokens": null,
  "output_tokens": null,
  "total_tokens": null,
  "candidates_investigated": 3,
  "findings_reported": 2
}
```

## restic

Revision verified: `dd09b6bd01af220b1d5d95c0d4d7d472e05e05c4`. Reffector succeeded; report retained at `/tmp/reffector-restic.LIww3q`.

### 1. Upgrade recovery unnecessarily deletes a potentially valid config

**Evidence:** `upgradeRepository`, `internal/repository/upgrade_repo.go:33`, respects `HasAtomicReplace` when deciding whether removal is necessary. However, `UpgradeRepo` unconditionally removes the config during recovery (`:91`), then attempts to restore it (`:92`).

If the upgrade write failed before replacing the original, recovery deletes that still-valid original. A second write failure then leaves the repository without its config—even on an atomic-replace backend such as local (`internal/backend/local/local.go:87`).

**Improvement:** Restore through atomic replacement when supported; do not remove first. Keep necessary removal explicitly confined to non-atomic backends, and handle rollback errors separately.

**Benefit:** Recovery no longer creates an avoidable config-loss window.

**Preserve:** Version checks, the original upload error, recovery-error reporting, and the retained backup path when recovery fails.

**Uncertainty/tradeoff:** A failed save may have committed before reporting a later error; do not simply assume failure means “original unchanged.” Recovery also uses the potentially canceled operation context; bounded recovery cancellation needs an explicit policy.

**Tests inspected:** `TestUpgradeRepoV2Failure`, `upgrade_repo_test.go:49`, checks error reporting and backup availability, not preservation of the original config.

### 2. Pack temporary-file ownership is incomplete across merging and failure

**Evidence:** In `(*packerManager).mergePackers`, `internal/repository/packer_manager.go:79`, entries are removed from manager ownership (`:87`), but successfully merged source packers are never closed. `pack.Packer.Merge` (`internal/repository/pack/pack.go:192`) only copies data.

Additional paths lose deterministic cleanup: failed queueing in `Flush` (`packer_manager.go:56`) and early errors in `(*Repository).savePacker` (`:221`). The latter closes its temporary file only after successful backend upload (`:270`).

**Consequential effects:** Open file descriptors and temporary-file storage survive beyond their useful lifetime. Unix unlink-on-open and Windows delete-on-close (`internal/fileio/file_unix.go:11`, `file_windows.go:14`) do not replace explicit closure.

**Improvement:** Establish one owner until successful queue transfer; close merged source packers immediately, close uploader-owned files on every exit, and clean up unqueued packers on abort.

**Benefit:** Resource lifetime becomes deterministic and failure handling easier to verify.

**Preserve:** Pack merging’s information-leak mitigation, randomized distribution, upload backpressure, and primary errors. Avoid double-closing files whose ownership transferred successfully.

**Uncertainty:** Go finalizers may eventually reclaim unreachable files; indefinite leakage was not established. Existing packer tests (`packer_manager_test.go:58`) do not assert file closure.

### 3. Local listing mistakes callback failures for missing directories

**Evidence:** `visitFiles`, `internal/backend/local/local.go:344–349`, returns callback errors unchanged. `(*Local).List` then converts **any** error matching `os.ErrNotExist` into success (`:263–276`), regardless of origin.

A callback returning a wrapped “not found” error therefore stops enumeration, but its caller receives `nil`, falsely indicating successful completion. This violates the backend contract (`internal/backend/backend.go:50`). `(*Repository).List` forwards callback errors directly (`internal/repository/repository.go:962`).

**Improvement:** Handle expected missing directories at the filesystem operation producing that error; propagate callback errors unchanged. Decide separately whether disappearing subdirectories should be skipped or reported.

**Benefit:** Successful listing cannot conceal callback failure or an incomplete traversal.

**Preserve:** Missing optional directories may represent an empty listing; callbacks remain synchronous and run at most once per entry.

**Evidence boundary:** Generic listing tests cover enumeration and cancellation (`internal/backend/test/tests.go:255`, `:349`), but not callback-origin `ErrNotExist`. The contract violation is source-supported; its frequency in normal operation is unknown.

### Limitations and metrics

Reffector reported 233 functions, 231 uncertain. Its direct filesystem effects led to upgrade/local-backend inspection; transitive create/write/remove effects led to packer ownership analysis. Backend interface operations and callback behavior were established from source, not assumed from the effect lists.

No runtime, filesystem-failure, or concurrency validation was performed. Reported risks are bounded by that static evidence. Metric definitions and telemetry limitations are the same as above.

```json
{
  "repository": "restic",
  "revision": "dd09b6bd01af220b1d5d95c0d4d7d472e05e05c4",
  "scope": ["./internal/repository", "./internal/backend/local"],
  "configuration": "reffector",
  "start_utc": "2026-10-20T03:00:47Z",
  "end_utc": "2026-10-20T03:02:10Z",
  "elapsed_seconds": 83,
  "reffector_seconds": 0.75,
  "search_commands": 16,
  "source_files_inspected": 20,
  "tool_calls": null,
  "input_tokens": null,
  "output_tokens": null,
  "total_tokens": null,
  "candidates_investigated": 4,
  "findings_reported": 3
}
```