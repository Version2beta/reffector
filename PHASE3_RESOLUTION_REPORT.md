# Reffector resolution and serious-finding verification

## Scope and baseline

Current checkout: `5553a1d13011bf638148661f6a23bf561c3eef5a` (`main`), Go `go1.27.2 darwin/arm64`. It already had extensive uncommitted Phase 1/2 user work in analyzer, CLI, effects, output, docs, tests, and experiment artifacts; it was preserved. This work adds only the VTA candidate-resolution changes, `cmd/reffector/candidates_test.go`, and this report on top of that state. Dependencies were already cached; all builds/tests used `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local`, and third-party loads also used `GOFLAGS=-mod=readonly`.

Baseline implementation used local SSA value unwrapping: direct callees, closures, `Phi`, `MakeInterface`, and `ChangeInterface` could establish a target. Parameters, returns, fields, callbacks, and open interface dispatch stayed unresolved. Effect sites behind those boundaries therefore had no reverse path to the caller. The current code already modeled unresolved API contracts and reverse caller paths, but did not model candidate implementation edges.

## A. What changed

- `internal/analyzer/analyzer.go`: runs `golang.org/x/tools/go/callgraph/vta` over selected source functions and closures. It records VTA-resolved dynamic targets only when SSA does not already establish the same target.
- `internal/analyzer/details.go`, `internal/analyzer/effectview.go`, `internal/output/effects.go`: call edges now expose `resolution: "established"` or `"candidate_vta"`; JSON graph edges also carry `candidate`. Candidate edges participate in effect-view path discovery, but **do not** enter normal direct/inherited effect summaries or remove the original unresolved boundary.
- `cmd/reffector/candidates_test.go`: regression fixture covers direct interface values, multiple values, an unreachable implementation, interface fields, a returned interface, callback fields, an unknown external implementation, a hidden `os.Rename`, and a generic interface function. It rejects an invented edge from an interface parameter or unused implementation and asserts candidate versus established labeling.
- `skills/reffector/SKILL.md`: documents candidate VTA edge meaning and required source/runtime follow-up.

This is a compatibility extension to effect-view JSON/text. Existing `CallEdge` consumers should tolerate the added `resolution` field; omitted legacy fields are not changed.

## B. Before/after evidence

Built current source with:

```sh
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go build -o .tmp-phase3/reffector ./cmd/reffector
```

Acceptance query, from `experiments/restic`:

```sh
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly \
  reffector -effect fs.rename -function saveUnpacked -max-depth 5 -max-sites 20 -max-edges 100 -json \
  ./internal/repository/... ./internal/backend/local/...
```

The pre-change installed Phase 2 executable emitted no matching site/path. The new executable emits `(*Local).Save` → `os.Rename` at `internal/backend/local/local.go:180`, with the reverse edge:

```text
(*Repository).saveUnpacked -> (*Local).Save [candidate_vta]
```

The original `backend.(Backend).Save` call remains an `interface_dispatch` uncertainty. This is the intended distinction: VTA says the Local implementation is a possible value-flow target in this selected build, not that every `Backend.Save` call executes it.

On that query, the baseline run reported 0.81 s / 926.6 MB peak footprint; new run 0.59 s / 905.9 MB. These single cached-machine samples do not establish a performance improvement; they show no observed regression in this narrow comparison. VTA is additional analysis work and must be benchmarked on representative larger scopes before any performance claim.

## C. Serious findings

| Hypothesis | Analyzer/source result | Runtime classification |
|---|---|---|
| restic asynchronous producers still work when pack finalization begins | Reffector locates `packerUploader` workers (`packer_uploader.go:24–47`), `savePacker` finalization (`packer_manager.go:221`), and `flushPackUploader` shutdown/wait (`repository.go:659–680`). Source orders `treePM.Flush`, `dataPM.Flush`, queue closure, then `packerWg.Wait`; no violation was established. | Still unverified; no deterministic producer barrier was added. |
| restic upgrade rollback may remove valid config on atomic-replace backend | Reffector now retains `Backend.Save` as unresolved while candidate paths expose concrete persistence operations where VTA can support them. Source at `upgrade_repo.go:75–95` unconditionally removes config before attempting rollback Save. | **Reproduced under induced double-Save failure.** Disposable-worktree test `TestPhase3FailedUpgradeRollbackLeavesNoConfig` proved both Repository and backend config reads fail after upgrade Save and rollback Save are induced to fail. This proves data unavailability under that tested fault sequence, not whether backup-only recovery violates restic’s intended contract. |
| restic temporary pack files lack reliable failure cleanup | `newPacker` creates temp files (`packer_manager.go:204`); `savePacker` has several failure returns before `tmpfile.Close` and no visible cleanup in that function (`:223–278`). | Still unverified; source supports a cleanup question, but no deterministic filesystem/error injection test was built. |
| Caddy response buffering interferes with protocol upgrades | `responseRecorder.Hijack` marks `hijacked`, switches to stream mode, and `WriteResponse` becomes a no-op (`responsewriter.go:298–365`). | **Rejected under tested HTTP/1.1 recorder/hijack conditions.** The disposable worktree ran `TestResponseRecorderSwitchingProtocolsIsHijackAware`; it passed and asserts no buffered body write after 101/hijack. This does not cover every middleware/protocol stack. |
| Caddy blocking connection cleanup executes while shared mutex is held | Source follow-up found `closeIdleRawConns` snapshots connections under `wtServeMu` then closes outside it (`server.go:1644–1667`); `closeWebTransportSessions` follows the same pattern (`:1729–1748`). | Rejected for these inspected paths at source level; not runtime-verified for every cleanup path. |
| Caddy shutdown hooks execute before asynchronous shutdown completes | `configureServer` registers `onShutdownFuncs` through `http.Server.RegisterOnShutdown` (`server.go:1275–1281`), while Caddy separately exposes `RegisterOnStop` as after complete shutdown (`:1293–1300`). | Source-supported behavior, but still unverified as a defect: hook ordering may be intentional API semantics. No contract/test says every on-shutdown hook must wait. |

### Executed disposable tests

```text
$ go test ./internal/repository -run '^(TestPhase3FailedUpgradeRollbackLeavesNoConfig|TestUpgradeRepoV2Failure)$' -count=1 -v
--- PASS: TestPhase3FailedUpgradeRollbackLeavesNoConfig
--- PASS: TestUpgradeRepoV2Failure
PASS

$ go test ./modules/caddyhttp -run '^TestResponseRecorderSwitchingProtocolsIsHijackAware$' -count=1 -v
--- PASS: TestResponseRecorderSwitchingProtocolsIsHijackAware
PASS
```

The worktrees were detached at the pinned revisions, contained only the disposable restic test, and were removed after output/test preservation. Neither pinned experiment checkout was modified.

## D. Remaining limitations

- VTA models possible value flow, not runtime configuration, conditional execution, external implementations, reflection, or unsafe behavior. Candidate edges must never be treated as established execution.
- Mutable state and operation ordering are not represented. For example, the upgrade rollback’s `Remove` → failed `Save` ordering required source and fault injection.
- Goroutine completion is not a call-graph fact. The pack-finalization question requires a completion/barrier model or deterministic runtime harness.
- Lock-held calls, resource ownership, cleanup, and transactional/crash semantics remain source/runtime questions. Reffector sees `Close`, `Remove`, and `Sync`, not lock ownership or durable publication.
- Current effect vocabulary did not itself name Caddy’s hijack/upgrade semantics or restic pack-temp lifetime; source follow-up was required.

## E. Recommendation

The highest-value next change is **annotated concurrency/completion relationships**, beginning narrowly with `go` starts, `WaitGroup`/`errgroup` wait sites, channel close/send/receive, and context cancellation as explicitly uncertain ordering evidence. The unresolved restic producer/finalization question remained important after VTA because it is not a dispatch problem. Do not infer race, deadlock, or completion; report only supported synchronization relationships and retain uncertainty.

## Validation and final state

```sh
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go test ./...
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go test ./cmd/reffector -run TestEffectViewVTACandidatesFollowIndirectValues -count=1
git diff --check
```

All passed. `git diff --check` produced no output.

```text
$ git diff --stat
 README.md                         | 264 +++++++++++++++++++++++---
 cmd/reffector/integration_test.go |  19 +-
 cmd/reffector/main.go             |  69 ++++++-
 internal/analyzer/analyzer.go     | 354 ++++++++++++++++++++++++++++++----
 internal/effects/effects.go       |  66 ++++++-
 internal/output/output.go         |   7 +
 man/man1/reffector.1              | 386 ++++++++++++++++++++++++--------------
 skills/reffector/SKILL.md         | 212 +++++++++++----------
 skills/reffector/embed.go         |   2 +-
 9 files changed, 1051 insertions(+), 328 deletions(-)
```

```text
$ git status --short
 M README.md
 M cmd/reffector/integration_test.go
 M cmd/reffector/main.go
 M internal/analyzer/analyzer.go
 M internal/effects/effects.go
 M internal/output/output.go
 M man/man1/reffector.1
 M skills/reffector/SKILL.md
 M skills/reffector/embed.go
?? .DS_Store
?? .tmp-phase3/
?? HANDOFF.md
?? PHASE1_EFFECTS_REPORT.md
?? PHASE2_SKILL_VERIFICATION.md
?? PHASE3_RESOLUTION_REPORT.md
?? cmd/reffector/candidates_test.go
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

This pre-existing dirty checkout prevents an authoritative attribution of the aggregate stat to this task. The new untracked test is `cmd/reffector/candidates_test.go`; no commit was made. The pinned Caddy and restic worktrees were clean at final check.
