# Pre-discovery-pivot historical/WIP checkpoint

## Standing and baseline

Supervisor accepted the **65-path file partition and evidence-index content** and authorized the local commits/tag.
This records supervisor acceptance, not a separate user or independent-review content verdict.
The four implementation/tooling commits below are complete; this record belongs to the final documentation commit.
Annotated local tag `checkpoint/reffector-pre-discovery-pivot-2026-10-28` will be created immediately after that commit.
The annotated tag is the authoritative pointer to the containing final checkpoint commit.
No self-referential containing-commit SHA is recorded here; actual Git receipts are kept outside the repository.

Preparation baseline: `main` at `d766ee95bde0041549cedc1bdbc9a1a6d3a0ef29`.
Locally cached `origin/main` resolves to that same SHA; ahead/behind **0/0**, with no fetch/network check.
`e1e8088` is historical `pre-ralph`; baseline HEAD already contained Go effects/contracts, uncertainty,
function-value/callback provenance, argument provenance, structural CFG, their Go fixtures/tests,
archchar tooling and the original deterministic investigation builder/progressive reducer.
There was no pending Go/module change; the optional model and later reducer/metadata work is captured below.

## Local commits and preserved tooling

Actual preceding commits, in order:
1. `9a046e41f9d9f32b2a521f1f65f231dbe6541e32` — `Test analyzer-to-packet uncertainty boundaries` (2 paths).
2. `dea0d5fd0de6fc457263619cb869c87481f0d3a2` — `Preserve experimental typed investigation validation and persistence` (2 paths).
3. `eee7d691722a047574719480181ff042f26a9bd3` — `Reuse packet reducer work with exact-size differential checks` (6 paths).
4. `5332b60bd6b95d27ebb4f3b6f5010fe197533c62` — `Preserve historical Ralph tooling and campaign inputs` (5 paths).
Final documentation subject: `Document bounded investigation work and pre-pivot checkpoint evidence` (50 paths).
Exact allowlists, original-file hashes and decisions are in `reports/pre-discovery-pivot-evidence-manifest.json`.
That file is a **frozen pre-commit preparation snapshot**, preserved at its accepted bytes, not a live status report:
its planned commits, empty-index and tag-absent (`tag_exists: false`) fields describe capture time, not post-commit status.
New-file references remain path-only, avoiding self/record hash circularity; the index is not the external corpus.

The historical tooling commit preserves unchanged `ralph.py`, `test_ralph.py`, `TASK.md`, `state.md`, `REPORT.md`.
These are source/tests and required historical inputs/handoffs, not a new loop execution.
Their previous completion claims are explicitly superseded by the post-Ralph technical/correctness reviews,
later scalability reports and the limitations below; runner tests were **not executed** in Phase 1.

## Known limitations, not checkpoint fixes

The typed model module is **experimental**: identity binding is not semantic entailment or runtime proof.
The saved offline probe rejects an actual expanded builder packet because its graph endpoint objects
conflict with the wrapper's string-ID validation; progressive compact edges use string IDs.
No fix was made. Legacy M5/M7/M8 artifacts remain historical and incompatible with schema 2.
M5 has three failed calls and one historically accepted result; M8 is one null pair with a shared omitted
source line and unequal total context, not an efficacy demonstration. No new provider calls occurred.
The canonical full 100-candidate packet build remains **incomplete**: both optimized historical attempts
record `wall_timeout_270s`, child exit −15, and absent output directories; no full build was rerun.
Single-target equivalence/timing does not prove full-workload scalability or runtime lifecycle behavior.
The relevant rclone RC dispatch edge remains a candidate; no daemon leak measurement is claimed.
Provider capture-before-limit and whole-file source-hashing resource limits also remain documented.

## Offline validation standing

Earlier Phase 1 tested the unchanged sources with Python 3.15.0 / Go go1.27.2 darwin/arm64.
Checkpoint preparation/execution rechecked saved summaries/counts; it did **not** rerun tests/probes.
Go and Go-backed Python used `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local`; Python used `-B`/no bytecode.
Each test child had a 270-second guard and sampled 3.5-GiB direct-child limit, not aggregate process memory.
- Investigation discovery: **44 tests**, exit 0; `python3 -B -m unittest discover -s tools/effect_investigation -p 'test_*.py' -v`.
- Root Go: **96 top-level / 238 including-subtest pass events**, exit 0; `go test -count=1 -json ./...`.
- Archchar discovery: **40 tests**, exit 0; `python3 -B -m unittest discover -s tools/archchar/tests -p 'test_*.py' -v`.
- Nested `tools/archchar/go-structure`: **2 tests**, exit 0; `go test -count=1 -json ./...`.
No test failures/skips were recorded; two root packages have no tests. Root Go excludes the nested module.
Earlier tracked-only `git diff --check` passed; staged imported-oracle checking separately warned:
`tools/effect_investigation/fixtures/original_packet_reducer.py:117: new blank line at EOF.`
Supervisor approved preserving its exact reviewed bytes as a whitespace exception; no check/hook/config was bypassed.
No new production/test edits, Ralph execution, model calls, external checkout edits or Phase 2/network research occurred.

## Evidence provenance and exclusions

Artifacts: `/tmp/reffector-pre-discovery-pivot-checkpoint/`; preparation in `preparation-v2/`, execution in `execution-884de0a3/`.
The evidence index preserves complete bounded inventories/manifests, both full-build
command/supervisor records, validation summaries/counts/probe and six excluded-stream pointers/hashes.
It qualifies the original 156-file pre-draft snapshot and distinguishes inventory-only from fresh hashes.
All 156 original pending/ignored files remain byte-identical; only this record changed for commit finalization.
Large external blobs are not embedded; **historical measurements cannot be fully replayed from Git alone**.
Parent report: `/tmp/rclone-average-loop-preflight/dependency-cache-retry-20261028T205455Z/evidence/analysis-retry/effects.json`.
Its 457,954,303-byte SHA-256 is `a1569ae03c77facbd421225d2524ba3166f84209a8911c66a107f5dc61ff2440` (reverified).
Current builder SHA-256: `f38ea2a5fe0bb344fa9b84a6464515d1ba41cdb684dbe09a6608320338f00f8c` (reverified).
Under that retry root, `evidence/source-model/source-manifest.json` SHA-256:
`b391044af613eea55d0bd6d9b05c1bccaf2e031c1762274099625f52a7fbfe6e`; program-model SHA-256:
`17c27123905aab97168258acfebe69b0b4e9683782c86fbad3718eefceb31ec6` (45,188,579 bytes).
All 991 manifest source hashes matched the disposable pre-fix tree; historical pin is `c1b5756eec76b9f0f7ee4da43fdbcae9cb73a2db`.
Historical roots `/tmp/rclone-average-loop-preflight/{packet-performance-diagnostics,packet-reducer-optimization,packet-metadata-optimization}/`
remain external; the new index preserves their detailed bounded manifests, not just manifest digests.

Excluded unchanged: `.ae/journal.ndjson`, `DONE`, `run_state.json`, `run.log`, `.runner.lock`, `logs/`, archive and bytecode.
Also excluded are these six raw provider streams; only metadata is preserved in Git, never their messages/session values.
Their original relative paths remain; private copies are under artifact-root `copies/<path>`.
| Raw provider stream | Bytes | SHA-256 |
|---|---:|---|
| `reports/m5-iteration25-real-investigation/provider-events.jsonl` | 69027 | `c6aa438157ca916e8b4f486a04d7afaa5edf1c5a77b62ee16cbc90a2ba78e530` |
| `reports/m5-iteration27-real-investigation/provider-events.jsonl` | 81607 | `a126d163730f95bb3c5e557ef58b0c0f1367986b1eff2b20f28cb621a084810a` |
| `reports/m5-iteration29-real-investigation/provider-events.jsonl` | 89848 | `555eaaf6bcaec84c6c5971ec84b82f895eaec50793e23ad9edf7d5264c924272` |
| `reports/m5-iteration30-real-investigation/provider-events.jsonl` | 88838 | `cc78b18543cfed02c97ae68e5cab2d9a716396876980d5ccfb1dfd5909bdeed7` |
| `reports/m8-iteration34-pair/reffector-supported-provider-events.jsonl` | 145631 | `631e9d9ca5f88bcae5c50bb5ea4c9bcbc2cd8e3f0d199ed7f30f1024ec50cf2d` |
| `reports/m8-iteration34-pair/source-only-provider-events.jsonl` | 140891 | `12d83fc59c7ffc97c61fbfe2b144e5505e22a167a2a91562cd4a0c6d295d051e` |
No credential/private-content absence guarantee is implied by heuristic scanning; raw streams remain unsuitable for routine commits.
The manifest contains original paths as provenance, not credentials. `/tmp` storage is not a durable backup.
Five local commits and the annotated tag preserve this historical/WIP checkpoint; no push or Phase 2 is included.
Main-session verification of the Git receipts is required before any next phase.
