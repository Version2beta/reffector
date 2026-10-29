# M8 controlled pair — iteration 34

## Execution and verification
The preregistration (`preregistration.md`) predates both calls and fixes the task, identical question, truth criteria, rubric, model/reasoning, order, budgets and invalidation conditions. Source-only ran first, followed sequentially by the Reffector-supported arm, in distinct `pi --no-session` calls. Both exited 0 and raw assistant events report `openai-codex` / `gpt-6-luna`; no retry was made. Readiness was checked before invocation (`pi --list-models openai-codex` listed `gpt-6-luna`; auth returned `ready`, `oauth`).

- Source-only artifacts: `source-only-request.json`, `source-only-answer.txt`, `source-only-provider-events.jsonl`, `source-only-manifest.json`, `source-only-stderr.txt`.
- Treatment artifacts: `reffector-request.json`, `reffector-supported-answer.txt`, `reffector-supported-provider-events.jsonl`, `reffector-supported-manifest.json`, `reffector-supported-stderr.txt`.
- Shared setup: `invocation-setup.json`; pre-registration: `preregistration.md`.
- Source pin is repo HEAD `d766ee95bde0041549cedc1bdbc9a1a6d3a0ef29`; source hashes are recorded in preregistration and matched current files. The two inspected source files have no working-tree diff.

Both arms received the same question and the same 26 source lines (`loading.go:73-93`, `analyzer.go:139-143`). The source-only request was 1,583 bytes. Treatment request was 4,888 bytes and additionally included the canonical `os.RemoveAll` effect packet, one evidence entry, and packet context/limitations. There was no shared conversation. The excerpt selection unintentionally omitted `loading.go:94`, the line returning `cleanup`; this affects both arms equally and is a recorded limitation, not silently treated as unavailable in the pinned source. The full source was independently reviewed after responses.

## Results against the preregistered rubric

| Criterion (0–2) | Source-only | Reffector-supported | Independent check |
|---|---:|---:|---|
| Error cleanup paths | 2 | 2 | Both identified cleanup calls for `.work` and `.sum` write errors and non-NotExist `.sum` read errors (`loading.go:80-91`). |
| Returned callback and caller defer | 1 | 1 | Both correctly cited `AnalyzeReport`'s defer (`analyzer.go:139-143`) but said the supplied source could not establish callback return. Full source line 94 explicitly returns `cfg, cleanup, nil`; both missed this because the excerpt stopped at 93. |
| Attempt vs successful removal | 2 | 2 | Both correctly noted `_ = os.RemoveAll(tmp)` ignores errors (`loading.go:78`), so successful deletion is not proven. |
| Avoid unsupported defect claim | 2 | 2 | Both returned `defect: null`; no source-supported correctness defect was asserted. |
| Uncertainty/runtime limitation | 2 | 2 | Both limited claims to source and disclaimed runtime confirmation. |
| **Total / 10** | **9** | **9** | No score advantage for treatment on this pair. |

Full-source review confirms the bounded truth criteria: post-temp write/read error branches call cleanup; success returns it at line 94; `AnalyzeReport` defers it after successful `loadConfig`; the removal error is ignored. Thus attempted cleanup is source-supported for the represented paths and ordinary caller; successful removal is not guaranteed. Neither model arm establishes a runtime defect. Both answers correctly avoid a defect claim, but both miss the return line due to the shared excerpt omission. This is a real miss and illustrates that packet evidence does not repair incomplete retrieval.

## Effort and observed usage
One call per arm. Source-only: 11.15 s, 629 input + 489 output = 1,118 reported tokens, reported cost $0.0003074. Treatment: 9.365 s, 1,688 input + 464 output = 2,152 reported tokens, reported cost $0.0004008. Combined CLI-reported cost $0.0007082; these are provider-event reports, not independently billed amounts. No other usage/cost inferred. Both requests contained 26 source lines; treatment additionally received one packet/effect evidence item. Treatment context increased request size by 3,305 bytes and reported input by 1,059 tokens. No measurable answer-quality improvement under the fixed rubric in this single pair.

## Qualified conclusion
This is a valid, traceable, source-pinned paired comparison with independent fresh sessions, same verified model/reasoning, same question, matched source excerpts, bounded calls and a precommitted rubric. It provides only a null result for this one task: no observed quality gain, and both arms shared the same missed source line. It does not establish causal or general usefulness. No code changed in this iteration. M8 passes as a bounded controlled comparison; preserve the miss and null result rather than generalizing.
