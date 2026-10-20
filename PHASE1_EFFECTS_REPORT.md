# Phase 1 — effect-centered investigation

## Baseline and scope

Worked on the local checkout at `/Users/rob/Development/reffector`, not public source.
Baseline revision: `5553a1d13011bf638148661f6a23bf561c3eef5a`, **dirty**.
Toolchain: `go version go1.27.2 darwin/arm64`; module requires Go 1.26+.
No repository-local `AGENTS.md` was found; global agent instructions applied.

Baseline evidence is retained locally in `/tmp/reffector-phase1.wAEnae/` (`E`
below): status, tracked diff/stat, source SHA-256 manifest, help, executable and
uncached test log. Manifest SHA-256:
`62e9a3a9b03af939579be3cf8317a729e9b478b8867e9057bb637c91b514afc4`.
This identifies the dirty starting source, not just HEAD. Evidence files are
transient local artifacts, not committed report dependencies.

Initial `git status --short`:

```text
 M README.md
 M cmd/reffector/integration_test.go
 M cmd/reffector/main.go
 M internal/analyzer/analyzer.go
 M internal/effects/effects.go
 M internal/output/output.go
 M man/man1/reffector.1
 M skills/reffector/SKILL.md
?? .DS_Store
?? HANDOFF.md
?? cmd/reffector/coverage_test.go
?? cmd/reffector/effectvariants_test.go
?? cmd/reffector/effectview_test.go
?? cmd/reffector/emptyeffect_test.go
?? cmd/reffector/generics_test.go
?? cmd/reffector/inventory_test.go
?? cmd/reffector/provenance_test.go
?? cmd/reffector/uncertainty_test.go
?? experiments/
?? internal/analyzer/details.go
?? internal/analyzer/effectview.go
?? internal/analyzer/selection_test.go
?? internal/analyzer/uncertainty.go
?? internal/output/effects.go
?? internal/output/effects_test.go
?? reffector-self.json
```

The initial tracked diff was 8 files, 843 insertions, 225 deletions (untracked
work is additional). Existing work was preserved with incremental edits.

Confirmed from README, man page, bundled skill, CLI, analyzer, output code and
regression tests: default effect groups → canonical source sites → reverse
callers; exact API/site selectors; substring effect/operation/root filters;
exact function roots; unresolved-only selection; bounded variant-specific
graphs; uncertainty reasons; selected/dependency coverage and full inventory.
`-json` already emits schema-version-1 effect objects; `-view functions -json`
and `-details` already provide compatibility projections. A substantial advisory
`find` skill already exists; no new skill or report type was needed.
Build: `go build -o "$E/reffector-before" ./cmd/reffector`; actual options were
checked with that executable's `-help`. Baseline `go test ./...` passed cached;
`GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go test -count=1 ./...` then passed
all packages (CLI suite 19.536s).

## Findings and bounded changes

| Candidate | Disposition and evidence |
|---|---|
| Concrete `Close` | **Already supported.** `ConcreteClose` reports `os.(*File).Close`, `resource.release`, no uncertainty. |
| Standard-interface `Close` | **Reproduced and fixed.** Open `io.Closer`/`io.ReadWriteCloser` calls previously appeared only in unresolved leads; bound calls were labeled `unrecognized_stdlib`. Added the declaring `io.(Closer).Close` contract summary and bound-call dispatch reason. Embedded contracts work without matching arbitrary methods by name. |
| Application persistence boundary | **Already represented; classification intentionally bounded.** Exact interface API, source identity and reverse callers already existed. `Persistence.Save` is still unresolved with empty effects; no inferred filesystem/database operation or edge to any loaded implementation. Real Restic contracts were inspected in `internal/backend/backend.go`. |
| Rename discovery/classification | **Reproduced and fixed.** Existing `os.Rename`/`os.(*Root).Rename` had `fs.create` + `fs.remove`, so `-effect rename` returned zero. Added distinct `fs.rename`, retaining old entry-change categories for compatibility. These are not separate remove/create calls, nor proof of a new destination. No alias to `fs.write`, and no method-name heuristic. |
| Significant unresolved calls buried by incidental calls | **Fixed selection, not significance inference.** New effect-view `-reason` filters existing uncertainty-reason substrings. `-unresolved -reason interface_dispatch` excludes opaque stdlib logging and lock calls; exact API/operation filters then select the desired contract. Remaining interface calls are not automatically consequential. Unfiltered `-unresolved` remains available. |
| Site/caller navigation, shared versus independent operations | **Already supported and regression-tested.** Two callers share `Persist`'s single Save site; `Independent` has its own site. No second graph engine or output schema was added. Documentation now explains `-unresolved -site ID` and how to combine reason/API selectors. |
| Coordination metadata | **Intentionally deferred.** The fixture's close location exposes its immediately surrounding lock/deferred unlock. Caddy's `streaming.go:599` similarly points into the lock/deferred-unlock region at lines 586–587; its site query exposes two immediate callers. Source context is adequate for the next experiment. Lock-region metadata would require extra control-flow/lifetime semantics without an observed navigation need. No lock/goroutine/barrier inventory or ordering claims were added. |

Implementation is limited to the summary table, existing site selection, CLI
flag wiring, documentation and focused fixtures/tests. Source-based IDs and
reverse traversal are unchanged. An unresolved standard contract supplies no
concrete target. A locally proven concrete target and its interface contract
remain two pieces of evidence at **one** source site, not duplicate sites/edges.

Compatibility: JSON structures and version remain unchanged. Function summaries
can now contain direct/inherited `resource.release` from standard close contracts
and `fs.rename`; exhaustive effect-enum consumers must accept the new identifier.
Previously unresolved-only close sites move into recognized groups but keep
`interface_dispatch` uncertainty when open. IDs do not change from classification.
`-reason` is case-sensitive, conjunctive with other filters, and effect-view only
(including rejection with `-details`). Coverage and operation metadata are not
rewritten by it. README, man page and bundled skill document these semantics.

## Representative before/after evidence

For the commands below, `E=/tmp/reffector-phase1.wAEnae`,
`B="$E/reffector-before"`, `A="$E/reffector"`; both binaries were built from
this local checkout, before/after the incremental implementation respectively.
All analyzer/Go commands used `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local`;
third-party commands also used `GOWORK=off`.

In `testdata/phase1`, with identical source:

```sh
"$B" -effect resource.release -max-depth 0 .
"$A" -effect resource.release -max-depth 0 .
# before: resource.release (1 sites), ConcreteClose only
# after:  resource.release (6 sites), including:
# io.(Closer).Close ... in example.com/effectinvestigation.StreamClose
# [call; interface_summary] possible: resource.release
# unresolved: interface_dispatch

"$B" -effect rename -max-depth 0 .
"$A" -effect rename -max-depth 0 .
# before: 0 match filters (not evidence of no effects)
# after: fs.rename (2 sites): os.Rename, os.(*Root).Rename

"$A" -unresolved -reason interface_dispatch \
  -api 'example.com/effectinvestigation.(Persistence).Save' -max-depth 1 .
# two unresolved-only sites; effects remain empty
# CallerA -> Persist and CallerB -> Persist share one origin
# Independent has a separate origin
```

The original three focused tests failed for the intended missing behavior
(one release site, no rename matches, undefined `-reason`); all four final
investigation tests pass. Baseline fixture unresolved sites: 12; after: 7
(five standard close calls moved to recognized operations, not certainty).
Reason selection narrows those seven to four open-interface leads; exact Save
selection narrows to two. Tests check direct call positions, exact API identity,
uncertainty, inherited summaries, known concrete dispatch, independent/shared
sites, negative queries, deterministic bytes and site/depth/edge bounds.

### Untouched pinned experiments

Both trees initially had clean status and the requested revisions:

- Caddy `0e392c89baa50f558bc24440482c5d57fff4499f`.
- Restic `dd09b6bd01af220b1d5d95c0d4d7d472e05e05c4`.

Offline `go list -mod=readonly -deps` passed on each requested scope, so no
missing-cache blocker prevented the smoke queries. No application was executed.

From `experiments/caddy`:

```sh
# Run identically with "$B" and "$A":
"$A" -function '=(*tunnelTracker).closeAttachedConnections' \
  -effect resource.release -max-depth 1 -json ./modules/caddyhttp/...
# before: sites_matching_filters=0, sites=[]
# after (selected fields):
# api="io.(Closer).Close", function="(*tunnelTracker).closeAttachedConnections"
# location=streaming.go:599:31, effects=["resource.release"]
# recognition="interface_summary", uncertain=true
# uncertainty_reasons=["interface_dispatch"]

"$A" -site site:50eafe75c05ca22a747fb415b2a68e9d472ad22f4970ea30e7e165ab12ef4031 \
  -max-depth 1 -max-edges 20 -json ./modules/caddyhttp/...
# cleanupAttachedConnections -> closeAttachedConnections at streaming.go:620:37
# cleanupAttachedConnections$1 -> closeAttachedConnections at streaming.go:629:37
# truncated: depth=true, edges=false, sites=false
```

The exact-root query intentionally excludes outside callers; the site query
restores them. The source closure is a distinct caller, not proof of when it
executes. Repeating the exact-root JSON command produced byte-identical output.
An additional `-api 'io.(Closer).Close' -max-depth 1 -max-sites 10 -max-edges 20`
query emitted 10 sites/10 edges with `sites=true, depth=true` truncation: it is
explicitly incomplete, and does not replace the targeted site query.

From `experiments/restic` (same scope on every command):

```sh
"$B" -api os.Rename -max-depth 1 -json \
  ./internal/repository/... ./internal/backend/local/...
# (*Local).Save, local.go:180:20, effects=["fs.create","fs.remove"]
"$A" -effect rename -max-depth 1 -json \
  ./internal/repository/... ./internal/backend/local/...
# same site:9a8cbcbfba99c06fc0ee41fa4c19d4b53c29a2e634c66fca321f022a51a01a3c
# effects=["fs.create","fs.remove","fs.rename"], uncertain=false

"$A" -unresolved -reason interface_dispatch \
  -api 'github.com/restic/restic/internal/backend.(Backend).Save' \
  -max-depth 1 -max-sites 10 -max-edges 20 -json \
  ./internal/repository/... ./internal/backend/local/...
# 4 distinct Save sites; 3 reverse caller edges; truncated.depth=true
# repository.go:526:17: effects=[], uncertain=true, reason=interface_dispatch
```

Running the last command with the baseline binary, omitting only the unsupported
`-reason`, gives identical sites, reverse graphs and edges. Thus persistence
identity/navigation was already supported; the new filter is selection evidence,
not new resolution. No edge was invented from `Backend.Save` to `Local.Save`.

## Validation and limits

All commands exited 0 unless explicitly described as reproduction failures:

- Baseline: `go test ./...` and offline `go test -count=1 ./...`.
- Focused: offline `go test ./cmd/reffector -run '^TestInvestigation' -count=1`.
- Final: offline `go test -count=1 ./...` (CLI 24.383s), `go build ./...`,
  `go vet ./...`; CLI tests build and execute the actual analyzer binary.
- Offline dependency checks and all smoke commands above passed; stderr empty.
- Fixture tests compare repeated output and enforce bounded queries, negative
  filters and source preservation. Baseline/final unresolved fixture IDs match.
- Before/after smoke source snapshots matched: Caddy 737 files, Restic 1395
  files (excluding `.git`); both experiment trees remain clean at pinned HEADs.

This is reporting evidence, not defect detection, successful cleanup, storage
implementation resolution, cross-function coordination proof, or complete
behavioral coverage. Unknown resource kinds and arbitrary application contracts
remain unknown. Existing loading failures still return errors; empty matches and
bounded/truncated output are not absence proofs. No dependencies were added,
third-party files changed, commits made, remotes mutated or history rewritten.

## Final working-tree checks

The following cumulative Git output includes the substantial pre-existing user
work; `git diff --stat` does not count untracked files. Phase additions include
`cmd/reffector/investigation_test.go`, `testdata/phase1/`, and this report.

```text
$ git diff --stat
 README.md                         | 244 ++++++++++++++++++++++--
 cmd/reffector/integration_test.go |  17 +-
 cmd/reffector/main.go             |  67 ++++++-
 internal/analyzer/analyzer.go     | 291 +++++++++++++++++++++++++----
 internal/effects/effects.go       |  66 ++++++-
 internal/output/output.go         |   7 +
 man/man1/reffector.1              | 384 ++++++++++++++++++++++++--------------
 skills/reffector/SKILL.md         |  64 ++++++-
 8 files changed, 915 insertions(+), 225 deletions(-)

$ git diff --check
(no output; exit 0)

$ git status --short
 M README.md
 M cmd/reffector/integration_test.go
 M cmd/reffector/main.go
 M internal/analyzer/analyzer.go
 M internal/effects/effects.go
 M internal/output/output.go
 M man/man1/reffector.1
 M skills/reffector/SKILL.md
?? .DS_Store
?? HANDOFF.md
?? PHASE1_EFFECTS_REPORT.md
?? cmd/reffector/coverage_test.go
?? cmd/reffector/effectvariants_test.go
?? cmd/reffector/effectview_test.go
?? cmd/reffector/emptyeffect_test.go
?? cmd/reffector/generics_test.go
?? cmd/reffector/inventory_test.go
?? cmd/reffector/investigation_test.go
?? cmd/reffector/provenance_test.go
?? cmd/reffector/uncertainty_test.go
?? experiments/
?? internal/analyzer/details.go
?? internal/analyzer/effectview.go
?? internal/analyzer/selection_test.go
?? internal/analyzer/uncertainty.go
?? internal/output/effects.go
?? internal/output/effects_test.go
?? reffector-self.json
?? testdata/phase1/
```
