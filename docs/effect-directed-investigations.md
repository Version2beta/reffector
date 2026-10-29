# Effect-directed investigations

## Current increment

The local tool `tools/effect_investigation/investigate.py` consumes Reffector EffectReport v1, optional audit JSONL, optional argument-provenance/control-flow projections, and optional source-manifest/program-model JSON. It deterministically identifies review leads, reduces reverse call graphs, and emits bounded JSON and Markdown packets. The opt-in Go SSA projections are separately versioned structural evidence; they do not change effect recognition or classification. The core packet-building command makes no provider call.

```sh
python3 tools/effect_investigation/investigate.py effects.json \
  --audit audit.jsonl \
  --source-root /pinned/source-checkout \
  --manifest source-manifest.json \
  --program-model program-model.json \
  --provenance argument-provenance.json \
  --control-flow control-flow.json \
  --out /tmp/effect-review \
  --max-candidates 100 --max-packet-bytes 48000
```

The packet budget remains 48,000 bytes by default and is the UTF-8 byte length of canonical compact model-facing JSON (sorted keys, no trailing newline), including reduction reports. If a selected candidate's expanded packet exceeds the cap, a deterministic progressive reducer builds a minimum evidence packet and attempts prioritized graph, source, provenance, control-flow, and audit detail additions. A candidate is byte-deferred only when that minimum packet also exceeds the cap. Candidate-ID selection remains size-independent: the first `max_candidates` IDs stay selected, and smaller later candidates never replace them. Inventory/deferred records retain stable graph, source, provenance, and control-flow recovery references, statuses, and truncation. Full coverage and projections remain in the result-level evidence store. Reduced packets state their omissions and carry a structural-only path caveat. The byte/4 token estimate is explicitly heuristic, not a provider tokenizer count.

Candidate IDs and categories are deterministic investigation leads, not a ranking, finding, defect, or severity. Graph-frontier caller nodes are structural candidates, not proof of public entry points, external reachability, or security boundaries. Package-level distinctions are package evidence only, not architectural components unless an explicit mapping is supplied. Audit alternatives remain unresolved/possible evidence and never become established edges. Missing policy, authority checks, cancellation, ordering, or cleanup are questions to investigate, not claims established by static analysis.

Source excerpts come only from program-model declaration ranges and operation locations whose relative paths are listed in the supplied manifest and resolve beneath the supplied source root. File hashes are checked before reading excerpts. Outside-manifest/dependency-cache paths and path escapes are explicitly unavailable; they are not probed.

## Coverage and external investigation events

The analyzer's `-stats` JSON reports selected roots, source module path/version, host OS/architecture, process `GOOS`/`GOARCH` values (host fallback), test-selection mode, successful package-load status, function/site counts, configured graph caps, and analysis duration. Effect coverage separates selected-source/dependency sites, unresolved sites, emitted established/candidate edges, and truncation. The whole-source function denominator is explicitly `unavailable` with a reason, so no source-coverage percentage is emitted. Git/source revision, custom build tags, and per-package load failures are unavailable; a package-load failure is returned as an error without partial stats. See `reports/investigation-observability-implementation.md` for field meanings and limits.

The packet builder accepts an optional external event ledger with `--events events.jsonl`. It validates an allowlisted JSONL schema for Reffector invocations, direct source reads with canonical repository-relative paths/ranges, source searches, candidate status, reproduction attempts, operation groups, and run termination. Termination records distinguish wall duration from measured or unavailable active duration. Recorded counts describe only supplied entries; absent input is `not_provided`, and ledger completeness is not independently verified. The builder does not launch or observe an agent, shell, tests, or active time. Its own packet-building source excerpts remain separate from investigator-read events. Invocation arguments are retained verbatim, so event producers must omit or redact sensitive values; environment variables are not collected.

## Purpose and investigation examples

The tool helps a reviewer ask a narrow behavior question with traceable code and call evidence. Examples of questions for later investigation include:

- **Correctness:** Which distinct callers can reach a file operation, and do their argument or state conditions differ?
- **Reliability:** Does an async/deferred path change operation ordering, error propagation, cancellation, cleanup, or recovery behavior?
- **Security:** Which candidate caller paths lead to a sensitive operation, where are authority and input-validation decisions made, and what evidence establishes actual reachability? A graph frontier or exported function alone does not answer this.
- **State/obligations:** Which shared resource or operation paths converge, and what lifecycle obligations are visible in their source?

These are investigation prompts, not inferred bugs. Argument provenance is an opt-in Go projection (`-argument-provenance`) integrated by Python `--provenance`; control flow is separately opt-in (`-control-flow`, consumed by Python `--control-flow`). Neither establishes execution, feasibility, trust, authorization, aliasing, path predicates, or runtime behavior.

## Optional isolated model investigation (implemented, separate opt-in)

`tools/effect_investigation/model_investigation.py` provides an optional adapter boundary; it is not invoked by the deterministic `investigate.py` command above. Its version-2 response schema accepts a free-form hypothesis plus typed claims that cite an operation site, a direct graph relationship, or a canonical source reference. A claim must match the operation site and API. Relationship claims resolve to the packet's canonical edge ID, endpoints, and recorded resolution; candidate classifications remain candidate and are never reported as executed. Source claims resolve to a stable source-reference ID whose path, whole-file digest, and exact line range are re-read under the supplied `repository_root`. Cached excerpts are replaced before the model call.

These checks bind evidence identity only. They do not establish that source text entails a free-form hypothesis, that a static edge is feasible, or that any operation ran. Model-selected `supporting`/`contrary`/`context` roles are retained as opinions, not verification outcomes. The system assigns `identity-bound` to checked citations, `unresolved` when no hypothesis is supplied, `unsupported` when a hypothesis has no identity-bound supporting citation, and `plausible-unverified` when it has at least one. A model-selected supporting role is still only an opinion, not semantic proof. `runtime_status` is always `not-confirmed`; `contradicted`, `supported-static`, and `runtime-confirmed` cannot be asserted by the model.

`run_and_persist` writes each invocation to a fresh, caller-selected directory; an existing directory is refused rather than reused. It writes the request and either a qualified result or error, retains captured provider events, and writes the manifest last with request/result or error hashes. Readers must require a successful manifest and matching hashes. `aggregate_findings` consumes these persisted invocation directories, revalidates typed claims against each request (and requires the repository root for source claims), and groups only exact duplicate hypotheses with the same canonical site and complete verified claim set. Groups are provisional evidence-equivalence deduplications, not shared-resource or common-cause conclusions; individual hypotheses, statuses, and citation identities remain in each group. Opaque resource/obligation labels and older unverified finding records are not accepted.

This module's schema and aggregation input are intentionally stricter than earlier artifacts. Historical M5/M7/M8 files remain unchanged and are not retroactively reclassified. No model/provider is called by the deterministic packet-building command, and this optional boundary does not run a benchmark.

## Principles and implemented pipeline

Principles: deterministic core; provider/harness independence; stable references; canonical evidence preservation; explicit uncertainty; manifest-bounded source access; bounded output with recoverable deferrals; effect-centered questions rather than architecture claims; no severity or defect assertions.

Implemented pipeline:
1. Load the structured EffectReport, optional audit JSONL, and optional versioned Go projections.
2. Validate schema, required object/array shapes, stable identities, and graph references.
3. Join audit sites by stable site ID and functions by `(package,function)`; account for unmatched audit scope.
4. Derive candidate categories from graph frontiers, unresolved relationships, async edges, branching/convergence, and distinct package-level callers.
5. Deduplicate canonical node/edge records in catalogs; packets carry readable summaries and stable references.
6. Resolve operation owner and relevant caller declarations from supplied program-model records.
7. Read only manifest-listed, hash-checked source beneath the supplied root; preserve unavailable/deferred source references.
8. Produce a category/operation-specific semantic question and evidence-linked packet.
9. Enforce the exact packet byte budget with deterministic progressive reduction; defer only when the minimum packet cannot fit, while preserving candidate-count deferrals.
10. Emit inventory, packets, catalogs, full projections, audit coverage, coverage/truncation, graph-reduction metrics, per-packet reduction reports, and heuristic size estimates.

## Relationship to architectural characterization

Architectural characterization describes components and structural relationships. Effect-directed investigation starts from concrete operation/effect sites and follows reverse call evidence to frame a semantic question. Architecture can offer useful context, but package boundaries are not automatically components and this tool does not depend on Archchar internals or its benchmark workflow.

## Evaluation

Validate small deterministic fixtures for IDs, category correctness, candidate stability, audit distinction, relationship recovery, path containment, source hashes, byte enforcement, and coverage/truncation. On pinned real reports, measure:

- evidence support and evidence coverage, including source/audit match and deferred evidence;
- useful findings/questions judged by independent reviewers;
- unsupported claims, especially mistaken reachability, authority, or defect assertions;
- reviewer effort, elapsed time, and context consumed;
- reduction and packet byte bounds, not only candidate counts.

A future controlled ordinary-agent comparison should use the same repository, task, evidence budget, and time limit, with blinded review where practical. Compare useful supported findings and unsupported claims, effort/time/context, and coverage. The deterministic packet-building command does not generate model answers; optional isolated investigations are separate and are not a benchmark comparison.

## Current bounded changes and measured baseline

Location serialization recursively normalizes `file`/`path` location fields against the supplied root and manifest. An emitted repository-relative path must be beneath the root, manifest-listed, present, and hash-matched. Paths failing verification become null with an explicit unavailable status/reason. Semantic strings are not rewritten. Without root/manifest, model-facing location paths are unavailable rather than raw machine paths. Local source lookup retains the raw input only while resolving excerpts; dependency caches are not read.

`max_candidates` selects the first candidate IDs in deterministic sorted order before size evaluation. An oversized selected packet is progressively reduced; only an over-budget minimum remains byte-deferred. A later smaller ID never replaces a selected one. This is deterministic selection, not relevance ranking. The per-packet limit remains exact canonical compact UTF-8 JSON bytes.

A fresh pre-provenance characterization used the pinned rclone revision `81e7372f6fbedfa80592e9a564593d63a8dc32f3`; its JSON/Markdown remain outside the repository in a temporary output directory. Of 401 candidates, direct packet remeasurement found 231 above 48,000 bytes and 170 within the limit. Under the old emit-as-you-fit policy, 100 were emitted, 231 byte-deferred, and 70 count-deferred; size-independent first-100 ID selection instead emitted 47, deferred 53 over budget, and deferred 301 by count. Exact component measurements total 27,513,235 packet bytes; repeated report coverage costs 313,983 bytes. The characterization verified 6,039 pinned source excerpt hashes with no mismatch; no absolute source-location paths appeared in the model-facing packet artifacts. Expanded path values were absent from the EffectReport and therefore were not measured. These are pre-provenance statistics and do not predict the integrated packet sizes. Structural correlations are descriptive, not causal; component accounting includes canonical key/value bytes and object framing. The generated JSON/Markdown preserve per-candidate details.

## Argument provenance (implemented)

Go emits a standalone `reffector.argument-provenance.v1` JSON object (`schema_version: 1`) with stable `value:`, `origin:`, `call:`, and operation `site:`/`operation:` IDs. Python validates schema/IDs, duplicate and dangling value/edge/call references, role references, resolutions, and EffectReport `site_ref` joins; a joined report site must list the same API through `api`, `apis`, or `operations[].api`. Rows without an available site join remain explicit with their limitation/resolution instead of being silently promoted. It joins by report site where available; without a site reference, a row is summarized only when both API and exact source location match the operation. Unjoined rows remain in the full result but are not attributed to a packet. Full provenance is retained once at result level; packets traverse backward from role values through only their relevant origin subgraph and summarize role refs, node labels/boundaries, edges/resolutions, calls, and explicit site/truncation limits. Candidate/unresolved relations stay explicitly distinct from `established`; notably `candidate_vta`, `candidate_function_value`, `unresolved_dynamic`, `unresolved_mutable_receiver`, and unresolved Cmd receiver evidence are not promoted. A provenance call row records the statically modeled caller/callee and arguments/results at that local source call; its `callsite_ref` is a local source-call identity and is not necessarily an EffectReport site. Established actual-argument links and VTA/function-value candidates retain their own resolutions, rather than merging candidates into established caller evidence.

The projection supports constants, parameters and statically grounded caller actuals, copies/conversions, composition, Phi, extracts/call results, direct `append` operands, and known local returns. Caller-to-parameter actuals are context-insensitive: different callsites can converge at one formal, so the edges remain static leads rather than path-sensitive execution proof. Mutable `os/exec.Cmd` path/args are not read or inferred; the receiver is an unresolved mutable-object boundary. For slices, direct append operands are retained, but slice-literal backing-store elements and heap stores remain unresolved and slice length/capacity are not presented as argv contents. Struct-field loads and mutable/external state, heap, map, global, and closure values may also remain explicit boundaries. Interface dispatch limitations are reported honestly: concrete interface candidates are not emitted as argument-provenance candidate evidence where the analyzer has no such relation. No static value or parameter is trusted or untrusted. This is structural provenance, not taint, trust, authorization, alias, path-predicate, or runtime analysis.

Go bounds default to depth 8, 2,000 value nodes, 500 calls, 32 alternatives, and 1 MiB compact JSON (`-argument-max-depth`, `-argument-max-nodes`, `-argument-max-calls`, `-argument-max-alternatives`, `-argument-max-json-bytes`). Python defaults to 100 selected candidates and 48,000 canonical packet bytes (`--max-candidates`, `--max-packet-bytes`). Python recursively normalizes serialized `file`/`path` location fields against a supplied source root/manifest; unavailable/outside/ambiguous paths become null with status/reason, and semantic labels (including strings that resemble paths) are unchanged. Call-graph provenance describes relationships leading toward an operation; argument provenance describes structural derivation of an operation's argument. Neither establishes runtime reachability.

## Gate audit and packet evidence

The provenance/packet validation gate was reproduced against pinned rclone SHA `81e7372f6fbedfa80592e9a564593d63a8dc32f3`. Of 477 EffectReport operation sites, 12 are eligible for the current fixed argument-role map; all 12 joined matching APIs (10 established, 2 unresolved mutable `os/exec.Cmd` receivers). This is a narrow role subset, not 12/477 join coverage. SSA inspection confirmed that deferred `os.Remove(fileName)` uses `*ssa.UnOp` over `*ssa.FreeVar`, while `Cache.CleanUp` removal dereferences `*ssa.FieldAddr`; these remain explicit capture/field boundaries. The bounded provenance implementation does not trace through captures or fields. The pinned provenance artifact contains exactly 23 call rows, all `established`, and 53 origin edges, all `established`; no candidate call/edge relation was emitted for the 12 role sites. Candidate and dynamic behavior is exercised in focused Go fixtures, not demonstrated by this pinned sample. Do not generalize the pinned counts into a claim that candidate relationships are absent elsewhere.

At the provenance-only gate, before progressive packet reduction, the first 100 sorted candidate IDs were independently measured in memory. Baseline packet total was 6,189,301 bytes; compact-coverage packets without provenance were 6,144,901 bytes (−44,400 bytes), and packets with provenance were 6,151,579 bytes (+6,678 bytes for provenance summaries). All 100 summaries added 51–960 bytes each. At that stage both baseline and provenance-enabled packets emitted 47 and deferred 53 selected candidates over the 48,000-byte cap. The former 100 emitted count used the old emit-as-fits policy; it is not evidence of provenance regression. These historical measurements do not describe the progressive reducer below or support causal claims beyond the measured byte deltas.

A fresh configured run used the same pinned rclone SHA and package patterns, with the preparation settings `-all -max-sites 50000 -max-edges 200000 -max-depth 100`; all 18 manifest-listed source files matched their hashes. EffectReport site, edge, and depth truncation were all false. The control-flow projection covered all 477 effect site/API pairs, with 101 explicit `unavailable_mapping` rows; its statuses were 104 complete, 257 function-truncated, 14 block-visit-truncated, and 1 branch-truncated. The first 100 candidate IDs matched both with and without control-flow and the prior provenance artifact. Both runs emitted 47 packets, deferred 53 over budget and 301 by candidate count; maximum emitted packet size was 47,134 bytes without control-flow and 47,834 with it. All deferred control-flow references resolve, and model-facing control-flow summaries carry the structural-only limitation. This is a bounded structural demonstration, not runtime or path-feasibility evidence. Run inputs, commands, and results are recorded under `/tmp/reffector-pinned-fresh-tE3Lay`.

## Control-flow projection (`reffector.control-flow.v1`)

Go emits a separately versioned, opt-in `reffector.control-flow.v1` projection with `-control-flow`; Python accepts it with `--control-flow`. The Go projection is standalone JSON, so obtain it in a separate CLI invocation from `-argument-provenance`. Its scope is intraprocedural SSA structure. It records entry reachability, dominators, SSA `If` successor ordering, postdominator-based control dependencies, bypass-return evidence, and local defer/RunDefers/panic/return/recover markers. Stable rows identify the canonical EffectReport site, API, function variant, and exact block/instruction ordinal. Candidate resolutions remain candidate; merged sites preserve API alternatives and distinct SSA variants. Missing site nodes or bodies, unavailable API identities, ambiguous instructions, non-exiting CFG regions, and budget truncation are explicit rather than guessed.

The projection's `limitations` state that these are structural facts, not evidence of executed or feasible paths, safety, authorization, or cleanup success. A local `recover` call and `Function.Recover` block are different evidence and remain attached to their respective SSA functions. Likewise, `fs/operations/operations.go:1486` is inside nested closure `rcatSrc$3`, while defer registration is in parent `rcatSrc`; the parent registration is not projected into the closure CFG. No marker verifies resource lifecycle or cleanup outcome.

Go defaults are 64 unique function variants, 128 blocks per function, 4,096 total CFG block visits, 256 exported branches, 2,048 dominator/control/return/marker relationships, postdominator-chain depth 8, and 1 MiB compact JSON (`-control-flow-max-functions`, `-control-flow-max-blocks-per-function`, `-control-flow-max-block-visits`, `-control-flow-max-branches`, `-control-flow-max-relationships`, `-control-flow-max-depth`, and `-control-flow-max-json-bytes`). The projection also reports when the upstream EffectReport site limit omitted sites. A block/function bound suppresses unavailable CFG conclusions rather than inventing them. These control-flow bounds are independent of the EffectReport reverse-call-graph `-max-depth` setting and the Python packet budget; `-control-flow-max-depth` bounds the postdominator-chain walk.

Python validates the projection and joins packet evidence only by canonical EffectReport site ID plus exact API; it never falls back to source location or function name. Packet control-flow summaries carry the projection limitations so the structural-only caveat stays with model-facing evidence. Full control-flow JSON remains at result level. Packets include matched full or compact row evidence; inventory and deferred records retain row IDs, statuses, and projection truncation flags, so a packet whose minimum still exceeds 48,000 bytes or is deferred by the candidate-count cap does not orphan its control-flow evidence. Candidate selection remains the deterministic first IDs before packet-size checks; control-flow bytes can change emit/defer status but not selected candidate IDs.

This projection is not symbolic execution, general interprocedural path sensitivity, lifecycle verification, or a runtime trace. It cannot prove execution, path feasibility, caller context, error semantics, authorization, correctness, process termination, asynchronous completion, panic outcome, or cleanup success. No call is labeled safe or cleanup solely by name. Argument provenance retains its separate limits (depth 8, 2,000 value nodes, 500 calls, 32 alternatives, 1 MiB) and structural-only boundaries for captures/free variables, fields, mutable/external state, and context-insensitive caller-to-parameter links.

## Progressive packet reduction (implemented)

The reducer first builds the expanded packet and records its pre-reduction canonical byte size. When that exceeds the budget, it creates a minimum packet containing the operation identity/effect and source reference or unavailable status, investigation question, bounded frontier/target summary, graph cycle/revisit/truncation and unresolved counts, compact audit evidence, bounded argument-provenance and control-flow summaries, and references to the canonical evidence store. The packet is explicit that this is incomplete structural context, not a runtime-feasible path or a safety conclusion.

Optional evidence is then attempted in deterministic priority order: the operation-local source excerpt; expanded argument-origin details; expanded control-flow details; graph relationships (frontier-to-target structure first, then candidate/unresolved relationships, async/package boundaries, branch/convergence relationships, and remaining established call chains); independently selected source declarations (operation owner, frontier callers, graph boundaries, then other declarations); and full audit records. Established same-package linear call sequences are represented as compact segments with stable segment and relationship references. Branching/convergence vertices remain explicit segment anchors; candidate, unresolved, async, synthetic, and package-crossing edges remain direct relationships. Each addition is measured using the exact canonical UTF-8 packet serialization; an addition that does not fit is rolled back, and later independently selected evidence may still fit. Large segment headers can retain connectivity while pointing to deferred member relationship IDs in the canonical store.

Every emitted packet includes a `reduction_report` with original expanded bytes, final serialized bytes, net bytes saved, applied steps, included/deferred graph node and relationship references, source references and excerpts, audit/provenance/control-flow detail counts, analysis truncation, and recovery pointers. Graph connectivity and explicit relationship-reference completeness are reported separately. The report itself counts against the 48,000-byte cap. Existing result keys and packet status values remain available: `deferred_over_budget` now means that even the minimum packet cannot fit; `deferred_max_candidates` remains the unchanged 100-candidate-count deferral. The result-level `candidates`, `node_catalog`, `relationship_catalog`, source references, audit records, and full projections retain retrieval paths for omitted packet evidence. Selected candidates additionally include `compact_graph_catalog`; packet references resolve by candidate ID and stable source/node/relationship/projection IDs. This is additive retrieval metadata, not a second analysis store.

### Reducer work reuse

The minimum packet is normalized before optional additions. Graph trials introduce no `file`/`path` fields, so they reuse that normalized evidence; additions containing source, provenance, control-flow, or audit locations still run the full path verifier. Rollbacks retain their existing graph/report refresh behavior. Each refresh measures the canonical packet once, then accounts exactly for the decimal widths of the two changing size-report integers rather than serializing the whole packet again for each convergence pass. Completed expanded, minimum, and progressive packets receive an independent canonical serialization check; a size/report mismatch raises an error. Only the immutable complete source-reference ID universe is reused within a progressive invocation; selected reference/excerpt sets and all other report metadata are recomputed on each refresh. The snapshot is never shared across candidates or invocations. No source-read cache is used.

### Pinned before/after demonstration

The comparison reused the saved EffectReport, audit, argument-provenance, control-flow, program-model, and manifest artifacts from pinned rclone revision `81e7372f6fbedfa80592e9a564593d63a8dc32f3`. The matched four-package scope was `vfs/vfscache`, `vfs/vfscache/downloaders`, `vfs/vfscache/writeback`, and `vfs/vfscommon`, selected by `./vfs/vfscache/... ./vfs/vfscommon`; analysis settings were `-all -max-sites 50000 -max-edges 200000 -max-depth 100`. The source checkout and manifest were verified in the earlier pinned run. This packet comparison reused those local outputs; it did not rerun Go analysis or call a model/provider.

| Measure | Before reduction | After reduction |
| --- | ---: | ---: |
| Candidates / EffectReport sites | 401 / 477 | 401 / 477 |
| First 100 candidate IDs | same set/order | same set/order |
| First-100 selected status | 47 emitted, 53 byte-deferred | 100 emitted, 0 byte-deferred |
| Candidate-count deferrals | 301 | 301 |
| First-100 expanded input bytes | 6,281,987 | — |
| Emitted packet bytes | 1,006,123 across 47 packets | 3,648,958 across 100 packets |
| Largest selected expanded packet | 174,036 | — |
| Largest emitted packet | 47,834 | 47,973 |
| Smallest emitted packet | — | 9,355 |
| Packet strategies | — | 44 expanded, 56 progressive |

The 6,281,987-byte comparison sums the old expanded packet size for all first 100 candidates, including the 53 that were not emitted before. The 3,648,958-byte result includes final reduction reports for all 100 now-emitted packets; it is 41.91% below that expanded total. Total emitted bytes increased from 1,006,123 because the new run emits 53 additional packets. All 100 final sizes matched their reports and stayed at or below 48,000 bytes. The candidate IDs and expanded-size measurements matched the saved baseline; all graph relationships remained recoverable from the packet or compact graph catalog and relationship catalog. All 595 emitted source excerpts were rechecked against pinned file contents and manifest hashes, and no checkout-root path appeared in the model-facing result. Full provenance/control-flow inventory references resolved to the retained projections. These measurements establish packet-size enforcement and evidence retrieval on this scope only; they do not measure finding quality, runtime behavior, path feasibility, or safety. Before/after artifacts are under `/tmp/reffector-pinned-fresh-tE3Lay/investigator-cf` and `/tmp/reffector-packet-reduction.iJN5Ox`.

## Roadmap

1. Effect-centered call-chain reduction and packets (implemented).
2. Path-normalized evidence and deterministic size-independent candidate selection (implemented).
3. Bounded Go SSA argument provenance and packet integration (implemented).
4. Bounded structural control-flow projection and packet integration (implemented, opt-in).
5. Optional isolated model investigation with typed evidence binding (implemented; semantic/runtime verification remains future work).
6. Conservative persisted evidence-equivalent duplicate aggregation (implemented; shared-resource and common-cause inference remains future work).
7. Controlled comparison (future).
