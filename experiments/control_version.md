## Caddy

**Three findings.** Revision verified as `0e392c89baa50f558bc24440482c5d57fff4499f`; working tree remained clean.

### 1. Health thresholds do not count consecutive results

**Evidence:** `Handler.doActiveHealthCheck`, [`healthchecks.go:391`](caddy/modules/caddyhttp/reverseproxy/healthchecks.go#L391), defines `markUnhealthy` and `markHealthy` at lines 464–506. Each increments its own counter, but resets counters **only when health changes**. `Upstream.countHealthPass`, `countHealthFail`, and `resetHealth`, [`hosts.go:233–255`](caddy/modules/caddyhttp/reverseproxy/hosts.go#L233), confirm that opposite results do not reset the streak.

This contradicts the documented **consecutive** thresholds at `healthchecks.go:111–117`. With `Fails=2`, a healthy upstream receiving **failure → success → failure** becomes unhealthy.

- **Consequential effect:** Probe responses change backend eligibility for request routing and emit health-transition events.
- **Improvement:** Give each upstream one coherent health-result transition that resets the opposite streak and changes health only at the configured threshold. Address overlapping probes: `doActiveHealthCheckForAllHosts` launches new goroutines without waiting for preceding probes.
- **Benefit:** Routing changes would follow the documented policy, with transitions directly testable using result sequences.
- **Preserve:** Per-handler health isolation, default thresholds, passive-health behavior, and events only for actual transitions.
- **Tests and uncertainty:** [`active_health_test.go:105`](caddy/modules/caddyhttp/reverseproxy/active_health_test.go#L105) checks isolation between handlers, not alternating results. The sequential counter defect follows directly from source; concurrent probe ordering needs an explicit policy. Serializing or skipping overlapping probes changes scheduling and requires care.

### 2. Failed upgrade setup leaves connections without a cleanup owner

**Evidence:** `Handler.finalizeResponse`, [`reverseproxy.go:1413–1415`](caddy/modules/caddyhttp/reverseproxy/reverseproxy.go#L1413), delegates HTTP 101 responses to `handleUpgradeResponse` and returns without closing their bodies.

`Handler.handleUpgradeResponse`, [`streaming.go:62`](caddy/modules/caddyhttp/reverseproxy/streaming.go#L62), returns on protocol validation errors, unsupported/failed hijacking, flush failure, or failure forwarding buffered bytes. There is no failure cleanup before ownership reaches the tunnel at lines 207–208.

In particular, after successful hijacking, failures at lines 173–190 leave the client connection outside both normal HTTP-server ownership and tunnel tracking.

- **Consequential effect:** Upstream connections and hijacked client connections are acquired but not explicitly closed on setup failure.
- **Improvement:** Retain cleanup responsibility throughout setup, transferring ownership explicitly only when the tunnel takes over. Before hijacking, close the upstream body on failure; afterwards, close both acquired endpoints.
- **Benefit:** Every setup exit would have an obvious resource owner.
- **Preserve:** Successful attached and detached tunnels must remain open for their intended lifetime. An unconditional deferred close would break detached tunnels.
- **Tests and uncertainty:** [`hopheaders_test.go:17–110`](caddy/modules/caddyhttp/reverseproxy/hopheaders_test.go#L17) exercises unsupported hijacking with a fake whose `Close` does nothing; it does not assert cleanup. Add close-recording failure-path tests. Connection-retention duration was not measured, and custom transports may behave differently.

### 3. Stream cleanup performs potentially blocking writes while holding its registry lock

**Evidence:** `tunnelTracker.closeAttachedConnections`, [`streaming.go:585–604`](caddy/modules/caddyhttp/reverseproxy/streaming.go#L585), and `closeConnectionsForUpstream`, [`streaming.go:756–777`](caddy/modules/caddyhttp/reverseproxy/streaming.go#L756), hold `ts.mu` while calling `gracefulClose` and then `Close`.

For WebSockets, `gracefulClose` reaches `writeCloseControl`, [`streaming.go:647–694`](caddy/modules/caddyhttp/reverseproxy/streaming.go#L647), which performs a synchronous write without establishing a deadline. Backend write timeout defaults to none: [`httptransport.go:128–129`](caddy/modules/caddyhttp/reverseproxy/httptransport.go#L128).

- **Consequential effect:** Configuration cleanup writes close frames and closes live network connections.
- **Problem:** A blocked close-frame write prevents the subsequent forced close, delays other connections’ cleanup, and blocks registration/deletion on the same tracker. Immediate `Handler.Cleanup` can consequently stall.
- **Improvement:** Select connections under the lock, then perform closure outside it. Bound graceful closure with transport-appropriate deadlines or a force-close fallback; avoid introducing abandoned writer goroutines.
- **Benefit:** Registry synchronization would no longer depend on peer responsiveness.
- **Preserve:** Attached-stream close delays, survival of detached streams whose upstream remains configured, removal-triggered closure, and useful error reporting.
- **Tests and uncertainty:** Existing cleanup tests use immediately completing connections; [`streaming_test.go:604`](caddy/modules/caddyhttp/reverseproxy/streaming_test.go#L604) tests cross-tracker deletion rather than stalled same-tracker I/O. Actual blocking depends on buffering and configured deadlines. Closing outside the lock also requires safe handling of concurrent natural tunnel completion.

**Limitations:** Static inspection concentrated on reverse-proxy health and connection ownership, not all of `modules/caddyhttp`. Four candidate hypotheses were investigated; probe overlap was incorporated into finding 1 rather than reported separately. No tests or application code were executed.

```json
{
  "repository": "caddy",
  "revision": "0e392c89baa50f558bc24440482c5d57fff4499f",
  "scope": ["./modules/caddyhttp"],
  "configuration": "control",
  "start_utc": "2026-10-20T02:47:37Z",
  "end_utc": "2026-10-20T02:48:43Z",
  "elapsed_seconds": 66,
  "search_commands": 8,
  "source_files_inspected": 10,
  "tool_calls": null,
  "input_tokens": null,
  "output_tokens": null,
  "total_tokens": null,
  "candidates_investigated": 4,
  "findings_reported": 3
}
```

## restic

**Two findings.** Revision verified as `dd09b6bd01af220b1d5d95c0d4d7d472e05e05c4`; working tree remained clean.

### 1. A stalled refresh can also stall lock-expiry enforcement

**Evidence:** `locker.refreshLocks`, [`lock.go:124`](restic/internal/repository/lock.go#L124), calls `lock.refresh(context.TODO())` at line 173. This disconnects backend refresh operations from cancellation of the protected operation.

When refreshability expires, `locker.monitorLockRefresh`, [`lock.go:188–247`](restic/internal/repository/lock.go#L188), attempts to send on the unbuffered `forceRefresh` channel at lines 233–236. A refresher blocked in backend I/O cannot receive it. The monitor then waits there instead of continuing expiry enforcement. Meanwhile, `unlocker.Unlock`, [`lock.go:22–25`](restic/internal/repository/lock.go#L22), waits for both goroutines.

- **Consequential effect:** Lock-file creation/removal governs whether repository operations may safely continue.
- **Problem:** A sufficiently stalled backend can prevent automatic cancellation of protected work even as the stored lock ages. Explicit unlock can also wait indefinitely. This is a conditional failure path, not evidence of observed repository corruption.
- **Improvement:** Give refresh operations a bounded, cancellation-aware lifecycle, and keep expiry enforcement independent of the refresher accepting a message. Coordinate completion and cleanup so cancellation cannot race replacement-lock adoption.
- **Benefit:** The mechanism responsible for stopping unsafe continuation would remain responsive when refresh itself fails to make progress.
- **Preserve:** Successful slow-refresh recovery, stale-lock existence checks, backend freezing during stale revalidation, and bounded cleanup grace in [`lock_file.go:233–243`](restic/internal/repository/lock_file.go#L233).
- **Tests and uncertainty:** [`lock_test.go:100`](restic/internal/repository/lock_test.go#L100) tests returned refresh errors; line 189 tests finite delays. Neither covers a refresh awaiting cancellation. Backend-specific timeouts may limit exposure. Context cancellation alone cannot interrupt an uninterruptible filesystem syscall.

### 2. Temporary pack-file ownership is incomplete across merging and failures

**Evidence:** `packerManager.newPacker`, [`packer_manager.go:202`](restic/internal/repository/packer_manager.go#L202), acquires an open temporary file. Several paths lose explicit cleanup responsibility:

- `mergePackers`, lines 79–113, copies a source pack into another and drops its manager slot without closing its file.
- `SaveBlob`, lines 137–145, forgets a full pack before queue submission; rejected submission does not close it.
- `Repository.savePacker`, lines 221–270, returns from finalization, flushing, hashing, or backend-save failures before reaching the sole explicit close.

`pack.Packer.Merge`, [`pack/pack.go:192`](restic/internal/repository/pack/pack.go#L192), only copies data; it does not assume ownership of the source file.

- **Consequential effect:** Packing creates open files holding encrypted backup data and temporary storage.
- **Problem:** Resource release depends on garbage collection or process exit rather than operation completion. Some error paths also retain packers through repository fields.
- **Improvement:** Define explicit pack ownership: the manager owns a pack until successful queue handoff; the uploader then owns closure. Close merged-away sources immediately, clean up rejected handoffs, and abort remaining manager-owned packs after workers stop.
- **Benefit:** Descriptor and temporary-space release would become deterministic and testable across success, cancellation, and errors.
- **Preserve:** Random blob distribution and small-pack merging, which mitigate information leakage; successful upload before index publication; no closure while an uploader is still reading. Preserve primary errors when cleanup also fails.
- **Tests and uncertainty:** [`packer_manager_test.go:57–102`](restic/internal/repository/packer_manager_test.go#L57) checks packing outcomes without asserting file closure. Add ownership tests for merging, rejected submission, and backend failure. Unix files are already unlinked and Windows files use delete-on-close—[`file_unix.go:11`](restic/internal/fileio/file_unix.go#L11), [`file_windows.go:14`](restic/internal/fileio/file_windows.go#L14)—so this is nondeterministic resource retention, not necessarily permanently orphaned named files.

**Limitations:** Three candidate hypotheses were investigated. Local-backend cancellation was not reported separately because retry and connection-limit wrappers explicitly enforce pre-call cancellation checks. Local durability code and prune ordering received limited inspection; exhaustive crash-consistency, platform, and concurrency analysis remains undone. No tests, application code, or external-service commands were run.

**Metric conventions:** Search counts include each executed `rg` invocation, including file discovery and unsuccessful searches. File counts include distinct Go files opened for inspection, including tests and followed dependencies. Per-repository runner telemetry and full-session timing were unavailable. Investigation durations exclude final-report composition.

```json
{
  "repository": "restic",
  "revision": "dd09b6bd01af220b1d5d95c0d4d7d472e05e05c4",
  "scope": ["./internal/repository", "./internal/backend/local"],
  "configuration": "control",
  "start_utc": "2026-10-20T02:48:48Z",
  "end_utc": "2026-10-20T02:50:08Z",
  "elapsed_seconds": 80,
  "search_commands": 13,
  "source_files_inspected": 22,
  "tool_calls": null,
  "input_tokens": null,
  "output_tokens": null,
  "total_tokens": null,
  "candidates_investigated": 3,
  "findings_reported": 2
}
```