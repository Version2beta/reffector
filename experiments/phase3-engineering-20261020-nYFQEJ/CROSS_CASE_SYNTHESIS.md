# Phase 3 — Cross-case synthesis

## Evidence boundary

Five fresh, independent Pi CLI sessions completed without timeout (222–257 seconds each). Each selected `openai-codex/gpt-6.1-sol`, reasoning `high`, loaded the same explicit phase2-1 Reffector skill, and used the same frozen executable: SHA-256 `ad9f0ac67b2d512880197e24e4b6be7077553497750380a4af7bc6c8c138ff97`, built from `5553a1d13011bf638148661f6a23bf561c3eef5a` with `vcs.modified=true`. Selection metadata is not independent backend-model attestation. Both third-party checkouts remained clean and pinned after all runs. The reports, JSONL transcripts, prompts/hashes, raw analyzer output, supervisor audit, and pre/postflight records are preserved in this directory; the archive made before this synthesis is `../phase3-engineering-20261020-nYFQEJ-artifacts.zip` (SHA-256 `84574430c9f8628e7c2bd4fc11b886af1ded61fbab9c665f26a3554122031b1d`).

This is a heterogeneous engineering exploration, not a control/treatment benchmark. Analyzer coverage and agent behavior are evidence about investigation support; they do not prove that a proposed repository change is beneficial.

## By case

### A — Caddy architectural simplification

**Outcome:** plausibly useful but unverified: factor the three custom DNS resolver/dialer constructions in `reverseproxy` while retaining independent response-state machines. The proposed helper is bounded and preserves distinct timeout/default and caller responsibilities. It is simpler only if it preserves late resolver reads and does not invent shared policy.

**Material Reffector contribution:** it surfaced the three direct `net.(*Dialer).DialContext` sites and separated those from unrelated response-writing sites, giving a concrete comparison set. Source follow-up established real duplicated construction and meaningful non-duplication elsewhere.

**Missing/misleading/not pursued:** callback assignment is not callback invocation; reverse graphs supplied no caller edges for the callback sites. Build selection excluded tests and platform variants. A first invocation used invalid `-max-edges 0`; it failed before loading and was corrected.

**Skill:** helped by discouraging site-count-driven consolidation and requiring source validation. Its comprehensive preflight guidance added some repeated provenance work but no material bias.

**Smallest refinement:** expose callback/function-value assignment as an explicit uncertainty/relationship (for example, “stored as resolver Dial callback”), without claiming execution.

### B — restic side-effect isolation

**Outcome:** weak-to-moderate implementation candidate: a private pure-ish classifier for indexed-pack actions inside prune planning. The report correctly rejects larger plan/executor splits for Local.Save, upgrade recovery, and lock refresh because ownership and time ordering are the point of those protocols. The extraction is worthwhile only if a decision table becomes materially clearer than the current callback.

**Material Reffector contribution:** the unresolved `Repository.List` boundary and caller graph identified the narrow planning seam and showed it was shared by unrelated repository work, preventing a mistaken backend-policy consolidation.

**Missing/misleading/not pursued:** interface dispatch leaves the concrete backend/path unproven; category searches also matched interface type names, so operation-name results required source checking. No runtime or cross-platform evidence was gathered.

**Skill:** helped maintain the “do not split temporal protocols merely for purity” discipline. It adds friction through mandatory coverage narration, but that directly prevented overclaiming.

**Smallest refinement:** show API match kind (call versus type/signature/reference) in output or add a call-only selector, reducing false leads from names such as `ParallelRemove`.

### C — Caddy reverse-proxy header impact

**Outcome:** valid and useful impact analysis, not an implementation proposal. It establishes that a `finalizeResponse`-only change would miss independent response commitments: handled/replacement paths, server error routes, 1xx, HTTP upgrades, and WebTransport. Header ownership/precedence must be specified before coding.

**Material Reffector contribution:** direct `WriteHeader` sites and reverse callers made the ordinary shared path visible; dependency-site reachability exposed WebTransport’s separate commit boundary.

**Missing/misleading/not pursued:** `-operation Upgrade` returned no sites even though source/dependency inspection found relevant upgrade behavior. Interface dispatch, callbacks, implicit writes, wrappers, error routing, and HTTP protocol/runtime behavior are outside a complete static claim. Scope also excluded broader Caddy packages.

**Skill:** helped by making uncertainty and dependency sites reportable rather than treating the primary finalizer as complete coverage. It did not prevent the API-name query from being an ineffective route to semantic upgrade coverage.

**Smallest refinement:** add semantic operation vocabulary/aliases for HTTP upgrade/commit boundaries, or document a response-commit query recipe based on known APIs rather than method-name substring search.

### D — restic local replacement test design

**Outcome:** strongest immediately actionable follow-on: three focused test families around `(*Local).Save` distinguish pre-publication failure, post-rename directory-sync failure, and retry policy. They increase behavioral confidence without changing production responsibilities. A minimal private directory-sync test hook may be justified only for the post-publication case.

**Material Reffector contribution:** it mapped Sync, Rename, deferred cleanup, directory sync, and the unresolved `Backend.Save` boundary into a single persistence lifecycle, directing source reading to the right assertions rather than merely generating test ideas.

**Missing/misleading/not pursued:** no concrete interface edge proves callers reaching Local; platform differences and crash durability remain untested. The audit’s coverage counts do not prove error-path completeness.

**Skill:** strongly helped: it drove a boundary-first test plan and kept “failure returned” distinct from “publication rolled back.” Little friction beyond repeatable provenance.

**Smallest refinement:** add a compact per-site lifecycle view grouping acquisition, publication, cleanup, and release sites by containing function. It should remain descriptive, not infer transactional guarantees.

### E — restic historical review

**Outcome:** justified no-production-change decision. The selected historical short-ciphertext guards appear bounded and correctly placed; source inspection explains why cache deletion and backend reader ownership are not implied regressions. The appropriate follow-up is caller-policy/boundary testing.

**Material Reffector contribution:** current source-site and reverse-caller evidence quickly falsified a tempting but unsupported cache-deletion path. It also kept unresolved backend loading separate from Local implementation evidence.

**Missing/misleading/not pursued:** Reffector analyzed only the pinned current tree, not the historical diff; the agent correctly used `git show` for that. No runtime validation, alternative backends, or historical-tree analyzer comparison occurred.

**Skill:** accurately set expectations that Reffector has no revision-diff feature and prevented a false historical-analysis claim. The manual join between diff lines and current sites remains laborious.

**Smallest refinement:** none required in the analyzer. Add one skill sentence: use `git show` for historical deltas and label current-tree analysis separately.

## Cross-case conclusions

Reffector was most useful as a **lead generator and boundary map**, not a decision oracle. The recurring valuable evidence was: exact consequential source sites, shared versus distinct direct sites, reverse callers where resolved, dependency-site visibility, and explicit uncertainty. The agents then made the engineering decision from focused source/test-contract reading. Its strongest support was test design (D) and impact scoping (C); its weakest coverage was callback/interface reachability (A/B/D) and semantic behavior not named by an API query (C).

The Phase 2 skill was directionally helpful in all five cases. It biased investigation toward selective queries, source follow-up, coverage limits, and rejection of “effect = defect” reasoning—appropriate bias for this task. Friction was mostly duplicated per-run provenance and repeated explanation of known limits; that cost is acceptable for experiments, but a concise report template could reduce it in routine investigations.

## Shortlist for separately authorized implementation/testing

1. **D:** add the Local.Save replacement-state and retry tests first; evaluate a narrow directory-sync hook only if real fault injection cannot express the post-publication branch.
2. **C:** write an explicit `X-Audit-Trace` ownership/precedence contract and response-path matrix before choosing an implementation.
3. **A:** prototype only the private reverse-proxy DNS resolver-construction helper, with resolver-mutation and default-policy characterization tests.
4. **B:** do not extract `decidePackAction` until a decision table demonstrates reduced rule duplication; keep the existing planning/execution boundary.
5. **E:** no production change; consider only boundary/caller-policy tests if those contracts matter.

No proposal has implementation or runtime-test evidence from this phase.

## Later controlled comparisons

If controlled comparisons are later authorized, prioritize **C** (impact-path completeness), **D** (test-plan quality and missed publication/retry distinctions), and **A** (whether analyzer-led source comparison avoids unsound consolidation). **B** is secondary because its value claim is largely subjective simplification. **E** should not receive priority: its useful outcome was a bounded no-change review, and a comparison would need a distinct review-quality protocol. Such comparisons must use isolated prompts, fixed repository revisions, predeclared scoring, and source-only baselines; none was run here.
