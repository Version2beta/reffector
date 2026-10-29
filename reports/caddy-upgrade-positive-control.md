# Caddy historical positive-control evaluation

**Scope:** Evaluator-only validation of Caddy PR #8269, performed after the blind Caddy discovery report was frozen. This is not a novel finding and is excluded from the discovery candidate inventory and Æ handoff.

## Control identity

- PR [#8269](https://github.com/caddyserver/caddy/pull/8269): `reverseproxy: close backend body and hijacked socket on upgrade setup failures`.
- Merge commit: `62bc6064335af61914e68ad2197e93e6d37a5efc`; its sole parent is `8e237f195506741d3109ba7792e81d57a5f65036`, verified as the PR base in the pre-survey identity record and GitHub API metadata.
- Pre-fix and fixing source archives are preserved under `/tmp/reffector-caddy-discovery/historical-control/`; archive SHA-256 values are recorded in `evaluator/tree-integrity.txt`. Each extracted tree contains 740 files. The current discovery target's `streaming.go` and `streaming_test.go` are byte-identical to the fixing snapshot.

## Defect and regression evidence

Before the fix, `(*Handler).handleUpgradeResponse` in `modules/caddyhttp/reverseproxy/streaming.go` returned early on failed protocol checks, unsupported/failed hijack, handshake flush failure, or buffered-backend write failure without consistently closing the backend response body and any hijacked client connection. After `Hijack`, net/http no longer owns client-socket cleanup. The PR adds a deferred cleanup guard and transfers ownership only when the tunnel starts.

The PR's regression tests were used unchanged. To run them against the pre-fix implementation, a disposable copy of the pre-fix tree received only the fixing revision's `streaming_test.go`; the original test file was preserved. The fixed test workspace remained byte-identical to its prepared snapshot.

- **Pre-fix:** `go test ./modules/caddyhttp/reverseproxy -run '^TestUpgradeResponseCleanup' -count=1 -v` failed. The body remained open after a protocol mismatch; in both injected post-hijack failures, the client and backend peers timed out instead of observing EOF.
- **Fixing revision:** the same command passed all cleanup tests.
- **Current target `32bba1545989e07368c0977a37652f33d80c8cd4`:** the same command passed. Its `streaming.go` and regression-test hashes match the fixing snapshot.

These tests use `httptest` and `net.Pipe`; they demonstrate missing close behavior in the simulated connection setup, not production file-descriptor exhaustion under load. The PR describes the operational consequence as leaked sockets and possible socket exhaustion.

Commands, outputs, exit statuses, hashes, and the one-file pre-fix test-workspace diff are under `/tmp/reffector-caddy-discovery/historical-control/evaluator/` (`test-pre-fix.*`, `test-fixed.*`, `test-current.*`, `tree-integrity.txt`, and the source manifests).

## Reffector result and what it establishes

Using the same Reffector binary as the blind survey (SHA-256 `822b818277e4cf37a134de5b0b0c058c5e70ad612d888a741f096c7b3063c967`), the bounded query was:

```text
reffector -stats -direct -effect resource.release \
  -function handleUpgradeResponse \
  -package modules/caddyhttp/reverseproxy -max-sites 500 -json .
```

- On the **pre-fix** revision, it emitted **0** matching direct `resource.release` sites for the selected function.
- On the **fixing** revision and the **current target**, it emitted **2** `io.(Closer).Close` sites in `(*Handler).handleUpgradeResponse$1`: `site:b08b35129be93cd8a9be32d92a600c5597e9a5fafad17da5b5909c68971e60a6` at `streaming.go:70`, and `site:903ee7267b7e88a21c279614b53036f2b2401e007f2168c0fe52e91f3bd10e04` at `streaming.go:72`. Both are recognized as `interface_summary` with `interface_dispatch` uncertainty. The selected direct-site count rises from 835 to 837 across these snapshots.

This is evidence that Reffector exposes the cleanup operations added by the fix and distinguishes their absence in the pre-fix function. It does **not** establish the early-return obligation, the guard's control flow, or runtime socket leakage. Those conclusions came from source review and the pre-fix/fix regression test.

**Positive-control verdict:** The cleanup defect is confirmed by the isolated regression comparison, and Reffector surfaces the fixed operations. The blind survey itself did **not** identify this control as a candidate: although it inventoried reverse-proxy effects, its source follow-up concentrated on active health checks and did not inspect upgrade setup/streaming cleanup. Therefore this run is not evidence that the blind survey independently discovered the known defect. It is a partial tool-positive result (effect-site contrast after the fix), not a blind-discovery success.

## Evidence boundary

The blind candidate inventory and novel discovery report were frozen before this evaluator read the control identity or PR materials. Control data remains isolated under `/tmp/reffector-caddy-discovery/historical-control/` and is not included in the novel-finding handoff. No original or authoritative Caddy source/test files were changed; only the disposable pre-fix evaluator copy received the PR regression test file.
