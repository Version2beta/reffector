# Canonical packet reducer optimization

**Disposition: `REDUCER_OPTIMIZED; CANONICAL_BUILD_INCOMPLETE`**

The localized optimization preserves the canonical bytes of every completed differential fixture and required target-only diagnostic. The 36,721-edge target at 48,000 bytes improved from **31.59 to 8.92 seconds** in corresponding single observations. The one permitted full-report attempt nevertheless reached its external time limit without emitting files. This does **not** establish full-build scalability.

## Baseline and provenance

Work started at HEAD `d766ee95bde0041549cedc1bdbc9a1a6d3a0ef29`, with pre-existing dirty and untracked work. The pre-optimization builder was copied before production edits:

- Executable copy: `/tmp/rclone-average-loop-preflight/packet-reducer-optimization/original_investigate.py`
- Original source SHA-256: `555e60da1092dc01a489711fc665ac09b12333fa9a287b51419ec436d1a90871`
- Final optimized source SHA-256: `d7e69168756d079dab5cfd0bf431e8167628dc5de16582172468bb911919293b`

The parent EffectReport was rehashed and matched `a1569ae03c77facbd421225d2524ba3166f84209a8911c66a107f5dc61ff2440`. All 13 previous derived-input hashes matched their manifest. The full target-only derived report matched `445fd392e46ee084b9a8ff81c693b9405b947b03406a352bbd764bc564bc654a`. These remain explicitly reduced-scope diagnostics, not replacements for the parent report.

Before the full attempt, the archived command, source-manifest and program-model file hashes, pinned revision, and all **991 manifest-listed source file hashes** were reverified. The model's manifest join is the canonical hash of the manifest's `files` map, distinct from the pretty-printed manifest file hash; both identities were checked. Source revision: `c1b5756eec76b9f0f7ee4da43fdbcae9cb73a2db`.

Artifacts are exclusively under the new directory:

`/tmp/rclone-average-loop-preflight/packet-reducer-optimization/`

The previous diagnostics, EffectReports and review documents were not modified.

## Original measured problem

The earlier review found 49,082 canonical serialization calls, 2,400,799,818 cumulative serialized bytes and 19,255,807 recursive normalization invocations for the 8,192-edge, 48 KB diagnostic. Each graph addition, and frequently its rollback, refreshed the reduction report by normalizing the entire packet and repeatedly serializing it until its two size integers stabilized. Graph trials could therefore do this work thousands of times without introducing any new location fields.

The current baseline attribution reproduced those three counts exactly. The approximately hour-long historical full build was not rerun unbounded.

## Production changes and invariants

Production edits are confined to `_refresh_reduction_report`, `_try_packet_addition`, and the directly necessary `_checked_reduction_size` helper in `tools/effect_investigation/investigate.py`.

1. **Skip normalization only for path-free graph trials.** The minimum packet has already been normalized. The three graph-addition labels introduce IDs, package/function strings, relationship classes and counts, not `file` or `path` fields. Those additions and their rollbacks reuse the existing normalized evidence. Source, provenance, control-flow and audit additions still use full normalization, including containment and file-hash checks. Final refreshes also normalize normally. No shared canonical graph/source records are mutated, and there is no source-read cache.
2. **Serialize once per refresh, not once per size-convergence pass.** After constructing the fresh reduction report, only `final_serialized_bytes` and `net_bytes_saved` change in that convergence loop. The code measures the entire canonical UTF-8 representation once, subtracts the two integer spellings, and adds their exact decimal widths on each pass. This includes the minus sign for negative savings. All other escaping, multibyte text, key ordering, arrays, nested delimiters and field additions/removals are handled by the actual canonical encoder on each fresh trial. Nothing is cached across trials.
3. **Independently check completed packets.** Expanded packets, minimum packets (including rejected ones), and final progressive packets receive an independent canonical serialization. A mismatch with the predicted size or reduction metadata raises `ValueError`; an inconsistent or oversized packet is not silently accepted. Existing exact budget comparisons remain in place.

Rollback refreshes were deliberately retained. Restoring the graph can itself refresh graph flags/count fields, so blindly restoring an old byte count or old reduction report would not be an adequate proof of equivalence. A general field-contribution/rollback accounting subsystem was rejected as unnecessary complexity for this measured improvement.

Candidate selection/order, graph traversal and expansion, source extraction, evidence identities, schemas, omission rules and model/verification/aggregation code were not changed. `docs/effect-directed-investigations.md` gained only a short reducer-work-reuse section; its pre-existing edits were preserved.

### Separate existing size-report boundary

A small synthetic decimal-width fixture exposed a pre-existing eight-pass oscillation: with `original_expanded_bytes=130`, the original helper can return an actual size of 120 while reporting 121 and savings of 9. The optimization preserves the original **trial** fallback and its bytes exactly. In accordance with the requested final independent check, such an inconsistent **completed** packet is now rejected rather than emitted. This is an explicit fail-closed integrity guard, not a silent repair of the convergence algorithm. A dedicated regression test records both behaviors. Equivalence claims below concern valid completed outputs; they do not claim that invalid final metadata remains accepted.

## Differential equivalence evidence

`tools/effect_investigation/fixtures/original_packet_reducer.py` freezes the original three reducer functions. An AST/source comparison confirmed that all three match the preserved executable copy verbatim. The oracle imports only unchanged construction helpers. `test_reducer_equivalence.py` executes the original and optimized reducers through the same unchanged builder and requires **complete canonical result byte equality**, not just equal packet lengths.

The preserved baseline captures include expanded/no-reduction packets, exact fit, one-byte-over, minimum rejection and exact minimum, deterministic candidate-cap deferral, mixed established/candidate edges, Unicode/escaping, nested provenance/control-flow/audit data, truncation, source references and repeated failed additions. Additional tests cover unchanged shared inputs, changed/missing/escaping source paths, stale size fields, negative savings/decimal-width boundaries, and rollback graph/step/report integrity.

Examples from baseline and optimized artifact comparisons:

| Fixture | Result in both implementations |
|---|---|
| Expanded and exactly 7,922-byte budget | Two expanded packets, each 7,922 bytes |
| Same packet at 7,921-byte budget | Two progressive packets, each 7,202 bytes |
| Minimum at 9,781-byte budget | Deferred over budget |
| Minimum at exactly 9,782 bytes | One progressive packet, 9,782 bytes |
| Repeated failed graph additions | Whole result equal; at least eight relationship rollbacks checked |

There are **13 saved canonical fixture result comparisons** in `baseline-fixtures-final/` and `optimized-fixtures-final/`. Before adding serialization accounting, the normalization-only snapshot separately passed **17 tests**, the then-current fixture comparisons, and the 8,192-edge full-result byte comparison. That executable snapshot is retained as `normalization_only_investigate.py`.

All **16 optimized benchmark/attribution outputs** matched their corresponding baseline canonical bytes. Independent validation also checked packet limits/report sizes, sorted candidates, node and relationship closure, compact-catalog recovery, candidate/established resolutions, deferred references and source-reference membership. Complete-result equality preserves coverage, uncertainty, structural qualifications and recovery metadata, including fields not individually enumerated by that validator. This is fixed-input structural evidence, not runtime proof of an rclone lifecycle or candidate dispatch.

## Validation

Both requested suites used local dependencies and offline Go settings:

```sh
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH=tools/effect_investigation \
  GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local \
  python3 -m unittest discover -s tools/effect_investigation -p 'test_*.py' -v

GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go test -count=1 ./...
```

- **Python: 40 tests passed**, no failures or skips; final run 2.364 seconds. Includes the analyzer-to-packet integration test and six new reducer tests.
- **Go: passed**, four packages with tests and two packages reporting no test files; 71.702 seconds including supervisor overhead (command package: 69.567 seconds). The non-verbose Go output does not enumerate individual test counts.
- `git diff --check` passed. No existing assertion was weakened.

Diagnostic setup failures are retained separately, not counted as passing measurements: initial oracle-file extraction, an empty-array shell invocation, a summary glob that included log files, and an initial verification assertion confusing the two manifest hash formats. These were corrected before the relevant acceptance checks. No benchmark timeout is counted as a success.

## Before/after measurements

Environment: Python 3.15.0, macOS 27.0.1, arm64. Builder flags and derived input bytes were identical within each pair. These runs omit source evidence, as did the corresponding earlier target-only diagnostics. No `cProfile` was used. Call counts came from separate wrapper-instrumented attribution runs and are not used as timing results.

Every diagnostic/build child had an external **270-second deadline** (below five minutes), a **3.5 GiB sampled physical-footprint guard**, separate stdout/stderr, captured exit status and a unique destination. All required target-only runs completed with exit 0. Timing excludes final output capture/comparison; memory peaks include those diagnostic activities and a short post-run hold. Optimized captures additionally read the baseline for comparison, so memory peaks are not an isolated builder-allocation comparison. They are periodic samples, not kernel maxima. Small timing differences are not performance guarantees.

Three timing repetitions were used for 512, 2,048 and 8,192 edges; full-target cases are single observations. Total build seconds are medians with ranges; loop seconds are medians.

| Edges / budget | Reps each | Baseline build [range] | Optimized build [range] | Packet loop before → after | Speedup |
|---|---:|---:|---:|---:|---:|
| 512 / 48 KB | 3 | 0.355 [0.354–0.356] | 0.114 [0.113–0.114] | 0.348 → 0.107 | 3.12× |
| 2,048 / 48 KB | 3 | 1.459 [1.456–1.464] | 0.452 [0.447–0.452] | 1.437 → 0.430 | 3.23× |
| 8,192 / 48 KB | 3 | 7.187 [6.793–7.226] | 1.821 [1.817–1.843] | 6.746 → 1.735 | 3.95× |
| 36,721 / 48 KB | 1 | 31.587 | 8.917 | 31.224 → 8.552 | 3.54× |
| 36,721 / 1 GB | 1 | 1.218 | 1.156 | 0.626 → 0.567 | 1.05×, single observations |

| Edges / budget | Canonical calls before → after | Cumulative encoded bytes before → after | Recursive normalization calls before → after | Peak sampled MiB before → after |
|---|---:|---:|---:|---:|
| 512 / 48 KB | 2,974 → 997 | 144,121,958 → 49,780,254 | 1,170,903 → 80,777 | 30.6 → 30.8 |
| 2,048 / 48 KB | 12,200 → 4,079 | 594,461,923 → 203,194,307 | 4,779,235 → 245,845 | 52.7 → 52.7 |
| 8,192 / 48 KB | 49,082 → 16,395 | 2,400,799,818 → 816,146,759 | 19,255,807 → 803,897 | 151.8 → 146.4 |
| 36,721 / 48 KB | 220,332 → 73,539 | 10,741,411,665 → 3,641,129,565 | 84,383,003 → 3,163,155 | 513.1 → 553.2 |
| 36,721 / 1 GB | 141 → 140 | 171,135,401 → 148,412,879 | 3,718,836 → 3,718,836 | 635.1 → 613.8 |

Exact output representation was retained, not just its length:

| Edges / budget | Canonical complete result bytes, both | Packet bytes, both | Strategy/status, both |
|---|---:|---:|---|
| 512 / 48 KB | 1,450,215 | 47,340 | progressive; one emitted, none deferred |
| 2,048 / 48 KB | 4,426,794 | 47,389 | progressive; one emitted, none deferred |
| 8,192 / 48 KB | 14,398,701 | 47,682 | progressive; one emitted, none deferred |
| 36,721 / 48 KB | 56,192,716 | 47,488 | progressive; one emitted, none deferred |
| 36,721 / 1 GB | 75,916,122 | 22,722,522 | expanded; one emitted, none deferred |

These compact canonical results differ in formatting from the earlier review's pretty-printed JSON size observations. Full bytes, hashes, ranges, commands, statuses and source identities are retained in `measurement-summary.json`, `baseline/`, `optimized/`, and `reference-validation/`; `artifact-manifest.json` records 556 diagnostic artifact identities. Final preservation checks confirmed the six protected starting paths were unchanged apart from the explicitly added documentation section. No single-candidate speedup is extrapolated to the 100-candidate workload.

## One bounded full canonical attempt

All prerequisite gates passed before launch. The actual optimized CLI entry point ran against the unchanged parent report, matching source snapshot/model/manifest, and the archived settings `--max-candidates 100 --max-packet-bytes 48000`, with only the output destination changed. A `runpy` wrapper enabled a five-second periodic Python stack dump; it did not replace builder functions or add profiling. Command/source identity is recorded in `full-canonical-attempt/command.json`.

- **Incomplete:** supervisor reason `wall_timeout_270s`; wall time **270.963 seconds** including termination handling.
- Child exit **−15 (SIGTERM)**; supervisor exited 124. No retry or higher limit.
- Peak sampled physical footprint **2,254,857,830 bytes (about 2.1 GiB)**; sampled CPU time reached **3:45.18**.
- Stdout was empty. The output directory did not exist, so there is no canonical full-build count, packet-size result or emitted/deferred inventory to validate.

Of 54 periodic stack dumps, 44 showed the builder inside progressive reduction; 40 showed graph-item processing, and 36 included the two source-reference-set comprehensions in `_reduction_report` (lines 1006–1007). These are stack observations, not an exhaustive time profile. They show remaining repeated metadata work during graph reduction on source-rich candidates. They do **not** establish that later unselected candidate materialization or repeated disk/source reads became dominant. No evidence-integrity or byte-limit regression was observed in completed validation, but a stopped full build cannot prove full-workload success.

## Remaining risks and next step

The reducer still performs a canonical packet measurement and reconstructs metadata on each addition/rollback. `_reduction_report` repeatedly rebuilds identical all-source-reference sets from `state['source_items']`; real candidates have far more source evidence than the no-source target benchmarks. Candidate materialization before the cap, repeated source reads, and fingerprint peak memory remain unchanged and outside this implementation.

The smallest justified follow-up is a separately authorized, measured change to compute immutable per-candidate reduction-report totals/reference sets once, retaining the same differential oracle and final size guard. The current evidence does not yet justify changing candidate selection, graph traversal, evidence-store architecture or operation-centered extraction.

Only the reducer, new test/oracle files, the small documentation section and this report were changed for this task. Pre-existing work was preserved, including `.ae/journal.ndjson`, `README.md`, existing test edits and Ralph artifacts. No commits, staging, network access, dependencies, model/provider calls, subagents or original-rclone-checkout changes were made.

`REDUCER_OPTIMIZED; CANONICAL_BUILD_INCOMPLETE`
