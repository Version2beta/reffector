# Caddy investigation observability audit

## Executive findings

This is a read-only retrospective on the saved Caddy survey, candidate follow-up, and historical control evidence. It does not repeat discovery or tests. The only new source computation was a package/file inventory and AST function count over the preserved snapshot; the remaining new files derive counts and chronology from saved transcripts and JSON. All derived files are under `/tmp/reffector-caddy-discovery/observability-audit/`; artifact names below refer to that directory unless another path is given.

The evidence supports these conclusions:

- **Source identity is well supported by pinned archive URLs and hashes, but not by Git metadata inside the survey snapshot.** The blind target was Caddy `32bba1545989e07368c0977a37652f33d80c8cd4`; its extracted tree has no `.git`. The archive URL names that commit and its SHA-256 is recorded.
- **The survey reached all 49 Go package roots selected by `go list ./...` on the recorded `darwin/arm64` environment, but this is not comprehensive source coverage.** Reffector's full reverse graph and broad query outputs were truncated; tests were excluded from its analysis; most source files/functions were not manually read.
- **The reported “about 14 active minutes” is an unverified sum of worker estimates, not measured time.** Structured logs show two normal worker runs spanning 2m41s and 6m18s, with 22 and 67 tool-start events. Neither run timed out. The reason the supervisor did not schedule another survey phase is not recorded.
- **The known control was not rediscovered as a defect.** The blind target already included the fix. A phase-2 Reffector output did return the two fixed `Close` sites in `handleUpgradeResponse`, and those lines appear in the worker transcript; the worker did not read `streaming.go` or raise a cleanup hypothesis. No blind worker analyzed the pre-fix revision. The separate pre-fix query was evaluator-targeted and emitted zero matching `resource.release` sites.
- **The ACME result remains a qualified, configuration-dependent database-layer candidate.** Later disposable tests reproduced normalized-ID aliasing, an ACME database account-key conflict, and a database path outside the ACME-server subdirectory but still inside the temporary data home. No HTTP-level consequence or remote attacker path was demonstrated.

No Caddy production source, prior report, or journal was changed by this audit. It did not run Reffector, tests, Æ retrieval, or the canonical packet builder. It adds only this new report to the feature worktree; derived audit files are in `/tmp`.

## Revisions and provenance

| Purpose | Revision and evidence | Use and limitation |
|---|---|---|
| Blind discovery/current target | `32bba1545989e07368c0977a37652f33d80c8cd4`; codeload URL pinned to that revision; archive SHA-256 `1d14b9b0861654a73a002105b0fb9668475e6d93b182b880406e0f8f57c6f239`; extracted archive has 742 files | Used by both blind phases and current-target tests. The source snapshot lacks Git metadata, so the commit is identified by the pinned archive URL and preserved provenance records, not a local `git rev-parse`. The worker copy was checked against the snapshot manifest. |
| Historical pre-fix control | `8e237f195506741d3109ba7792e81d57a5f65036`; archive SHA-256 `601ba36d01cc03b4605fb0a086635d96976943beeb09c2f2df91f20371738a56` | Used only for evaluator queries and regression tests after the blind worker results were returned. Its extracted source archive has 740 files. |
| Fixing merge | `62bc6064335af61914e68ad2197e93e6d37a5efc`; archive SHA-256 `7bc46477f63230b35deaa9b34fe22f9bf4ceac62ebf6a8b0ad3cc2786a9fbd96` | The saved API/merge record says its sole parent is the pre-fix revision. The current target's `streaming.go` and regression test are byte-identical to this snapshot. |
| Reffector | Feature worktree `e0e62a296170881822b08bafd50c1f0b81104ce7`; binary SHA-256 `822b818277e4cf37a134de5b0b0c058c5e70ad612d888a741f096c7b3063c967`; Go `1.27.2 darwin/arm64` | Same binary was used for blind and control queries. Build information marks the build checkout modified; the recorded feature-worktree status has no tracked source changes, but the binary should be identified by its hash as well as commit. |

The original local Caddy checkout at `/Users/rob/Development/caddy` was not the analyzed source: it remained at `0e392c89baa50f558bc24440482c5d57fff4499f`, behind the cached upstream by three commits. The Reffector analysis used the disposable pinned archive, not that checkout.

The pre-fix regression test workspace had one intentional test-only difference: the fixing revision's `streaming_test.go` was copied into a disposable pre-fix source tree so the regression test could run against the old implementation. The pre-fix production `streaming.go` remained unchanged. The source and test hashes and the one-file difference are recorded in `historical-control/evaluator/tree-integrity.txt`.

## Coverage accounting

### Package and source scope

The preserved `go list ./...` root list and Reffector's `coverage.packages` list are identical: **49 of 49 selected package roots (100%)** for this host configuration. This percentage is only root-list agreement under `darwin/arm64`; it does not represent all platforms, build-tag variants, source lines, or Caddy code.

| Inventory | Count | Interpretation |
|---|---:|---|
| Go files in the source tree | 401 | Includes 173 `_test.go` files. |
| Host-selected production Go files | 209 | `go list` build-selected files. |
| Host-selected test Go files | 167 | Tests were not included in the Reffector analysis (`-tests` was false). |
| Go files ignored for this host | 25 | 19 production and 6 test files; platform/build-tag scope is therefore limited. |
| Production `go list -deps` packages in the saved preflight | 1,032 | Separate dependency preflight, not a per-package Reffector load ledger. The corresponding test dependency graph listed 1,121 packages. |

The analyzer reported **2,636 source functions**. A separate AST inventory of the 209 host-selected production files counted 2,637 function units (2,167 declarations and 470 literals). The one-unit mismatch is unresolved; the AST and Reffector may use different identity/merging rules. The ratio is not a function-coverage percentage. There is no LOC denominator, and no defensible percentage of eligible Caddy code analyzed can be calculated.

Across the successful direct `read` calls in the two blind transcripts, **21 distinct production file paths** received at least one source excerpt (22 successful production read events because `caddyauth.go` was read twice), and five distinct test-file paths received excerpts. Against the host-selected file counts, that is 21/209 = **10.05% of production file paths** and 5/167 = **2.99% of test file paths**. These are retrieval counts only: excerpts were partial, and grep/shell output returned snippets from additional files that are not counted. They are not semantic review or test coverage percentages. Exact read paths, ranges, and failures are in `/tmp/reffector-caddy-discovery/observability-audit/phase1-read-events.tsv` and `phase2-read-events.tsv`.

The 21 production files with successful direct reads were:

- Phase 1: `modules/caddyhttp/reverseproxy/healthchecks.go`, `modules/caddyhttp/reverseproxy/caddyfile.go`, `modules/filestorage/filestorage.go`.
- Phase 2: `modules/caddyhttp/caddyauth/caddyauth.go`, `modules/caddyhttp/requestbody/requestbody.go`, `modules/caddyhttp/rewrite/rewrite.go`, `modules/caddyhttp/timeouts/timeouts.go`, `modules/caddyhttp/app.go`, `modules/caddyhttp/server.go`, `modules/caddyhttp/intercept/intercept.go`, `modules/caddypki/adminapi.go`, `modules/caddypki/acmeserver/acmeserver.go`, `modules/caddypki/acmeserver/policy.go`, `modules/caddypki/ca.go`, `modules/caddypki/pki.go`, `modules/caddytls/capools.go`, `modules/caddytls/certmanagers.go`, `modules/caddytls/acmeissuer.go`, `modules/caddytls/ondemand.go`, `modules/logging/filewriter.go`, and `usagepool.go`.

The five test paths excerpted by the blind workers were `modules/caddyhttp/caddyauth/caddyauth_test.go`, `modules/caddyhttp/requestbody/requestbody_test.go`, `modules/caddyhttp/timeouts/timeouts_test.go`, `modules/caddyhttp/app_test.go`, and `modules/caddypki/acmeserver/acmeserver_test.go`. Phase 1 also obtained test-source search snippets, but did not make a successful direct read of the active-health test file. Phase 2 attempted to read `modules/caddyhttp/tracing/tracing.go`, but that path read failed; no successful source read or retry appears in its transcript. Thus the report's tracing source review is not supported by a successful source retrieval event.

### Reffector counts and truncation

The full phase-1 direct inventory reported 2,636 functions (1,911 function summary records emitted; 725 omitted), 2,623 identified operation sites (837 selected-source and 1,786 dependency sites), and 11,802 unresolved sites. The full direct output contained 839 site records: 837 with recognized effects and two with only possible effects. It grouped operations into 40 effect categories. The 574-node graph was bounded at 1,000 edges (394 established and 606 candidate edges); edge and depth truncation were both set, and 692 of the 839 reverse roots reported truncation. The separate unresolved output emitted only the first 40 of 11,802 sites. These counts describe the saved static output, not runtime calls, bugs, or completeness.

Package filters narrowed reported site output; each saved report still records all 49 selected roots. Phase-1 full and package direct queries used `max-sites 2000`; phase 2 used `max-sites 500`. The unresolved sample was capped at 40, and the three phase-2 reverse-site queries used `max-depth 2`/`max-edges 100`. The broad phase-1 graph hit its 1,000-edge bound (the CLI's saved default depth bound was 6). Representative output sizes:

| Phase | Output selection | Sites reported | Material limitation |
|---|---|---:|---|
| 1 | Full direct inventory | 839 | Reverse graph truncated at edge/depth limits. |
| 1 | `modules/caddyhttp/reverseproxy` | 169 | Reverse-depth truncation. |
| 1 | `modules/caddytls` | 104 | Reverse-depth truncation. |
| 1 | `modules/caddyhttp/fileserver`; `modules/caddyhttp/encode` | 6; 9 | Bounded direct-site views. |
| 1 | `modules/caddyconfig/caddyfile`; `modules/filestorage` | 0; 0 | No matching direct sites in these filters, not proof of no behavior. |
| 2 | `modules/caddyhttp` | 328 | Includes many child packages; 97 reverse roots depth-truncated. |
| 2 | `modules/caddypki`; `modules/caddypki/acmeserver` | 25; 2 | ACME query showed `MkdirAll` and `DialContext` sites. |
| 2 | `modules/caddyhttp/caddyauth`; `modules/caddyhttp/requestbody`; `modules/caddyhttp/tracing`; `modules/logging`; `modules/internal/network` | 11; 1; 4; 23; 12 | Direct-site counts only; tracing source retrieval failed as noted above. |
| 2 | `modules/caddyhttp/headers`; `modules/caddyhttp/intercept`; `modules/caddyevents` | 2; 5; 1 | Bounded direct-site views. |
| 2 | `rewrite`; `timeouts`; `map`; `httpcaddyfile`; `filestorage`; `metrics` | 0 | No matching direct sites for the chosen filters. |

Three phase-2 reverse-site queries used `max-depth 2` and `max-edges 100`. One returned no direct graph; two were depth-truncated. The broad CaddyHTTP query and phase-1 full inventory are not exhaustive reverse-call maps. The query-by-query figures and truncation flags are preserved in `/tmp/reffector-caddy-discovery/observability-audit/phase1-query-ledger.tsv` and `phase2-query-ledger.tsv`.

## What the agents actually examined

The worker transcripts show a breadth-first static inventory followed by selected source review, not a function-by-function audit. Phase 1 reviewed core lifecycle/configuration effects, command and filesystem operations, reverse proxy health checks, fileserver/encoding, TLS/certificate management, and a thin filestorage wrapper. It queried reverseproxy, fileserver, encode, TLS, Caddyfile, and filestorage. Phase 2 broadened to auth, request-body handling, rewrites/timeouts/tracing, intercept/headers, PKI/ACME, logging, internal networking, events, and the broader HTTP package. It also ran the existing offline ACME-server package tests, which passed; it did not run the full Caddy test suite.

The substantive hypotheses and their boundaries were:

| Lead | Evidence actually obtained | Outcome |
|---|---|---|
| Templated active-health header key expanding to `Host` | Phase-1 source excerpt from `healthchecks.go:453-459`; Reffector showed `net/http.(*Client).Do` (`site:8d0cfb05b42791f6d16f849d2b90a247b78390c159cb70ba0c527ebbfbcb03dc`, callback uncertainty) and `io.(Closer).Close` (`site:3d14a333882d1ecec2f2ffa4fe1f4bb66e6ada7b7cf525c896326231412fe386`, interface-dispatch uncertainty). Test search snippets did not show a key-expansion regression case. | Low-confidence source hypothesis. No discriminating test or runtime reproduction was run. The workers did not establish that templated header names are a supported contract. |
| Oversized active-health response body / connection reuse | Search output and source review of response size limiting, draining, and close behavior. | Rejected as a defect: the bound appeared deliberate and no harmful consequence was reproduced. |
| ACME CA-ID normalization and database path | Phase 2 read `acmeserver.go`, saw Reffector's `os.MkdirAll` site `site:4949f26ce0586e810576f24dbaa9fb67a14599bc38c7e00e860e631c745a9c34` in `(Handler).openDatabase$1`, and ran a standard-library-only regex/path probe. The worker's probe did not create/open a database. | Candidate retained. The worker did not establish database aliasing or an HTTP consequence. Those were tested later by the supervisor in a disposable source copy. |
| Auth/middleware, request body, timeouts, shutdown, TLS/PKI policy, logging | Selected implementation/test excerpts, direct-site queries, and bounded reverse queries. | No concrete independent failure established. These areas were sampled, not exhaustively audited. |
| Historical upgrade cleanup control | Current-source Reffector sites were returned in phase 2 (details below); no blind worker source read of `streaming.go`, test of the historical failure, or control hypothesis appears. | No independent control finding. Do not treat an effect-site listing as defect discovery. |

After the blind output was returned, the supervisor read `getDatabaseKey`/`openDatabase` in `acmeserver.go`, the `databasePool`/`UsagePool` path, and pinned Smallstep database implementation. The source cleaner lowercases/trims and applies ``[^\w.-_]`` before using the result both as a database-pool key and as a path component. A disposable copy was then used to reproduce the ACME alias and path behavior. The local probes showed `a-b` and `ab` share a database wrapper, and `CreateAccount` for a repeated account key then fails in the ACME database adapter with `key-id to account-id index already exists`. A traversal-like ID created a database outside Caddy's `acme_server` subdirectory but still inside the test's temporary XDG data home. No HTTP-level registration flow, remote attacker path, arbitrary-file overwrite, or privilege escalation was demonstrated. The discovery report and handoff correctly keep this as a qualified, configuration-dependent candidate.

## Worker timing and termination

Structured timestamps are from the worker transcripts and metadata; active time was not instrumented.

| Run | Configured ceiling | Launch-to-final wall span | Tool events | Worker-reported estimate | Termination evidence |
|---|---:|---:|---:|---|---|
| Phase 1 | 5,400,000 ms (90 min) | 161.011 s (2m41s) | 22 starts / 22 ends; 2 tool errors | “about 8 minutes”; rough 16 calls | Exit code 0; final `stopReason=stop`; task explicitly said to stop after the initial survey. |
| Phase 2 | 4,200,000 ms (70 min additional) | 378.296 s (6m18s) | 67 starts / 67 ends; 2 tool errors | 5m43s; rough 29 calls | Exit code 0; final `stopReason=stop`; task requested a separate supplement and allowed up to 70 additional minutes. |

Worker metadata also records phase-1 input/output/cache-read tokens of 101,828/8,172/630,272 and phase-2 totals of 81,300/17,750/5,277,696; these are usage counters, not activity-duration measurements. The worker estimates sum to about 13m43s, but this is not a measured “active” total. Phase 1's estimate is longer than the 2m41s recorded run span; phase 2's estimate is a pair of worker clock reads, not an activity timer. Structured records contain no timeout/cancellation event. The phase-2 `tracing.go` read failure was not followed by a successful source read, but the worker reported no persistent blocker and completed normally.

The parent explicitly resumed phase 1 once with a 70-minute additional ceiling. No further discovery-worker run followed phase 2; the supervisor moved to candidate-specific follow-up and then control evaluation. The logs do not contain a decision record explaining why it considered further breadth unnecessary. “They stopped because they used 14 minutes” and “they stopped because the 90-minute budget expired” are both unsupported. The ceilings were maxima, not guarantees that work would continue until the timeout.

## Positive-control audit

The separate control report at `reports/caddy-upgrade-positive-control.md` accurately reports the PR #8269 `TestUpgradeResponseCleanup` comparison: the cleanup tests failed on the pre-fix source, passed on the fixing source, and passed on the current target. The saved command was `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOWORK=off GOFLAGS=-mod=readonly go test ./modules/caddyhttp/reverseproxy -run '^TestUpgradeResponseCleanup' -count=1 -v`. The tests use local `httptest`/`net.Pipe` behavior; they do not demonstrate production file-descriptor exhaustion under load.

The targeted Reffector command was `reffector -stats -direct -effect resource.release -function handleUpgradeResponse -package modules/caddyhttp/reverseproxy -max-sites 500 -json .`; it filtered on the known function and effect. It returned zero matching direct sites pre-fix, and two `io.(Closer).Close` sites in the fixing and current snapshots (`streaming.go:70` and `:72`, both with `interface_dispatch` uncertainty). The selected operation-site count rose from 835 to 837. This shows the analyzer recognizes cleanup operations present in the fix. It does not show that the analyzer identifies the missing cleanup obligation before the fix. The pre-fix query was evaluator-targeted using the known function name; no broad pre-fix inventory was saved.

One important transcript detail refines the existing report: the phase-2 blind query over `modules/caddyhttp` already returned the two current-target `resource.release` sites in `(*Handler).handleUpgradeResponse$1` at `streaming.go:70` and `:72`, along with the `WriteHeader` and goroutine sites. The site IDs were `b08b35129be93cd8a9be32d92a600c5597e9a5fafad17da5b5909c68971e60a6` and `903ee7267b7e88a21c279614b53036f2b2401e007f2168c0fe52e91f3bd10e04`. Those site lines are present in the worker's phase-2 tool-result transcript and extracted in `/tmp/reffector-caddy-discovery/observability-audit/blind-control-sites.json`. However, the blind read ledger contains no `streaming.go` read, and neither frozen worker report raises an upgrade-cleanup hypothesis. The agent received operation evidence but did not turn it into a defect finding.

That distinction does **not** make this a blind rediscovery: the blind workers only analyzed the already-fixed current target, and no blind worker examined the vulnerable pre-fix revision. Under the original A–F taxonomy, the agent-level historical rediscovery result is best called **F — Inconclusive** (with the observation that no control finding was made), not D: there was no historical defect in the source revision given to the workers. The saved pre-fix query is a zero result for the specific `resource.release`/known-function filter; it is evidence of a gap for that query, but not enough to label all pre-fix Reffector output **E — Coverage gap**, because no broad pre-fix analyzer inventory or all-effects function query was preserved. The existing report's “partial tool-positive” wording is defensible only as a post-fix effect-site contrast, not as evidence the defect was detectable before the fix.

The control chronology also has limits. The supervisor created the separate control identity record before launching the blind worker, as the original task required, and read it only after the discovery report had been read/frozen. The pre-evaluator frozen discovery copy is `/tmp/reffector-caddy-discovery/novel-discovery/caddy-effect-directed-bug-discovery-frozen.md` (SHA-256 `102456a7d2b5312e3a866633ad0d6f793992ff568b759d71e30a44ef1f0b6449`). The phase-2 worker prompts and separate workspace show **worker-level blinding**; the supervisor was not blind to the control, and the same main session—not a distinct evaluator subagent—performed the post-freeze control work. The discovery report received a later edit at 04:17:41 UTC, after the control report was written at 04:12:15 UTC. Inspection of that edit shows revised ACME database-layer consequence wording, not a change to the candidate or the control. Thus the worker outputs and frozen pre-evaluator copy existed before evaluation, but the final discovery synthesis was not byte-frozen before evaluation. Exact timestamps and final-report hashes are in `/tmp/reffector-caddy-discovery/observability-audit/control-chronology.tsv` and `/tmp/reffector-caddy-discovery/novel-discovery/final-report-hashes-after-clarification.sha256`.

## Smallest useful future instrumentation

1. **Record run completion and time explicitly.** Emit run ID, launch/resume, worker start/end, wall duration, any measured active duration and its definition, timeout/cancel state, and a structured completion reason (“phase done,” “budget exhausted,” “blocker,” or “supervisor stopped”). Preserve the supervisor's decision to continue or stop. Do not reconstruct active minutes by adding free-text estimates.
2. **Save a machine-readable analysis/read ledger.** For each Reffector invocation record target-tree digest/revision, binary digest/build info, host/build tags, tests flag, package roots, exact filters/caps, result counts, load errors, and site/edge/depth truncation. Also record source paths/ranges, successful or failed reads, and test commands/exit statuses. This would make file/function denominators and failure boundaries reproducible without treating output size as coverage.
3. **Separate the blind and evaluator boundaries in the record.** Hash/freeze the worker result before the evaluator reads control materials; identify whether the evaluator is a different agent; explicitly state whether blinding applies to workers only or to the supervisor too. Report post-fix operation detection separately from pre-fix defect-detection evidence.

## Evidence and remaining limits

Principal prior reports are `reports/caddy-effect-directed-bug-discovery.md`, `reports/caddy-upgrade-positive-control.md`, and `reports/caddy-discovery-handoffs/acme-ca-database-key-normalization.md`. Saved phase reports and Reffector outputs are under `/tmp/reffector-caddy-discovery/novel-discovery/`; control source, query, and tests are under `/tmp/reffector-caddy-discovery/historical-control/`. Worker transcript paths, input/output artifacts, and derived audit files are indexed in `/tmp/reffector-caddy-discovery/observability-audit/evidence-manifest.tsv`.

This audit does not establish whole-repository function coverage, runtime reachability for static sites, exhaustive review of all candidate paths, or a remote security impact for the ACME candidate. The main unresolved observability gap is the absence of a trustworthy active-time/stop-decision record; the main positive-control limit is that only post-fix code was supplied to the blind workers.
