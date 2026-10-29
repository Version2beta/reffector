# Reffector loop handoff — iteration 34

## Completed
- **Milestone 3: complete (bounded current-repo audit).** Decision and scope limits: `reports/milestone3-iteration17-audit-decision.md`.
- **Milestone 4: complete (bounded adversarial tests; no change warranted).** Candidate provenance non-promotion and CFG structural uncertainty assertions: `tools/effect_investigation/test_investigate.py`.
- **Milestone 5: complete (bounded single-packet integration).** Iterations 19–24 built the fail-closed isolated adapter, confined digest-bound retrieval, and durable artifacts; iteration 30 accepted one real structured result. Its artifacts and usage are documented below.
- Validation: `PYTHONPATH=tools/effect_investigation python3 -m unittest test_model_investigation test_investigate` — 19 tests passed (0.059s). `pi --help` confirms CLI flags exist, not model readiness.

## Iteration 22
- Added `prepare_model_packet` to rebuild packet excerpts from digest-verified, packet-named source refs; stale/cached excerpt text is discarded. Retrieval count is capped at 32 and unsafe refs fail closed. Regression covers fresh source content and traversal rejection.
- Validation: `PYTHONPATH=tools/effect_investigation python3 -m unittest test_model_investigation test_investigate` — 20 tests passed (0.082s).
- Verified installed model and auth readiness: `pi --list-models openai-codex` listed `gpt-6-luna`; `pi auth check --provider openai-codex --model gpt-6-luna --json --no-refresh` returned ready/oauth. No model call; no usage metadata.

## Iteration 23
- Added a subprocess adapter in `tools/effect_investigation/model_investigation.py` pinned to `openai-codex` / `gpt-6-luna`, low reasoning, a fresh `--no-session` invocation, and disabled tools/MCP/extensions/context discovery. It caps request/event/answer sizes and timeout, extracts final JSON-mode assistant text, and rejects absent or mismatched provider/model metadata. `invoke_isolated` continues to enforce the packet-bound result schema.
- Added fake-runner command and metadata tests in `tools/effect_investigation/test_model_investigation.py`. Validation: `PYTHONPATH=tools/effect_investigation python3 -m unittest test_model_investigation test_investigate` — 21 tests passed.
- Provider checks: `pi --list-models openai-codex` lists `gpt-6-luna`; auth check returned ready/oauth. No real call; no observed response/usage. No packet CLI or durable request/result artifacts yet. M5 remains pending.

## Iteration 24
- Added durable single-packet invocation persistence to `tools/effect_investigation/model_investigation.py`: canonical request, success result or failure artifact, request-digest manifest, and provider JSONL event capture when using the pinned provider adapter. Deterministic tests cover success, raw event retention, identity, path redaction, and failure persistence.
- Validation: `PYTHONPATH=tools/effect_investigation python3 -m unittest test_model_investigation test_investigate` — 23 tests passed (0.063s).
- No additional model call; no returned provider metadata or usage observed. M5 remains pending: invocation persistence is tested, but no prepared candidate packet has been sent through the real adapter, and no actual isolated investigation exists.

## Iteration 25 — real isolated invocation failed closed
- Built current-repo EffectReport and packet inventory: 59 sites, 99 nodes, 238 edges; 57 candidates emitted, 0 deferred. `go build -o /tmp/reffector-iter25 ./cmd/reffector`, `/tmp/reffector-iter25 -json -all -max-depth 100 .`, and `python3 tools/effect_investigation/investigate.py /tmp/reffector-iter25-effects.json --out /tmp/reffector-iter25-packets --max-candidates 100 --max-packet-bytes 48000` (source root omitted because no manifest was supplied).
- One source-grounded Reffector packet (`os.RemoveAll` cleanup closure, digest-verified excerpt) went through one fresh `openai-codex` / `gpt-6-luna`, low-reasoning, no-session call. Raw invocation artifacts: `reports/m5-iteration25-real-investigation/`. Returned metadata verified; raw event reports 899 input + 213 output = 1,112 tokens and $0.0001964 cost. The assistant output was JSON but had a schema incompatible with the strict contract. `run_and_persist` correctly rejected it and wrote `error.json`; no accepted `result.json`, no defect claim accepted. One real call; no retry.
- Provider readiness was verified before call (`pi --list-models openai-codex`; auth check ready/oauth). M5 remains pending: align the response contract minimally and test, then another bounded call is needed. M6–M8 remain pending.

## Iteration 28 — adapter failure diagnostics
- Inspected iteration-27 raw events: the complete final assistant message is schema-shaped and metadata-valid; events terminate through `turn_end`, `agent_end`, and `agent_settled`. The stored trace does not include subprocess return code or stderr, so the exact historical exit status is unrecoverable from these artifacts.
- Updated `tools/effect_investigation/model_investigation.py` to include a nonzero CLI return code in the error and preserve the cause text through the isolated-boundary wrapper. Regression tests simulate a complete valid assistant event with return code 7, ensure it remains rejected, and verify the diagnostic/raw events are persisted.
- Validation: `PYTHONPATH=tools/effect_investigation python3 -m unittest test_model_investigation test_investigate` — 24 tests passed (0.071s). No actual invocation/call this iteration.
- M5 remains pending: diagnostics are testable, but iteration 27 remains unsuccessful and there is still no accepted `result.json`. Iterations 25 and 27 are the only additional real calls; reported usage/cost remain 1,112 tokens/$0.0001964 and 1,261 tokens/$0.0002281 respectively. No M6–M8 completion.

## Iteration 31 — M6 source/control-flow challenge
- Independently checked the accepted iteration-30 `os.RemoveAll` hypothesis against source beyond the model packet excerpt. `AnalyzeReport` (`internal/analyzer/analyzer.go:139-143`) immediately defers the cleanup returned by `loadConfig`; thus lack of cleanup on later ordinary returns is contradicted statically. `loading.go:78-93` invokes it on shown post-temp creation errors and returns it on success. Ignored `RemoveAll` errors leave runtime success unresolved, not a demonstrated defect.
- Evidence/reproduction: `reports/m6-iteration31-independent-source-challenge.md`; packet source digest matched current `loading.go` (`10a6d596e03185eba584ffa3e1156973c641e38addea3e0e5584048ba187c286`), no source diff. `PYTHONPATH=tools/effect_investigation python3 -m unittest test_model_investigation test_investigate` — 24 passed.
- M6 remains pending: current negative fixtures reject invented evidence IDs/ID mismatches and packet projection retains candidate-edge status, but explicit semantic rejection of candidate-only-edge promotion by an independent verifier is still required. No code changed, no model call or new usage. Four real attempts cumulative, only iteration 30 accepted.

## Iteration 32 — M6 bounded completion
- Added `verify_static_claims` to `tools/effect_investigation/model_investigation.py`, validating structured relationship resolution against canonical packet evidence and source path/API claims against packet-bound source and operation metadata. It expressly does not decide free-form hypothesis truth.
- Tests accept packet-consistent structured claims and reject candidate-VTA promotion to established, invented relationship IDs, fabricated paths, and API mismatch: `tools/effect_investigation/test_model_investigation.py`.
- Combined with the independent caller-source challenge and claim classifications in `reports/m6-iteration31-independent-source-challenge.md`, M6 is complete for this bounded result. The real result is ID/citation-bound and makes no candidate-edge promotion; all-path source cleanup concern is contradicted statically, while actual `RemoveAll` success remains unresolved without runtime evidence.
- Validation: `PYTHONPATH=tools/effect_investigation python3 -m unittest test_model_investigation test_investigate` — 25 tests passed (0.048s). No model call/new usage; cumulative four attempts, one accepted.

## Iteration 33 — M7 bounded aggregation
- Added deterministic `aggregate_findings` in `tools/effect_investigation/model_investigation.py`: grouping requires exact canonical effect/site, or exact effect plus shared supported caller segment, resource identity, and repeated obligation across sites. It retains original IDs, statuses, evidence IDs, basis, and provisional qualification.
- Tests demonstrate one valid shared-obligation group and reject a false group despite shared effect/category similarity when caller/resource identities differ. The one accepted real finding yields no aggregate group; normalized input and traceable result are in `reports/m7-iteration33-aggregation.json`. Missing resource/repeated-obligation support is represented as unknown, not inferred.
- Validation: `PYTHONPATH=tools/effect_investigation python3 -m unittest test_model_investigation test_investigate` — 27 tests passed (0.068s). No real model call or new usage. M7 complete within this bounded scope.

## Incomplete / limitations
- Runtime confirmation and general-purpose semantic verification are not claimed; the pair is one bounded task and not a general efficacy result.
- M3 scope limits remain documented in its audit decision: this was a current-repo inventory with no candidate count/byte deferrals or rejected packets, purposive overlapping source reviews, unavailable source-manifest excerpts and no matching pinned rclone checkout. No general detection/false-positive claims.
- Iteration 21 changed only `tools/effect_investigation/model_investigation.py` and `tools/effect_investigation/test_model_investigation.py`; pre-existing dirty/untracked runner and supervisor artifacts were preserved.

## Iteration 34 — M8 bounded paired comparison passed
- Preregistered a source-pinned cleanup-control task, truth criteria, rubric, identical question/source lines, limits and fresh-session order before either arm: `reports/m8-iteration34-pair/preregistration.md`.
- Ran source-only then Reffector-supported sequential fresh calls, both verified `openai-codex` / `gpt-6-luna`, low reasoning, exit 0. Raw events, exact requests, answers and manifests are under `reports/m8-iteration34-pair/`; independent source review and rubric scores are in `results.md`.
- Both scored 9/10 with no treatment advantage. Both missed `loading.go:94` returning the cleanup callback because the matched source excerpt ended at line 93; both correctly found the caller defer and ignored `RemoveAll` error, and neither asserted a defect. This shared miss is retained as negative evidence, not repaired in the scoring.
- Reported usage: source-only 1,118 tokens / $0.0003074; treatment 2,152 tokens / $0.0004008; combined CLI-reported $0.0007082. Not independently billed. 1 call per arm, no retries. M8 complete only for this qualified pair; no causal or general usefulness claim.

## Iteration 26 — explicit output schema
- Diagnosed iteration 25's strict-schema failure: the system prompt referred vaguely to a required schema but did not enumerate its keys or constraints. The actual response used a different JSON shape and was correctly rejected; iteration-25 evidence remains unchanged.
- Updated `tools/effect_investigation/model_investigation.py` to state the exact eight required keys, ID binding, null hypothesis option, evidence-ID citation rule, array constraints, and no-markdown requirement. Updated `tools/effect_investigation/test_model_investigation.py` to assert this contract is present in the invocation prompt.
- Validation: `PYTHONPATH=tools/effect_investigation python3 -m unittest test_model_investigation test_investigate` — 23 tests passed (0.049s). No real call this iteration; previous real call remains one failed schema response with 1,112 reported tokens and $0.0001964 per its raw CLI event. No new usage observed.

## Iteration 27 — schema-conforming answer, adapter failure
- One additional sequential fresh `openai-codex` / `gpt-6-luna` low-reasoning call was made after model-list/auth readiness checks and the 23-test suite passed. The packet excerpt was rebuilt from current `internal/analyzer/loading.go` with digest verification.
- Raw events in `reports/m5-iteration27-real-investigation/provider-events.jsonl` verify provider/model and contain a complete answer matching the eight-key schema, candidate/site IDs, and allowed citation ID. Reported usage: 1,006 input + 255 output = 1,261 tokens; CLI-reported cost $0.0002281. However, the adapter invocation was rejected and persisted `error.json` rather than `result.json`; raw events do not identify the subprocess exit code. Treat as an unsuccessful integration call, not an accepted result. No retry.
- M5 remains pending; next action is diagnose/record the subprocess completion status and fake-test a minimal fail-closed fix before another real call. M6–M8 remain pending. M3/M4 bounded scope limits remain as above.

## Iteration 29 — event-stream cap failure and bounded fix
- M3/M4 remain complete within documented limits; M5–M8 remain pending.
- A third real isolated call (fresh `openai-codex` / `gpt-6-luna`, low reasoning) returned a schema-shaped answer with matching IDs and valid evidence citation, but adapter rejected it because JSONL events were 89,848 bytes vs previous 64,000-byte cap. No accepted `result.json`; raw result is preserved, not treated as verified output. Artifacts: `reports/m5-iteration29-real-investigation/`. Provider-reported usage: 1,006 input + 426 output (1,432 total including 196 reasoning), reported cost $0.0003136.
- Increased the separate event-stream ceiling to 256,000 bytes while retaining the 16,000-byte answer cap; added oversized-stream regression. Files changed: `tools/effect_investigation/model_investigation.py`, `tools/effect_investigation/test_model_investigation.py`.
- Validation: `PYTHONPATH=tools/effect_investigation python3 -m unittest test_model_investigation test_investigate` — 24 tests passed (0.121s). Readiness checks listed `gpt-6-luna` and returned ready/oauth. At iteration 29, one sequential fresh invocation still needed to exit successfully and persist a validated result.

## Iteration 30 — M5 bounded gate passed
- One fresh isolated `openai-codex` / `gpt-6-luna` / low-reasoning call used digest-verified current-checkout source and passed adapter exit, provider/model, stable candidate/site ID, schema, and evidence-ID checks. Persisted request, result, raw provider events, and manifest: `reports/m5-iteration30-real-investigation/`.
- The result is a plausible but unverified cleanup concern, not a defect finding. Provider event reports 1,006 input + 281 output = 1,287 tokens; CLI-reported cost $0.0002411, not independently billed. Four real calls total (iterations 25, 27, 29, 30); first three failed closed, iteration 30 accepted.
- Validation: `PYTHONPATH=tools/effect_investigation python3 -m unittest test_model_investigation test_investigate` — 24 tests passed. Readiness checks listed `gpt-6-luna`; auth returned ready/oauth.
- M5 is complete within the single-packet boundary. M6 independent verification, M7 aggregation, and M8 paired evaluation remain incomplete.
