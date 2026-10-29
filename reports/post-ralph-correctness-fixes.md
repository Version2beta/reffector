# Reffector — post-Ralph correctness fixes

**Scope:** Bounded corrections to the optional model-investigation boundary, persistence, and finding aggregation. The deterministic analyzer and Go code were not changed. The checkout was already dirty; existing work and historical M5/M6/M7/M8 artifacts were preserved. No rclone source, `.ae/journal.ndjson`, or `README.md` was edited. No model/provider call, benchmark, dependency installation, or subagent run occurred.

## Executive summary

The cited defects reproduced before edits. The model boundary accepted an arbitrary graph `id` as supporting evidence and a filename-only source claim; aggregation grouped findings with the same site/effect despite different obligation or resource IDs; and a failed rerun in a reused directory left a prior `result.json` beside an error manifest.

The optional model workflow now uses typed claims bound to the packet's operation, direct relationship records, or canonical source references. Verification is part of the normal invocation and persisted result. Free-form hypotheses remain explicitly unverified; candidate relationship classifications are copied from evidence, never supplied by the model, and are not runtime confirmation. Aggregation consumes successful, hash-consistent run artifacts and groups only exact evidence-equivalent duplicates. Persistence refuses an existing output directory and writes the manifest last.

## Reproductions before edits

A bounded local probe against the pre-fix implementation observed:

- `candidate_relationship_citation_accepted = ['link:candidate']` for a packet edge marked `candidate: true`, `resolution: 'candidate_vta'`.
- `filename_only_source_claim_accepted = True` with only `source_excerpts: [{'path': 'pkg/f.go'}]` and no authenticated source reference.
- Two findings with the same site/effect but different obligation IDs were grouped; the same held for different resource IDs.
- After success followed by malformed output in the same directory, the manifest said `error` while both stale `result.json` and new `error.json` existed.

These reproduce boundary behavior; they do not establish that any historical model answer used the defects.

## Changes and guarantees

### Typed evidence and qualified claims — `tools/effect_investigation/model_investigation.py`

- The response schema is version 2 and requires exact keys. Each claim has `kind`, `evidence_id`, `site_id`, `api`, and a model-assessed `role`; extra status, resolution, endpoint, path, or runtime fields are rejected.
- Operation claims must cite the packet's canonical site and an API listed for that operation. Relationship claims must cite a direct canonical graph edge in the packet, with valid endpoint node IDs. The accepted record retains the exact edge ID, caller, callee, kind, resolution, candidate flag, and synthetic flag. The verifier sets `execution_established: false`; a model cannot turn `candidate_vta` into `established` by supplying a label.
- Source IDs are recomputed from the exact path, whole-file digest, and inclusive line range. Source claims require a repository root; the invocation replaces cached excerpts with freshly retrieved, digest-checked excerpts. The verified claim retains those exact reference fields and the excerpt hash/text.
- `verify_static_claims` now runs as part of qualification before a result is returned or persisted. Its outcome is `identity-bound`, not semantic proof. System-derived hypothesis states distinguish `unresolved`, `unsupported` (no identity-bound supporting citation), and `plausible-unverified` (at least one such citation). `runtime_status` is always `not-confirmed`. Model-selected `supporting`/`contrary` roles remain opinions; they do not prove entailment or contradiction.

### Evidence-based persisted aggregation

- `aggregate_findings` now accepts invocation artifact directories, not arbitrary records with caller-supplied resource, obligation, effect, status, or evidence labels.
- It accepts only version-2 success manifests, validates request/result hashes and IDs, and recomputes the qualified result from the persisted request. Source claims are rechecked against the supplied repository root. Failed, incomplete, mismatched, or stale artifacts are rejected.
- A group requires the same canonical operation site, exact hypothesis text, and the same complete set of verified typed claims. Each member keeps its own hypothesis, status, and verified citations. Groups are labeled provisional `identical-verified-claim-set` evidence deduplications, not common-resource/common-cause conclusions. Findings that differ in site or authenticated source range remain separate; opaque legacy labels are not interpreted.

### Consistent per-run persistence

- `run_and_persist` prepares and qualifies the same packet that it records. Each run requires a fresh caller-selected directory; attempting to reuse one fails before the adapter is called, preserving the prior run unchanged.
- A successful run has `request.json`, `result.json`, optional raw `provider-events.jsonl`, and a manifest written last with request/result hashes. A failed adapter/schema/claim validation run has an error artifact and error manifest but no result. An incomplete directory without a valid success manifest/hash is not accepted by aggregation.

### Documentation and regression coverage

`docs/effect-directed-investigations.md` describes the optional boundary, schema/status meanings, persistence rules, conservative aggregation, and compatibility boundary. `tools/effect_investigation/test_model_investigation.py` now covers typed citation attacks, wrong site/API/type/edge, fake and stale source evidence, candidate/runtime promotion attempts, opaque resource/obligation labels, persisted adversarial responses, exact-evidence positive grouping, unrelated/different-range non-grouping, artifact hash mismatch, and output-directory reuse. The real-Go analyzer packet-boundary test remains unchanged and passed in the full suite.

## Validation

- Reproduction probes ran before implementation and returned the observations above.
- Focused model-boundary suite: **19 tests passed** after implementation.
- Required Python discovery, including the Go-backed analyzer packet boundary test:
  `PYTHONPATH=tools/effect_investigation python3 -m unittest discover -s tools/effect_investigation -p 'test_*.py' -v` — **34 tests passed**. `PYTHONDONTWRITEBYTECODE=1` was set for the final run to avoid creating new bytecode artifacts.
- Required offline uncached Go suite:
  `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go test -count=1 ./...` — **passed** for `cmd/reffector`, `internal/analyzer`, `internal/output`, and `internal/skillinstall`; `internal/effects` and `skills/reffector` have no test files. Go sources were not edited afterward.
- `git diff --check -- docs/effect-directed-investigations.md` passed; no trailing whitespace was found in the changed Python files.

These suites establish the tested local behavior and Go regression suite only. They do not prove hypothesis truth, runtime behavior, artifact authenticity against an actor who can rewrite both files and manifest, or rclone packet quality.

## Compatibility and remaining risks

- Version-1 model responses (`supporting_evidence` / `contrary_evidence`) are intentionally incompatible with the version-2 exact schema. Source-bearing `invoke_isolated`/`run_and_persist` calls now need `repository_root`; `run_and_persist` requires a fresh output directory; `aggregate_findings` now takes persisted invocation directories. Historical M5/M7/M8 artifacts were not migrated or reclassified.
- Citation binding proves only that a typed reference resolves to the cited packet record. It cannot prove that source text entails a natural-language hypothesis, that a static edge is feasible, or that a call executed. Source-selection completeness remains the responsibility of the investigator packet and may be incomplete.
- Aggregation deliberately does not infer shared resources or obligations and will leave potentially related findings separate unless they are exact evidence-equivalent duplicates. The model boundary has no semantic obligation/resource verifier.
- Manifest hashes detect mismatched artifacts but are not signatures. The local artifact owner can rewrite both a file and its manifest. A process interrupted before the final manifest can leave an incomplete directory; aggregation rejects it, but does not clean it up. Reuse errors occur before a new manifest is written, so callers must observe the raised error and choose a fresh path.
- Existing hardening limits remain: Pi subprocess output is captured before the event-stream size check/sink handling, and source retrieval hashes a whole file before enforcing the excerpt limit. These broader resource-bound changes were not part of this bounded correction.

## Rclone preflight readiness

**Assessment: ready for a bounded, read-only rclone packet preflight only; not ready for a benchmark or model-efficacy claim.** The preflight was not run. Before any later model call, inspect the pinned local packet for the target operation, canonical relationship resolution, source-reference identity/range, coverage, and truncation; stop if the required evidence is absent or deferred. No rclone repository or benchmark artifact was changed, and no benchmark or provider call was run here.
