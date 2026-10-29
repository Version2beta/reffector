# Reffector Ralph loop — finish stages 3–8

You are **one fresh Pi worker** in a finite, externally supervised engineering loop. The supervisor provides this task and the latest `state.md` in your initial prompt, but they are *reports*, not proof of code behavior. Read the files on disk and inspect the actual checkout. Complete one **meaningful, verifiable outcome** per iteration, write a durable handoff, and exit. Never spawn another Ralph runner or an uncontrolled agent tree.

## Objective

Make Reffector's **effect-directed defect-investigation** workflow usable and measurably evaluable: start at consequential operations, retain deterministic call/provenance/control-flow evidence and its uncertainty, ask an isolated model a focused question, independently challenge its answer, and compare against source-only investigation. Do not pivot to whole-program architecture characterization or add complexity merely to make the tooling more elaborate.

**This is a continuation, not a new audit.** The prior loop used 16 iterations, reached the iteration ceiling in about 27 minutes, and did not complete milestones 3–8. Its reports found 57 candidates, all emitted, in the then-current Reffector-self-analysis; none were deferred by candidate count or byte budget. Numerous selected source sites were reviewed, with no demonstrated defect or justification for a selection change. Read the existing `state.md` and reports before considering new work; verify their claims against source and reproducible commands when they are important. **Do not repeat candidate-by-candidate spot checks on the same inventory.**

## Progress policy — outcome over activity

- Work on the **earliest incomplete milestone**, but do not mistake an open-ended investigation for a gate that can never close. Complete the explicitly bounded gate below, document limitations, and **advance in the next iteration**.
- Within each iteration, decide the concrete observable result **before** implementing or investigating it. Prefer a tested feature, gate decision, reproducible negative test, or reproducible paired experiment over another narrative sample.
- A gate can pass with an evidence-backed **"no change warranted"** conclusion when its purpose is to decide whether refinement is necessary. A limitation in scope does not automatically block it. A gate requiring implemented/tested functionality or an actual model experiment cannot be passed by a plan or documentation alone.
- **Do not fabricate completion.** If a mandatory capability cannot be demonstrated safely, diagnose and fix the smallest relevant blocker, or write `BLOCKED` with exact evidence and the external decision/dependency required. A time or iteration budget is not a reason to mark incomplete work complete.
- Avoid spending more than the indicated *target* number of new iterations in any milestone: **M3: 1; M4: 2; M5: 4; M6: 3; M7: 2; M8: 4** (16 total). These are planning targets, **not authority to skip a gate**. If a milestone overruns, explain why in `state.md` and prefer removing unnecessary scope over further serial sampling.
- If two successive iterations do not materially advance a gate (new tested behavior, verified conclusion, better failure diagnosis, or an accepted evidence artifact), **change approach or declare a specific blocker**. Rewriting or appending a similar report does not count as progress.
- Reserve opportunities for real isolated investigations and a paired evaluation. Don't spend the entire run perfecting deterministic analysis before testing whether it helps.

## Prerequisites, scope, and known limitations

1. Earlier stage 1, **progressive packet reduction**, was reported implemented. Stage 2, **removal of the default 100-candidate processing cap**, was *not* implemented; the builder retains a complete inventory and explicit `deferred_max_candidates`. Both remain outside this assignment. Verify their current behavior only as needed; don't silently broaden the mission. The present 57-candidate inventory doesn't exercise count deferral. A narrowly scoped fixture may establish deferral reporting without implementing stage 2.
2. The previous rclone local checkout did **not** match pinned revision `81e7372f6fbedfa80592e9a564593d63a8dc32f3`. Do not analyze a different revision and label it pinned. Use existing grounded Reffector or fixture evidence for development and explicitly scope conclusions, or mark a *specific external benchmark* blocked if its pinned source is genuinely indispensable. Never change an external source checkout.
3. Use existing `tools/effect_investigation/`, `docs/effect-directed-investigations.md`, Go analyzer and tests. Read repo `README.md` from `git show HEAD:README.md` when necessary: a Ralph-bundle README was previously copied over it. **Do not overwrite or silently restore files with unrelated user changes.** Keep the supervisor files outside the repository if possible; leave the project's root README intact.
4. The user's authorization permits additional **`openai-codex` model calls** for bounded investigations and comparisons. The supervising Pi worker is pinned to that provider. Every further model session must likewise pin the exact `openai-codex` provider and a verified installed model; inspect actual CLI and returned metadata before accepting results. Never fall back to a different provider or API-key route. Run extra model sessions sequentially and track counts, model/reasoning and reported usage. If metadata cannot be verified, **stop model experiments** rather than pretending they occurred. Provider quota is finite; do not attempt credit purchases. A fake adapter does not establish a real model result.

## Milestone 3 — Close the bounded candidate-quality audit (target: 1 iteration)

**Do next.** Consolidate earlier reports into **one short audit decision** instead of repeating another candidate sample. Recheck enough source and tool output to substantiate the existing denominator, selection categories, direct-site vs whole-graph fan-in distinction, candidate/established edge separation, async cases, source support, and progressive reduction evidence. Cite earlier reports with exact paths; record nonindependent/overlapping samples and what the inventory cannot establish.

Acknowledge absent count-deferrals, absent rejected packets in this run, missing source-manifest excerpts and missing pinned rclone revision as **coverage limits**, not reasons to indefinitely sample the same 57 candidates. Do not claim a representative defect-detection rate, severity ordering, or false-positive rate from purposive source reviews. Decide whether any deterministic selection-code change is actually justified by observed evidence; if not, **make no selection change**.

**PASS when:** a reproducible bounded audit synthesis includes inventory denominator, sample method and overlap, source-grounded false leads, known untested strata and a defensible selection/no-change decision. Update milestone 3 to `complete (bounded current-repo audit)` and advance to 4. **FAIL/BLOCK only if** earlier evidence cannot be verified sufficiently to make even that scoped conclusion; name the precise missing verification. No further per-candidate sampling absent a specifically demonstrated gap in this gate.

## Milestone 4 — Test provenance and CFG only where needed (target: 2 iterations)

Inspect existing packet-to-SSA joins, argument origins, candidate-only calls, CFG summaries/truncation and uncertainty. Use **one or two adversarial, reproducible fixtures** to challenge a consequential claim—not merely reread code. Prefer cases where wrongly promoting a candidate relation or inventing a control condition would mislead a defect investigation.

If behavior is wrong, make a **narrowly scoped change with regression test**. If existing behavior conservatively preserves uncertainty and no material gap is found, record `no change warranted` with concrete test/evidence. Do not implement speculative broad taint analysis, alias solving or symbolic execution.

**PASS when:** verified regression(s) and a clear fix, or documented negative/adversarial tests that justify no change. Explicitly distinguish unmodeled values, unsupported paths and established evidence. Advance to 5.

## Milestone 5 — Make one real isolated model investigation work (target: 4 iterations)

Implement the smallest **optional, bounded invocation path** from a candidate packet to a fresh independent model context, with read-only, repository-confined retrieval. Reuse existing projections rather than cloning them. Each invocation gets only its designated candidate/current relevant source, not prior conversations or other candidates. Source excerpts, evidence IDs, relationships and truncation/deferred references must remain traceable. Do not expose machine-local paths in model-facing evidence.

Require machine-checkable output with stable candidate/site IDs; `hypothesis` or explicit `none supported`; supporting **and contrary** evidence references; alternatives, limitations and a falsifiable verification plan. Distinguish a model-generated possibility from an observed defect. Bound prompt size, retrieval, response size and repeat calls; fail closed on output/schema/provider errors.

First test with a deterministic fake adapter, including malformed responses and unrelated/unsafe retrieval paths. Then carry out **at least one real, sequential, freshly isolated `openai-codex` investigation** if model metadata and budget are verifiable. Save exact invocation setup, request/evidence manifest, raw structured result, and observed provider/model/usage when available. A failed actual call is evidence of failure, not a passing gate.

**PASS when:** the integration is tested and a real isolated result is recorded with verified provider/model. If an external access/authorization problem prevents the real call, write `BLOCKED` with exact reason rather than marking stage 5 complete.

## Milestone 6 — Independently challenge investigation claims (target: 3 iterations)

Verify model claims mechanically against canonical sites, source paths/locations, evidence IDs, established/candidate/unresolved edges and provenance/CFG bounds. **Reject** fabricated citations, mismatched APIs, invented paths and candidate edges promoted to established calls; test those rejection cases.

Classify each issue separately as `contradicted`, `plausible-unverified`, `supported-static`, `runtime-confirmed` (only from actual reproduction), or `unresolved`. An independent verifier should receive minimal original evidence rather than an advocacy narrative. A fresh model verification may be used only with verified `openai-codex`; static checks remain authoritative for mechanical claims. Do not use live targets or unsafe exploit attempts.

**PASS when:** source-linked validation succeeds for one real investigation result, negative fixtures reject unsupported claims, and at least one genuinely independent check (separate fixture/source reproduction or fresh verifier) challenges a real result. If that independent check is impossible, record the specific blocker; do not waive the gate. Preserve contrary evidence and uncertainty. Advance to 7.

## Milestone 7 — Aggregate without inventing common causes (target: 2 iterations)

Create a bounded deterministic grouping of investigations by canonical effect/site, supported shared caller segments, resource identity and repeated obligation. De-duplicate genuine duplicates; do **not** group solely by effect category or similarly named resources. Preserve original finding IDs, status, contradictory evidence, and provenance of provisional grouping.

**PASS when:** a source/evidence-linked aggregation is produced and tests demonstrate both a valid group and a false-group rejection. An empty group or "no simplification justified" is acceptable if supported by actual input and tests. Advance to 8.

## Milestone 8 — Small controlled usefulness comparison (target: 4 iterations)

Compare an ordinary source-only investigation with a Reffector-supported investigation using the **same verified `openai-codex` model and reasoning level**, independent fresh sessions, comparable accessible source, identical question and fixed beforehand rubric. Pin the source revision for the chosen task; don't claim to have used pinned rclone when it is unavailable. Choose a testable known issue or carefully specified negative control, and include context/retrieval effort in the accounting.

Before seeing treatment outputs, write the task selection, truth criteria, scoring rubric, expected source access, call/time/token limits and what would invalidate the pair. Run the two arms sequentially. Independently review claims against source or reproductions; record misses, unsupported findings, quality, effort, usage/cost **only where observed**. Avoid causal or general efficacy conclusions from a single pair, and retain null results.

**PASS when:** a genuinely paired, traceable experiment and qualified report exist; not merely a harness, plan or solo treatment run. **BLOCK when:** valid arms cannot be executed under provider, source or safety constraints; preserve a reproducible proposed experiment and exact blocker. Never manufacture results to finish the sequence.

## Hard restrictions

- Never commit, stage, push, reset, clean, delete untracked work, change credentials/configuration, install global software, or alter historical evaluation evidence.
- Never modify `.ae/journal.ndjson` or external pinned source repositories. Respect dirty/untracked work in Reffector. Record `git status --short` before and after changes.
- No uncontrolled nested agents, parallel model subprocesses, changing providers, infinite loops, or extending the supervisor's eight-hour deadline. No live exploit targets.
- Do not treat source review, Go tests, synthetic model fixtures, or agreement between models as proof of a real vulnerability.
- Use current source, focused tests and independent evidence. Report any failing validations honestly. Keep assessment data and logs in clearly separated paths; do not replace the project's README.

## Required handoff at the end of every iteration

1. Update **the existing** `state.md` in the supervisor directory; preserve prior evidence rather than starting over. State the current milestone and its status (`pending`, `complete (bounded ...)`, or `blocked`), what *materially* changed, reproducible validation and exact output paths, provider/model/call counts, limitations and the **next acceptance gate**, not merely another candidate to sample. Update the cumulative iteration count accurately.
2. If the gate passed, explicitly mark it complete and make the next milestone the next action. If not, identify exactly what must change for it to pass. Do not loop on essentially identical sample reviews.
3. Write `BLOCKED` in the supervisor directory only for a genuine preventing dependency/authorization/provider failure, with evidence and required user decision. Write `DONE` only if **all six** stages 3–8 satisfy their gates. Neither marker is an assertion that the supervisor itself independently verified the work.
4. Maintain a concise `REPORT.md` for the morning: completed and incomplete stages, changed code paths, evidence, tests, source revisions, actual model experiments and observed usage, negative outcomes and next decisions. A completed *bounded* gate must retain its scope limits in the report.
5. End the worker process normally; the external supervisor alone determines whether another iteration may run.
