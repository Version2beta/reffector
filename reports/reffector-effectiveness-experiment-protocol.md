# Reffector Investigative-Value Pair — Preregistration

**Status:** BLOCKED before freeze. No investigator session has started.  
**Prepared:** 2026-10-30T02:02:50Z  
**Target suitability:** Passed the preregistered depth gate.

## Question and scope

Question: in this single matched case, does adding Reffector’s effect-directed workflow to ordinary read/Bash tools produce more independently confirmed, distinct, consequential correctness defects in the included Go implementation?

This is one paired observation, not a general benchmark. A result cannot establish causal or general effectiveness. No finding is expected, and there is no minimum finding count.

## Target and suitability gate

- Repository: `/Users/rob/Development/perhap`
- Pinned commit: `77b51dd01ec61ea99fb923afd833242b4f9d0bde` (`main`; clean at selection)
- The read-only suitability scout found distinct operational responsibilities in the Go target: domain configuration/typed event-handler registration (`sdk/go/domain.go`), JetStream service/event routing/replay (`sdk/go/service.go`), per-entity supervisor lifecycle/state (`sdk/go/supervisor.go`), and event/model-update records (`sdk/go/event.go`). These span consequential state, persistence/replay, routing, and lifecycle behavior rather than only pure computation or wrappers.
- The integration tests use an embedded NATS server bound to loopback, an ephemeral port, and temporary storage. No target production service is required. Investigators may run only self-contained/offline tests and local ephemeral services after inspecting their setup.

## Frozen source scope and exclusions

Two independent source snapshots were created from the pinned Git tree, then copied into isolated run directories. They have byte-identical contents and each matches the same 14-file manifest. Manifest SHA-256: `22e39ab721ad89a2f27904a43f338caaed9ac624bf2dd883bbab041c2426c477`. Exclusion/scope path manifest SHA-256: `0246f69c944b19ef6954717df533ab9fa0b8011dd8401728feb16b06d133a6b3`.

Preparation bundle: `/private/tmp/reffector-effectiveness-prep.ev0T0R`; isolated run root: `/private/tmp/reffector-effectiveness-run.0hivQf`; captured event/results root: `/private/tmp/reffector-effectiveness-captures.bZZSL2`.

Included paths:

- `README.md`, `go.mod`, `go.sum`, `spec/specification.md`
- `demo/cart/cart.go`, `demo/cart/main.go`
- `sdk/go/domain.go`, `sdk/go/domain_test.go`, `sdk/go/event.go`, `sdk/go/service.go`, `sdk/go/service_integration_test.go`, `sdk/go/service_test.go`, `sdk/go/supervisor.go`, `sdk/go/supervisor_test.go`

This is the Go implementation scope. Non-Go frontend sources and other repository files are outside the study.

The following 23 tracked complexity-analysis artifacts were excluded by path only; their contents were not opened. They are absent from both snapshots and inaccessible to the investigators:

- `.complexity/CALIBRATION.md`
- `.complexity/RUN_RULES.md`
- `.complexity/baselines/perhap-whole-repo-v1.json`
- `.complexity/examples/insufficient-coverage.json`
- `.complexity/examples/strong-structure.json`
- `.complexity/examples/temporal-risk.json`
- `.complexity/packet.schema.json`
- `.complexity/protocol.json`
- `.complexity/results/baselines/perhap-whole-repo-v1.json`
- `.complexity/results/baselines/perhap-whole-repo-v1.md`
- `.complexity/results/examples/insufficient-coverage.json`
- `.complexity/results/examples/insufficient-coverage.md`
- `.complexity/results/examples/strong-structure.json`
- `.complexity/results/examples/strong-structure.md`
- `.complexity/results/examples/temporal-risk.json`
- `.complexity/results/examples/temporal-risk.md`
- `SCORING.md`
- `complexity-report-cognitive-load.md`
- `complexity-report-refactor.md`
- `complexity-report.md`
- `scripts/complexity-freeze-calibration.mjs`
- `scripts/complexity-score.mjs`
- `scripts/complexity-score.test.mjs`

A tracked-path name scan found no additional investigation notes, solutions, bug/defect reports, findings, or handoff files. Snapshots were made with `git archive`, so untracked/ignored files were not copied. Each investigator’s sandbox denies access to `/Users/rob/Development`, all of `/private/tmp` except that investigator’s own run directory, and the other investigator’s run directory. Thus the excluded files, other checkouts, prior reports, and cross-session artifacts are not available in the sessions.

## Reffector build gate

The single authorized corrected build was run from a clean temporary checkout pinned to `95d92da26ba09697c94f9a15496798f77643e446`:

`GOPROXY=off GOSUMDB=off GOFLAGS=-mod=readonly go build -o /tmp/reffector-experiment ./cmd/reffector`

Observed build evidence:

- `go version -m`: Go `1.27.2`, `GOOS=darwin`, `GOARCH=arm64`
- `vcs.revision=95d92da26ba09697c94f9a15496798f77643e446`
- `vcs.modified=false`
- Binary SHA-256: `0a84c090fff23fc83c8ae6af6e73c8e80de2f0e9452f452b8177a5ddf1cf369e`

The earlier binary with `vcs.modified=true` is not used.

## Matched conditions and isolation

- Order is fixed: ordinary-tools control first, then Reffector treatment. Treatment starts only after control artifacts are frozen and verified; no control result content is inspected or supplied to treatment.
- Both sessions use Pi CLI `1.4.0` (binary SHA-256 `da0c6a49fe94aadc9765cc7afc6ba6b17e553eed0247d0385c160bc188f5cfbb`), the active model `openai-codex/gpt-6-luna`, thinking level `high`, tools `read,bash`, JSON mode, and fresh, arm-specific session directories. Context files, extensions, skills, and prompt templates are disabled. Exact invocation settings are `--mode json --model openai-codex/gpt-6-luna --thinking high --no-context-files --no-extensions --no-skills --no-prompt-templates --tools read,bash --session-dir <arm-runtime>/session --print <frozen-arm-prompt>`.
- The common prompt is byte-identical in both arms. Control prompt SHA-256: `d4dff0e86d89245f50e0a2c7d9957956abbcd905be28a6894cc9bf2549a0ac2f`. Treatment adds only the effect-directed workflow and analyzer access; treatment prompt SHA-256: `565770c2a7be6ce15208d4026f06c393b90d60c845ecfe2aa72b6877909cbfc3`.
- Control cannot read the Reffector binary. Treatment's sole additional file exception is `/private/tmp/reffector-experiment`, whose hash is pinned above. Both snapshots are read-only; no fixes, commits, pushes, or remote mutations are authorized.
- Each process has an externally enforced 1,800-second wall-clock deadline. On expiry the runner sends `SIGTERM`, waits 10 seconds, then sends `SIGKILL` to the process group. A timed-out control still permits the preregistered treatment run; a non-timeout process error stops the pair without retry. If Pi naturally exits before the cap, that is recorded as a Pi process exit—not a supervisor stop or active-time measurement.
- Environment is allowlisted; `PI_SKIP_VERSION_CHECK=1` prevents a separate Pi release check, and `GOENV=off`, `GOPROXY=off`, `GOSUMDB=off`, `GOFLAGS=-mod=readonly`, `GOTOOLCHAIN=local`, and `GOWORK=off` prevent user Go configuration and dependency fetching. Go build/temp caches are arm-local. The shared Go module cache is read-only. No API-key or provider credential environment variables are passed to Pi/Bash.
- The tested macOS `sandbox-exec` policy begins with `(deny default)`. It explicitly permits required process/toolchain use, loopback test listeners, provider outbound networking, system/runtime file reads, the read-only Go module cache, Pi installation/auth/settings paths used internally, and each arm's own run directory. File-content reads under `/Users` and `/private/tmp` are otherwise denied; writes are denied except to that arm's runtime directory, with both source snapshots and the Go module cache write-blocked. `/Volumes`, `/Network`, and macOS shared temp-cache paths are denied. Treatment alone can read/execute the analyzer binary; control cannot. System file reads are allowed for process startup, so this is not a complete filesystem allowlist. Pi's installed package and auth/settings files are accessible to the Pi process; prompts prohibit using Read/Bash to inspect them, so that tool-level restriction is not independently enforced. No credential environment variables are passed, and tool events will be audited for out-of-scope reads. Profile SHA-256: control `fcbe38dd9637a05a5f05fdcbf84273c298a1cb64823d6f86114b449ef7bfa1c7`; treatment `15ff2a96af111e154e7976c61192367c482904cfa4f6de4ba3e7255a414e7c79`.
- The Pi provider requires network access. The sandbox therefore does not independently firewall Bash from outbound network while allowing the provider. The prompts prohibit network, upstream, issue, and pull-request searches; dependency fetching is disabled; raw tool events will be audited for external-search or remote-access commands. Any such use will be reported as a protocol deviation, not silently ignored.

## Investigation instructions

Both investigators receive the same task, scope, and JSON response schema. They must survey at least three distinct operational areas or flows, record source paths and line ranges, distinguish retrieval from review, and report candidates with triggers, consequences, contract basis, alternatives, tests/probes and exact exit status. They must not modify the source, search upstream, access production, or claim novelty. The specification is a working draft; planned or explicitly unimplemented behavior is not a defect in this Go-only study.

The treatment-only instruction requires the initial query:

`GOPROXY=off GOSUMDB=off GOFLAGS=-mod=readonly GOTOOLCHAIN=local GOWORK=off /private/tmp/reffector-experiment -view effects -stats -max-sites 64 -max-edges 128 -max-depth 4 ./...`

Treatment may make focused follow-up queries within the same caps. Analyzer output is a navigation aid, not proof. It must preserve established/candidate/unresolved distinctions and verify claims in source. Analyzer site/edge coverage is reported separately from investigator source-read events; neither is described as line coverage or substantive review. A requested read range does not establish returned extent. Active time is unavailable; only external wall duration is measured.

Runner implementation SHA-256: `ede9ef95dfad3194b150fc5bcffe26f7d2e24fa46febaaa119dd868ef690df8e`. Before either session, it verifies the exact preregistered source-manifest SHA-256, exclusion/scope-manifest SHA-256, prompt/profile hashes, and every source-file hash in each arm.

## Frozen evaluation rubric

A fresh independent evaluator receives a randomized, arm-blinded docket of all candidate claims only after both runs and their artifacts are frozen. It receives a third byte-identical copy of the pinned source scope, not the excluded artifacts, investigator prompts, raw transcripts, analyzer output, or arm mapping. Candidate cards preserve the claimed behavior, trigger, consequence, contract/source references, and tests while removing tool/arm attribution. Residual clues in candidate wording may make blinding imperfect; that will be disclosed. The evaluator uses a fresh Pi CLI `1.4.0` session, the same `openai-codex/gpt-6-luna` model at `high` reasoning, `read,bash` only, no context files/extensions/skills/templates, the same source/cache restrictions, and a 1,800-second outer deadline. Its fixed prompt SHA-256 is `d5e6941ead1dd46833a2f47fd5a46c71eab1c3b8b5ff7f269684e505674bd53d`; only the blinded docket content is prepared after the runs. After freeze, the coordinator assigns neutral randomized IDs/order and records the arm mapping outside the evaluator workspace; investigator self-classifications, tool queries, and method attribution are omitted. This is an independent context, not an independent model family or human judgment.

Each claim is classified exactly once as:

- **Confirmed:** current included Go code has a concrete trigger and behavior with a material correctness consequence, supported by a source trace and conflict with an applicable explicit contract or clearly necessary invariant. A focused reproduction/test is preferred; if impractical, the source proof must be independently checkable. Runtime confirmation and static confirmation are labeled separately.
- **Refuted:** source, contract, or a safe test disproves the claim.
- **Unresolved:** evidence or contract is insufficient; uncertainty is not counted as a defect.
- **Out of scope:** claim depends on excluded code or a planned/unimplemented feature.

The evaluator also assigns mechanism groups to claims that describe the same underlying fault. After unblinding, repeated confirmed claims are collapsed within each arm only; if the same confirmed mechanism appears in both arms, each arm receives credit once. Primary outcome: distinct confirmed mechanisms per arm and their paired difference (`treatment - control`). Secondary outcomes: candidate classifications, precision (`confirmed / (confirmed + refuted)` after within-arm duplicate collapse; unavailable if denominator is zero), safe test outcomes, elapsed wall time, Reffector query/coverage and truncation, and source-read retrieval counts. Source-read counts measure successful retrieval only, not code review. No active-time, causal, statistical-significance, novelty, or general-effectiveness claim will be made from this pair. The evaluator may not change this rubric.

## Prelaunch hold

Do not launch either investigator under the current profiles. Independent review found that Pi's auth/settings files are readable by the investigator process, outbound networking is unrestricted, and broad system reads plus path coverage are not sufficient to establish the requested full access boundary. The profiles do block source writes and tested cross-arm/original-checkout reads, but prompt prohibitions do not enforce the remaining credential and network restrictions. Resume only after an approved tool/network isolation design prevents investigator Read/Bash access to credentials and unauthorized external services, followed by updated hash checks and independent review. No preregistration sidecar has been written.

## Freeze, stop, and reporting rules

1. Before either investigator starts, finalize this document, both prompts, both sandbox profiles, source manifests, and the runner; hash them and save a protocol SHA-256 sidecar.
2. Run control, freeze/hash its raw Pi event stream, stderr, session record, metadata, and investigator-created artifacts, then allow treatment to start only after hash verification. Do not inspect control candidate content at this gate.
3. Run treatment under the same deadline and freeze its artifacts. Do not evaluate either result until both are frozen.
4. Import the genuine Pi events after the runs; preserve run metadata and report active-duration status as unavailable. Audit the recorded tool commands for prohibited external access.
5. Conduct blind independent evaluation under the frozen rubric; only then unblind and compare. Any failed build, snapshot mismatch, session/process error, timeout, prohibited access, or incomplete import is reported precisely. No retries, score changes, or protocol edits after first investigator start.
6. Report the one-pair result with all limitations. Do not claim general or causal effectiveness. Do not edit the target or Reffector source, and do not commit or push experiment artifacts.
