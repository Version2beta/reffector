# Packet-builder scalability review

## Finding

The canonical packet run remains **incomplete**. I did not rerun it. Bounded diagnostics identify a credible, measured performance mechanism: when the 48,000-byte packet limit is crossed, the reducer repeatedly rebuilds the reduction report, recursively normalizes the whole packet, and serializes it to measure its exact canonical size for each attempted addition and rollback. The cost scales with the number of candidate graph relationships even though the packet itself is kept near 48 KB.

On a derived input containing the full `averageLoop` reverse graph (36,721 edges, one site), the stage-only builder took **31.18 seconds** at 48 KB, versus **1.20 seconds** with a 1 GB limit that avoided reduction. Both runs omitted the source model, contained only the selected target site, and are diagnostics—not the canonical full-report packet. In the original report, this candidate sorts at rank 24 of 186 and is selected by `--max-candidates 100`. The selected 100 candidates together reference 2,291,559 edges (references overlap; these are not unique graph edges), so the target-only result is not the whole workload.

The evidence supports repeated packet reduction as a major cost, but it does not establish the exact stage active when the earlier full run was stopped. That run had no stage instrumentation and was not repeated. It produced no packet files.

## Scope and inputs

The parent EffectReport is 457,954,303 bytes, SHA-256 `a1569ae03c77facbd421225d2524ba3166f84209a8911c66a107f5dc61ff2440`. It contains 215 sites, 4,996 nodes, 38,082 edges and 215 reverse rows. The target site's original reverse record has 4,405 node references (4,404 unique), 36,721 edge references, and no reverse-graph truncation. The derived target input preserves that original row and its referenced nodes and edges; its own metadata marks it as a diagnostic-only reduced scope. Its SHA-256 is `445fd392e46ee084b9a8ff81c693b9405b947b03406a352bbd764bc564bc654a`.

All derived inputs and their hashes, parent identity, relationship-label counts, and reduced-scope notes are recorded in:

`/tmp/rclone-average-loop-preflight/packet-performance-diagnostics/derived-input-manifest.json`

The 13 subsets preserve parent IDs and edge labels, validate reference closure, and do not turn a truncated subset into a complete analysis. The 512-edge candidate/established/balanced inputs select existing edges; they do not rewrite candidate status.

## Measurement method and limits

- The measured builder is a disposable copy of `tools/effect_investigation/investigate.py` with stage-boundary hooks only. The copy differs by those hooks; no repository production code or tests were changed. Original builder SHA-256: `555e60da1092dc01a489711fc665ac09b12333fa9a287b51419ec436d1a90871`.
- Wall times below come from a stage-only runner with no `cProfile` and no per-call wrappers. Each is a single run, not a statistically stable benchmark result. The runner separately measures in-memory serialization of the final result where noted; it does not write the CLI's `investigation.json` or Markdown artifacts.
- A separate `cProfile`/counter harness was used to identify call counts and bytes. Its timing is not used as the builder's elapsed time: at 8,192 edges, the profiled run took 24.63 seconds while the stage-only run took 6.79 seconds (about 3.6× slower). Call counts and canonical byte totals are useful attribution evidence, not wall-time estimates.
- Every child diagnostic was limited to 270 seconds and a sampled physical-footprint threshold of 3.5 GiB (3,758,096,384 bytes). No child hit either limit. The largest sampled footprint was 3,435,973,836 bytes (3.2 GiB) during the isolated full-report fingerprint. These are periodic `vmmap` samples, not kernel-recorded maxima.
- Environment: Python 3.15.0, macOS 27.0.1, arm64; Reffector HEAD `d766ee95bde0041549cedc1bdbc9a1a6d3a0ef29`. No network, provider/model, or production-system calls were made.

## Stage timings

All times are seconds. “Packet” is the builder's canonical packet byte count and reduction strategy. Graph prefixes are one-site, parent-derived subsets of the target reverse graph. The high-budget rows use 10 MB or 1 GB to avoid progressive reduction; the low-budget rows use the original 48 KB limit.

| Input | Unique edges / nodes | Packet limit | Build | Candidate packet loop | Packet result |
|---|---:|---:|---:|---:|---|
| Target prefix | 32 / 64 | 48 KB | 0.0093 | 0.0085 | 36,712 B, progressive |
| Target prefix | 128 / 226 | 10 MB | 0.0064 | 0.0034 | 180,256 B, expanded |
| Target prefix | 128 / 226 | 48 KB | 0.0790 | 0.0768 | 47,457 B, progressive |
| Target prefix | 512 / 692 | 10 MB | 0.0219 | 0.0121 | 608,434 B, expanded |
| Target prefix | 512 / 692 | 48 KB | 0.3633 | 0.3564 | 47,340 B, progressive |
| Target prefix | 2,048 / 1,515 | 48 KB | 1.4892 | 1.4683 | 47,389 B, progressive |
| Target prefix | 8,192 / 2,821 | 48 KB | 6.7916 | 6.7030 | 47,682 B, progressive |
| Full target reverse, no source model | 36,721 / 4,404 | 1 GB | 1.1951 | 0.6134 | 22,722,522 B, expanded |
| Full target reverse, no source model | 36,721 / 4,404 | 48 KB | 31.1793 | 30.4810 | 47,488 B, progressive |

At the larger low-budget sizes, the observed increase is roughly proportional to edge count: 512 to 2,048 edges took 0.36 to 1.49 seconds; 2,048 to 8,192 took 1.49 to 6.79 seconds. The reducer repeatedly serializes a packet held near the byte cap, so the observed workload is approximately “relationships attempted × bounded packet work” in this range—not an unbounded quadratic packet-size curve. Do not extrapolate these timings as a completion estimate for the full report; candidate shape, source resolution, and repeated reverse references vary.

The complete target-only low-budget diagnostic emitted one 47,488-byte packet. Its in-memory result serialization took 0.129 seconds and produced a 75,301,304-byte JSON representation. The corresponding expanded-budget diagnostic produced a 104,679,624-byte result representation. Neither is a canonical full-report artifact.

## Why the reducer costs so much

In `tools/effect_investigation/investigate.py`, `_try_packet_addition` calls `_refresh_reduction_report` to measure a trial. When the trial does not fit, it rolls back and refreshes again. Each refresh calls `normalize_model_paths` on the packet and repeatedly calls `canonical_json` until the reduction-report size fields stabilize.

For the 8,192-edge, 48 KB diagnostic, the counter run recorded:

- 8,105 `caller_relationships` addition attempts;
- 16,345 reduction-report refreshes;
- 49,082 `canonical_json` calls, encoding 2,400,799,818 bytes cumulatively;
- 16,348 top-level path-normalization walks and 19,255,807 recursive normalizer calls.

Those counts are from the instrumented run, not elapsed-time predictions. The stage-only run independently shows the candidate packet loop taking 6.70 of the 6.79 seconds. By comparison, graph compaction itself was only about 0.06 seconds in the attribution run. This explains why the earlier one-second sample in the JSON encoder was inconclusive: serialization is repeated inside the reducer, while the separate full-report fingerprint is a single call.

The full parent report fingerprint was measured separately. Loading took 1.337 seconds, validation 0.256 seconds, and canonical serialization plus SHA-256 1.378 seconds for 385,704,299 canonical bytes. This confirms a substantial one-time memory peak, but not an hour-scale CPU cause. The report fingerprint includes the full report object, including its 11,156-row `inventory`, not just the sites/nodes/edges the builder indexes.

## Candidate cap and source-resolution costs

The original build has 186 candidates. Reproducing its deterministic candidate ordering from the parent report places `averageLoop` at rank 24, so it is selected at the original cap of 100. Candidate reverse graphs have a lower median of 36,311 edge references and a 90th-percentile value of 36,321. Across all candidates there are 4,218,860 edge references; the selected first 100 account for 2,291,559. These counts include repeated references to the same report edges.

`--max-candidates` is not an early work bound in the current implementation. The builder iterates all candidates and builds compact graph data, resolves source evidence, and creates/normalizes the expanded packet before it checks whether the candidate is selected for emission. On a fixed 20-site, 61-edge subset, cap 1 versus cap 20 changed the packet loop only from 4.53 ms to 4.73 ms; total build time rose from 5.90 ms to 7.98 ms as more packets were retained. The cap primarily changes output, not pre-emission work.

Source-root diagnostics show another repeated cost. With the matching 45 MB program model and source manifest on a 128-edge subset, the stage-only build took 0.164 seconds at 10 MB and 0.480 seconds at 48 KB. The attribution run counted 2,429 `safe_source` calls and 101,572,841 bytes read across 114 source files at 10 MB; at 48 KB it counted 4,004 calls and 145,661,870 bytes. The files in the 10 MB run total 3,452,815 bytes on disk, so repeated reads were about 29× that unique content. `safe_source` reads and hashes file contents on each call; this is separate from the packet's canonical JSON serialization.

Candidate-density extremes at 512 edges produced expanded packet sizes from 566,003 bytes (all-candidate edges) to 659,479 bytes (all-established edges), with stage-only build times from 20.4 to 23.9 ms. The node counts also differ (598 versus 802), so this is a representative label/shape comparison, not a pure density-only experiment. It does show why an operation-centered benchmark should record both edge class and graph shape.

## Ranked corrections to evaluate (not implemented)

1. **Remove repeated whole-packet work from the progressive reducer.** Normalize stable, file-bearing evidence once per candidate and avoid walking the entire packet again on every trial. Then replace repeated full canonical serializations with exact incremental byte accounting for the fields changed by an addition/rollback, while retaining one final canonical-size check. This has the largest measured potential. The byte limit is a public behavior boundary, so estimates that can undercount UTF-8 escaping or reduction-report fields are not acceptable.
2. **Cache source reads/hashes within one build.** Key only after checking root containment and resolving the manifest-relative path; retain the verified bytes/digest for the invocation. This addresses repeated reads without weakening path checks or manifest-hash validation. Test that repeated references produce identical excerpts and that missing, outside-root, and hash-mismatch cases remain unavailable.
3. **Make the candidate cap an actual materialization bound only if the output contract permits it.** The current inventory includes source and graph details for unselected candidates. Skipping that work would require an explicit omission status and tests for what remains available; silently dropping evidence would change behavior. The current `max_candidates` value should not be described as a wall-time limit.
4. **Reduce fingerprint peak memory separately.** A streaming canonical encoder feeding SHA-256 could preserve the exact report fingerprint while avoiding the 385 MB encoded buffer and its sampled 3.2 GiB peak. This is a memory improvement, not the primary CPU fix.

Raising the packet limit or lowering the candidate cap is not a behavior-preserving fix: either changes which evidence is emitted, and the latter does not avoid the current pre-emission packet work.

## Operation-centered benchmark design

Use a no-`cProfile` runner for elapsed time and a separate attribution run for call counts. Pin and record the parent report, source snapshot, manifest, program model, builder source hash, runtime, flags, and each derived-input hash. Keep every derived graph labeled as reduced scope.

Benchmark these independent axes:

- **Graph size:** target reverse prefixes at 32, 128, 512, 2,048, 8,192, then the exact 36,721-edge target row.
- **Candidate count:** fixed 20-site input with caps 1 and 20, then the full 186-candidate input with cap 100.
- **Budget:** expanded/no-reduction, 48 KB, exact-fit boundary, and a budget below the minimum packet.
- **Relationship mix:** candidate-heavy, established-heavy, and balanced graphs at the same edge count; add a generated same-node/same-edge fixture where only the class mix changes to isolate density.
- **Source resolution:** no source model, then the matching source snapshot/model/manifest; include repeated references to the same files and report unique bytes versus bytes read.

Record stage wall times, emitted/deferred status, exact packet bytes, reducer attempts, canonical calls/bytes, source reads/hashes, output serialization size/time, and sampled physical footprint. Run several repetitions for stable small/medium cases and report medians and ranges. Use the same five-minute/3.5-GiB external guard. Keep cProfile out of timing runs.

Any optimization acceptance check should compare candidate ordering and statuses, exact canonical packet bytes at budget boundaries, source path containment/hash-failure behavior, and graph reference closure. Preserve candidate versus established labels, structural versus runtime claims, and explicit evidence omissions. None of these performance runs proves whether the candidate RC-dispatch edge executes or whether `averageLoop` leaks at runtime; the prior `reports/rclone-average-loop-dependency-retry.md` remains the source for that separate analysis.

## Artifacts and disposition

Diagnostic scripts, derived inputs, manifests, stage-only measurements, profiles, and supervisor records are under:

`/tmp/rclone-average-loop-preflight/packet-performance-diagnostics/`

No production code, tests, original rclone checkout, journal, README, or prior report was changed for this review. The original canonical full-report packet build was not rerun and remains incomplete.

**Disposition:** `PACKET_BUILDER_SCALABILITY_BOTTLENECK_MEASURED; CANONICAL_PACKET_INCOMPLETE; NO_IMPLEMENTATION`
