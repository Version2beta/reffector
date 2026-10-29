# Caddy pre-fix blind-control evaluation

## Result

**Class E — relevant behavior was absent from useful analyzer evidence.** The fresh investigator did not rediscover the defect addressed by Caddy PR #8269. It reported no confirmed defect; none of B1–B4 corresponds to the historical failure. This is one bounded control, not a measure of general Reffector accuracy or a claim that Caddy is defect-free.

## Frozen experiment boundary

The subject was the pre-fix Caddy revision `8e237f195506741d3109ba7792e81d57a5f65036`, not a fixed or current checkout. Its archive SHA-256 is `601ba36d01cc03b4605fb0a086635d96976943beeb09c2f2df91f20371738a56`; all 740 extracted files match the preserved per-file manifest (`e51148764e5b9fea4b61d911f7e828448474030a91003d876004471760f01098`). The original Caddy checkout was not modified.

The pre-observability Reffector binary is SHA-256 `822b818277e4cf37a134de5b0b0c058c5e70ad612d888a741f096c7b3063c967`, built from feature commit `e0e62a296170881822b08bafd50c1f0b81104ce7` (`vcs.modified=true`), Go 1.27.2, Darwin/arm64. Tests were excluded from analysis. The broad run selected 49 package roots and inventoried 2,629 source functions; it emitted 837 sites and 1,000 graph edges (393 established, 607 candidates), with edge/depth truncation but not site truncation. The unresolved query identified 11,782 sites but emitted only 40, with site/depth truncation; none of those 40 locations was local Caddy source. The CLI did not separately report per-package load outcomes or a complete source-code coverage denominator. Exact outputs and configuration are preserved under `/tmp/reffector-caddy-discovery/pre-fix-blind-control/analysis/`.

A new investigator session received the generic assignment, pre-fix snapshot, and analyzer output. Its tool list contained no Æ tools; its complete transcript shows no read or search outside the dedicated workspace. Bash access was not OS-sandboxed, so this is an audited session/access boundary, not kernel-enforced filesystem isolation; no contamination was observed.

The blind results were frozen before opening the historical evaluator material. The freeze record is `/tmp/reffector-caddy-discovery/pre-fix-blind-control/freeze-record.json`; its 792-file manifest SHA-256 is `ca7f54fecdbcb3c6880adfea9c78174d678a0c7fd43ae3be6d899a40c7cd2904`. Frozen final-report SHA-256: `2aca1904d7f6fc1bcaaea115cf1cb3f77dad1354f58c6c8b08b82defb99425ed`. Frozen candidate-ledger SHA-256: `8a3c839b52410c100c0fb83304129091975bd5eb7ff75f4371d0a5778881e197`. The manifest verifies all 792 files. Evaluator-only material was added later under the separate `evaluation/` directory and is not part of that blind manifest.

## Investigation result and coverage

The investigator reviewed several independent areas: HTTP/3/WebTransport application shutdown, the WebTransport reverse-proxy pump, Caddyfile overwrite persistence, TLS cleanup, and UsagePool. Its final ledger contains B1–B4. B1 and B3 received bounded follow-up; B2 remained explicitly untested; B4 received only a brief breadth review. It directly retrieved 16 source excerpts from 11 files (8 production files and 3 test files), made 7 source searches, and did not directly read `modules/caddyhttp/reverseproxy/streaming.go` or its tests. It made one successful narrow Reffector query for `(*App).stop`, not for the historical function. No tests or probes were run by the investigator.

The run was launched at `2026-10-29T18:00:42.268Z`; the worker's first prompt was at `18:00:42.951Z`. The supervisor sent STOP at `18:05:55.139Z`, the worker received it at `18:05:55.163Z`, and the runner completed normally with exit code 0 at `18:06:23.997Z`. Launch-to-completion wall time was 341.729 seconds; active investigation time was not measured. The configured 20-minute timeout did not fire. The worker's targeted note says it stopped at the deadline, which conflicts with runner timestamps; the raw note is preserved unchanged and the measured timeline is recorded separately.

## Historical defect and regression evidence

Saved PR #8269 metadata identifies merge commit `62bc6064335af61914e68ad2197e93e6d37a5efc`, whose parent and PR base are the pre-fix revision above. In pre-fix `modules/caddyhttp/reverseproxy/streaming.go`, `(*Handler).handleUpgradeResponse` starts at line 62. It can return on invalid or mismatched upgrade headers (lines 68–81), a non-writable response body (84–87), unsupported/failed hijacking (157–170), a failed handshake flush (173–179), or a failed buffered backend write (184–191). These setup-failure paths do not close `res.Body`; after successful `Hijack`, they also do not close the raw client connection. `net/http` no longer owns automatic socket cleanup after hijacking.

The merged fix adds a deferred guard at lines 67–75 that closes the response body and any hijacked connection unless ownership transfers. It sets `tunnelOwnsConnections = true` at line 234, immediately before handing the connections to `handleUpgradeTunnel`.

The saved regression comparison (not run by the blind investigator and not rerun for this evaluation) used `go test ./modules/caddyhttp/reverseproxy -run ^TestUpgradeResponseCleanup -count=1 -v` with module downloads disabled and read-only module mode. The regression test was injected into separate evaluator test copies; it was not present in or added to the frozen pre-fix source tree. Against the pre-fix implementation, `TestUpgradeResponseCleanupOnFailure` failed because the body remained open; both post-hijack failure cases observed read timeouts instead of EOF from the client and backend peers. The test passed against the fixed source. Full logs, source identities, and the PR patch are in `evaluation/`.

| A6 question | Blind result |
|---|---|
| Identified the affected behavior? | No. |
| Located the affected source? | No. |
| Recognized the ownership obligation? | No. |
| Described a feasible historical trigger? | No. |
| Proposed or demonstrated a discriminating test? | No. |
| Relied materially on Reffector evidence for this defect? | No. Reffector guided other, unrelated leads. |

## Why class E

The frozen full-inventory JSON contains three generic sites in `handleUpgradeResponse`: two `ResponseWriter.WriteHeader` sites (lines 119 and 149) and one goroutine site (line 238). It contains **no emitted Hijack site and no Close/resource-release site in that setup function**. Release sites occur later in `handleUpgradeTunnel` and tunnel-tracker teardown, after successful setup/ownership transfer. The unresolved sample did not surface local Caddy sites. The investigator's recorded top-70 effect summary did not mention `handleUpgradeResponse`, and no candidate, source read, or targeted query addressed it.

Thus the relevant evidence for the failure—resource acquisition through hijack and missing cleanup on pre-transfer returns—was absent from useful analyzer output. The raw file does contain generic sites in the function and nearby cleanup sites; those breadcrumbs are not evidence of the violated failure-path obligation. If any same-function breadcrumb were considered “relevant,” class D could be argued, but that would overstate the actual ownership evidence. I classify this as E, not a successful generic leak hypothesis or source-only rediscovery. The detailed comparison is preserved in `evaluation/analyzer-relevance.json` and `evaluation/investigator-criteria-assessment.json`.

## Limits and preservation

This investigator's negative result is bounded by its 341.729-second wall interval, 11 directly read source files, truncated analyzer graph, and lack of tests/probes. It does not establish the absence of other defects. The supervisor has not changed the frozen investigator artifacts, the original Caddy checkout, Æ, or either Reffector checkout's existing files. No files were staged or committed. This report was added only to the existing discovery-pivot checkout.
