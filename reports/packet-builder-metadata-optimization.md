# Reduction-report metadata reuse

**Disposition: `METADATA_OPTIMIZED; CANONICAL_BUILD_INCOMPLETE`**

A per-invocation immutable source-ID set removes repeated inventory scans without caching selected packet evidence. Source-rich diagnostics show a measurable improvement and identical complete canonical output bytes. The source-rich 36,721-edge target improved from **40.15 to 16.99 seconds** in corresponding single observations. The one bounded full canonical retry still reached its 270-second deadline without output files. No full-workload speedup or completion is claimed.

## Baseline, scope and identity

Starting HEAD: `d766ee95bde0041549cedc1bdbc9a1a6d3a0ef29`. Existing tracked and untracked work was inspected and preserved.

| Implementation | SHA-256 | Preserved executable |
|---|---|---|
| Immediate baseline | `d7e69168756d079dab5cfd0bf431e8167628dc5de16582172468bb911919293b` | `immediate_baseline.py` |
| Original pre-optimization builder | `555e60da1092dc01a489711fc665ac09b12333fa9a287b51419ec436d1a90871` | `original_investigate.py` |
| Metadata-optimized builder | `f38ea2a5fe0bb344fa9b84a6464515d1ba41cdb684dbe09a6608320338f00f8c` | `metadata_optimized.py` |

Those executables and all new diagnostics are under:

`/tmp/rclone-average-loop-preflight/packet-metadata-optimization/`

The parent EffectReport was rehashed and matched `a1569ae03c77facbd421225d2524ba3166f84209a8911c66a107f5dc61ff2440`. All 13 existing derived-input hashes matched their manifest. The pinned program model, source manifest, canonical manifest-files hash, source revision `c1b5756eec76b9f0f7ee4da43fdbcae9cb73a2db`, and all **991 manifest-listed source files** were verified before measurement, before the canonical retry, and after it. The graph subsets retain their earlier reduced-scope labels and reference closure. Neither the graph nor the program model was changed to create the source-rich comparison.

Previous reports, measurements, the original reducer oracle, existing test files, `.ae/journal.ndjson`, README and Ralph artifacts were not modified. No commits, staging, network access, dependencies, model/provider calls, subagents, or original-rclone-checkout changes were made.

## Precisely what was repeated

Locations below refer to `tools/effect_investigation/investigate.py` in the **immediate baseline**:

| Location | Computation | Dependency / lifetime |
|---|---|---|
| 1003 | `source_ref_ids` | IDs currently selected in `packet['source_refs']`; dynamic |
| 1004 | `included_excerpt_ids` | IDs currently selected in `packet['source_excerpts']`; dynamic |
| 1006 | `source_ref_ids_all` | All `item['ref']['id']` values in `state['source_items']`; fixed for one progressive invocation |
| 1007 | `source_excerpt_ids_all` | The **same** `item['ref']['id']` set, reconstructed again; fixed |
| 1082–1088 | Source inclusion/defer counts | Mix of fixed inventory membership and dynamic selected IDs; raw inventory length and unavailable count remain separate |
| 1216–1222 | Progressive reducer state | Created after candidate graph/source extraction; never shared between candidates or invocations |

The two complete-inventory comprehensions use `ref.id`, not `excerpt.id`. `total_excerpt_refs` uses the raw item-list length, while deferred excerpt counts use unique IDs. These distinctions, including duplicate-ID behavior, remain unchanged.

Graph additions and rollbacks change graph nodes, edges, segments, connectivity flags and `steps`, not the complete source inventory. Source additions change only the packet's selected reference/excerpt lists. Provenance, CFG and audit additions change their packet fields and corresponding report counts; those computations remain dynamic. The complete source inventory depends on the candidate's full graph/declaration lookup **before** reduction, not on which graph relationships currently fit in the packet.

## Evidence before production edits

A disposable instrumented copy counted the four source-ID set constructions separately from report calls, graph trials, serialization and normalization. It used the existing pinned source model/manifest and unchanged derived graphs. Attribution timings include instrumentation overhead and are not the wall-clock benchmark results.

| Source-rich input | Source items | Report calls | Complete-inventory set constructions | Inventory reference visits | Time in those two set constructions, attribution run |
|---|---:|---:|---:|---:|---:|
| 128 edges | 229 | 675 | 1,350 | 309,150 | 0.0135 s |
| 512 edges | 690 | 2,368 | 4,736 | 3,267,840 | 0.1249 s |
| 2,048 edges | 1,412 | 6,884 | 13,768 | 19,440,416 | 0.8369 s |

On the same 2,048-edge graph without source evidence, those inventory visits were zero. That comparison alone does not isolate metadata cost: source resolution and normalization also differ.

A **metadata-only ablation**, before production edits, provided the narrower check. In three alternating, non-profiled pairs, the 512-edge source-rich median fell from 1.038 to 0.928 seconds; the 2,048-edge median fell from 3.285 to 2.522 seconds. Complete result hashes matched. The immutable inventory scans were therefore material at 512 edges and above, not merely a sampled-stack hypothesis. The final production runs below independently reproduced the improvement and compared full result bytes.

## Minimal correction and immutability boundary

Production changes are limited to `_reduction_report` and `_progressively_reduce_packet`:

- Each progressive invocation snapshots all source-reference IDs once as a **`frozenset`**, stored in its existing private state (`source_ref_ids_all`, current line 1227).
- `_reduction_report` reuses that snapshot (current lines 1008–1010). Expanded packets and callers without a snapshot compute a local set instead. An empty snapshot is valid and does not cause a rescan.
- The redundant excerpt-universe set is removed because it used precisely the same `ref.id` expression. Selected packet reference/excerpt sets are still rebuilt on **every refresh**.

There is no invalidation framework. The scope makes one unnecessary: `source_items` and their IDs are fully assembled before reduction; the reducer reads them, sorts a separate list for source priority, and selects into packet-local lists without changing the inventory. Normalization copies packet structures; it does not rewrite inventory IDs. Rollbacks restore packet lists/graph state and steps, never this snapshot. Changed source inventories get a new snapshot on the next invocation. No mutable source/graph evidence is reused as selected-evidence metadata.

Raw inventory length, unavailable-source count, graph connectivity/counts, provenance, CFG and audit metadata remain recomputed as before. Source containment/digest verification, canonical byte accounting, the eight-pass trial fallback, the fail-closed final independent serialization check, selection, traversal and output schema are unchanged. The documentation gained only a clarification of this reuse boundary. No source-read cache, fingerprint optimization or earlier candidate-cap boundary was added.

## Equivalence and validation

`fixtures/immediate_reduction_report.py` freezes the preceding metadata function. New tests compare complete canonical results through that oracle, the original pre-optimization reducer oracle, and the optimized implementation. No fields are masked. Comparisons use identical inputs, including any temporary source root, within each test.

The four new tests cover expanded/progressive results; exact and one-byte boundary behavior through the existing deterministic cases; minimum rejection; source-rich/source-free inputs; mixed candidate/established edges; truncation; Unicode/escaping; provenance/CFG/audit reductions; missing sources; repeated graph/source failures; selected-source additions, removal and rollback; duplicate inventory/selected IDs; deliberately different `ref.id`/`excerpt.id`; fresh snapshots after inventory changes between invocations; and no rescanning of populated **or empty** snapshots. Existing decimal-width and fail-closed checks remain intact.

Separate artifact validation compared **24 optimized complete canonical results byte-for-byte** with their immediate counterparts: 17 timing runs and seven attribution runs. It also checked exact packet/report sizes, candidate ordering/statuses, node/relationship/catalog closure, candidate resolutions, deferred graph/source references, and source inclusion/defer metadata. Two fresh runs of the **preserved original builder executable**, on 128-edge and 2,048-edge source-rich inputs, also matched the optimized bytes exactly. No evidence-integrity regression was found in these completed checks.

Validation used local dependencies and offline Go settings:

```sh
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=tools/effect_investigation \
  GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local \
  python3 -m unittest discover -s tools/effect_investigation -p 'test_*.py' -v

GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go test -count=1 -json ./...
```

- **Python: 44 tests passed**, no failures or skips, 4.915 seconds. No previous differential assertions were weakened.
- **Go: 96 top-level tests passed; 238 pass events including subtests**, no test failures or skips. Four tested packages passed; two additional packages had no test files. `-json` only made test counts observable. Supervisor wall time: 73.931 seconds; command-package test time: 71.515 seconds.
- `git diff --check` passed. Diff review found no production changes outside the two metadata-related functions.

## Before/after benchmarks

All pairs use identical inputs, `max_candidates=1`, and an exact **48,000-byte** packet budget. “Rich” adds the unchanged pinned source root, manifest and program model; “free” supplies none. Timing runs use stage hooks only, **no cProfile or per-call counters**. Counters come from separate attribution runs. Both versions capture canonical output identically; comparison happens later in a separate process.

Every performance child had an external 270-second deadline and sampled 3.5-GiB physical-memory guard, unique destination, separate stdout/stderr and recorded exit status. All diagnostic cases below completed with exit 0. Short cases have three runs; full-target cases are single observations. Environment: Python 3.15.0, macOS 27.0.1, arm64. These observations are not general performance guarantees.

| Input | Runs each | Immediate build median [range], s | Optimized build median [range], s | Candidate loop before → after, s | Speedup |
|---|---:|---:|---:|---:|---:|
| 512 free | 3 | 0.113 [0.112–0.115] | 0.112 [0.112–0.114] | 0.106 → 0.106 | 1.004× |
| 8,192 free | 3 | 1.796 [1.796–1.797] | 1.803 [1.793–1.817] | 1.711 → 1.718 | 0.997× |
| 128 rich | 3 | 0.332 [0.330–0.370] | 0.320 [0.319–0.321] | 0.276 → 0.265 | 1.039× |
| 512 rich | 3 | 1.059 [1.051–1.083] | 0.937 [0.936–0.938] | 0.934 → 0.812 | 1.130× |
| 2,048 rich | 3 | 3.302 [3.266–3.303] | 2.529 [2.526–2.541] | 2.996 → 2.226 | 1.306× |
| 36,721 free | 1 | 8.962 | 8.960 | 8.601 → 8.599 | 1.000× |
| 36,721 rich | 1 | 40.146 | 16.985 | 36.663 → 13.107 | 2.364× |

Source-free results show no meaningful speedup. Source-rich improvements grow with repeated inventory work. Attribution confirms unchanged report calls, graph/source decisions, serialization and normalization, while complete-inventory constructions drop to two per case: one expanded measurement and one progressive snapshot. Selected packet sets still have **two constructions per report call in both versions**.

| Input | Report calls, both | Inventory set constructions before → after | Inventory reference visits before → after | Graph attempts / rollbacks, both |
|---|---:|---:|---:|---:|
| 512 free | 990 | 1,980 → 2 | 0 → 0 | 512 / 474 |
| 8,192 free | 16,345 | 32,690 → 2 | 0 → 0 | 8,190 / 8,151 |
| 128 rich | 675 | 1,350 → 2 | 309,150 → 458 | 128 / 87 |
| 512 rich | 2,368 | 4,736 → 2 | 3,267,840 → 1,380 | 512 / 474 |
| 2,048 rich | 6,884 | 13,768 → 2 | 19,440,416 → 2,824 | 2,048 / 2,010 |
| 36,721 free | 73,398 | 146,796 → 2 | 0 → 0 | 36,716 / 36,678 |
| 36,721 rich | 80,166 | 160,332 → 2 | 542,723,820 → 6,770 | 36,716 / 36,678 |

Graph attempts include direct relationships, segments and segment headers. Source addition/rollback counts also matched exactly; the 2,048-edge rich case attempted and rolled back 1,411 source additions, and the full rich target did the same for 3,384. The snapshot remains valid through those failures because only selected packet evidence changes.

| Input | Canonical calls, both | Cumulative encoded bytes, both | Normalization invocations including recursion, both | Peak sampled MiB before → after |
|---|---:|---:|---:|---:|
| 512 free | 997 | 49,780,254 | 80,777 | 28.3 → 28.5 |
| 8,192 free | 16,395 | 816,146,759 | 803,897 | 143.9 → 144.1 |
| 128 rich | 918 | 34,161,700 | 546,896 | 282.2 → 282.0 |
| 512 rich | 3,089 | 120,027,213 | 1,641,663 | 298.1 → 296.2 |
| 2,048 rich | 8,406 | 347,870,087 | 3,443,973 | 339.1 → 339.2 |
| 36,721 free | 73,539 | 3,641,129,565 | 3,163,155 | 493.3 → 493.3 |
| 36,721 rich | 84,182 | 3,992,877,358 | 10,734,044 | 882.9 → 882.1 |

Memory is a periodic physical-footprint sample, not a kernel maximum; it includes parsing and result capture plus a short post-run hold. No memory improvement is claimed.

All cases emitted **one progressive packet, none deferred**; omissions and recovery metadata matched completely. Exact canonical result sizes/hashes below are equal before and after, not just equal lengths:

| Input | Complete canonical result bytes | Packet bytes | Complete result SHA-256 |
|---|---:|---:|---|
| 512 free | 1,450,215 | 47,340 | `053a4d63792909da844a34c01afd45a23c3711a2d464f371afb95936e7ee8300` |
| 8,192 free | 14,398,701 | 47,682 | `d65d7311e5a446c0413ed290a1c6f82969e878d2eab606179ca9f2a9b7e56468` |
| 128 rich | 544,025 | 47,527 | `d23f9dbe6239326a229822fc6fdda90756e59f3a165d5885a09e2ec9d019a5f3` |
| 512 rich | 1,740,383 | 47,414 | `f045fba68b9c6893324b0228f4bb22105555343b770c34ec3550aff4c7a558d4` |
| 2,048 rich | 4,990,234 | 47,462 | `e175abb679cfa07e340a4edf5bd72558e19fff7224dbef39e82d30e3ea6a9953` |
| 36,721 free | 56,192,716 | 47,488 | `28ddd20322f38aedee5d062a9038e2a2bba162916c171d734f3eb3514229d082` |
| 36,721 rich | 56,874,262 | 47,563 | `7bb0e7657fd5ad8da14f18e8a1b69ef1ce7978b0f3a30a09f3f6b6b50a6b63b8` |

For example, the full rich target still reports 3,385 total/deferred excerpt IDs, zero included excerpts, one included source reference, 3,384 deferred source references and 1,189 unavailable-source records. Faster metadata calculation does not conceal omissions or improve evidence coverage.

## One bounded canonical retry

All five gates passed before launch: existing/new tests, complete-result equivalence, a source-rich improvement, completed full-target diagnostics, and no detected integrity regression. The existing CLI entry point ran against the unchanged parent report and pinned source inputs with the original settings `--max-candidates 100 --max-packet-bytes 48000`, changing only the output destination. A `runpy` wrapper enabled periodic stack diagnostics; it did not replace builder functions or use the diagnostic builder runner.

- **Incomplete:** external child deadline fired at 270 seconds; supervisor wall time **270.079 seconds**, including termination handling.
- Child exit **−15 (SIGTERM)**, supervisor status 124. No automatic retry.
- Peak sampled physical footprint **2,576,980,377 bytes (about 2.4 GiB)**, below the 3.5-GiB guard.
- Stdout was empty and the output directory did not exist. There are no full canonical counts, packets or artifacts to validate, and no equivalence claim for a completed full build.

Of 53 periodic stack dumps, 32 included JSON `iterencode`, eight included normalization, five included `safe_source`, and 18 included progressive reduction. Categories overlap; **29 dumps contained invalid/incomplete frames**. Only two showed `_reduction_report`, and these observations do not establish an exhaustive cost distribution. They suggest remaining encoding and source/normalization work, but do not prove which stage dominates the full run or how many candidates were processed. The diagnostic timer's repeated “Timeout (0:00:05)” messages are sampling intervals, not repeated build attempts.

## Remaining limits and next action

This change removes the measured repeated inventory scans. Per-trial canonical measurement, dynamic report reconstruction, source verification, full candidate materialization and full-report fingerprinting remain unchanged. The full rich target still encodes roughly 4 GB cumulatively during one completed diagnostic; that count alone does not establish its share of elapsed time. Single-candidate speedups must not be extrapolated to the 100-candidate workload.

The smallest justified next action is a separately authorized, bounded phase-level timing/profile of the remaining source-rich full-build work, with sampling that reliably identifies Python stages. The current incomplete stacks are not a sound basis for choosing another cache, changing selection or traversal, or redesigning extraction. No excluded optimization was implemented merely to make the full build finish.

Reproduction, commands, executable copies, attribution counters, complete canonical bytes, memory/exit records and validation summaries are retained in `reproduction/`, `benchmark/`, `measurement-summary.json`, `validate_outputs-run/`, the test-suite directories, and `full-canonical-attempt/` under the new artifact root.

`METADATA_OPTIMIZED; CANONICAL_BUILD_INCOMPLETE`
