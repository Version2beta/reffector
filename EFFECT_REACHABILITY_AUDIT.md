# Effect discovery and interface reachability audit

## Result

**Direct effect discovery was correct for the independent fixture; reachability is useful but scope-sensitive and incompletely presented.** The strongest demonstrated need is **clearer presentation of relationships Reffector already knows**, followed by **broader interface-contract reachability**. Better value-flow resolution is third: VTA found field/return/callback paths once implementation packages were selected, but intentionally does not make externally supplied interface values concrete. No evidence shows a direct-effect-discovery defect in the tested vocabulary.

This was a read-only experiment against Reffector production source. Raw fixture, commands, JSON, stderr/time output, and test output are under `/tmp/reffector-effect-reachability-audit-20261020T184137Z/`.

## Baseline

| Item | Observation |
|---|---|
| Local/pushed commit | `482ff039a8a6488d139ffc3034d48e47a1eab11b`; `git ls-remote origin refs/heads/main` returned the same SHA. |
| Local changes | Only untracked `experiments/caddy/` and `experiments/restic/`, the pinned external checkouts. Reffector source was unchanged by this audit. |
| Toolchain/build | Go `go1.27.2 darwin/arm64`; `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local`; fixture/restic loads also used `GOFLAGS=-mod=readonly`. |
| Validation | `go test ./...` passed. Built `/tmp/.../reffector` from current source. |
| CLI/report behavior | Effect view reports direct sites and reverse edges. `established` means local SSA supports a call; `candidate_vta` means possible VTA value flow. Candidate edges are deliberately absent from ordinary inherited-effect summaries. |

`PHASE3_RESOLUTION_REPORT.md` and `cmd/reffector/candidates_test.go` correctly describe this boundary. The central audit question is whether that boundary is understandable and sufficient in an ordinary investigation.

## Independent fixture

The external module has packages `api`, `implfs`, `implmem`, `implrename`, `implunused`, and `calls`.

- `api.Backend` declares `Save([]byte) error`; `api.Invoke` accepts an arbitrary `Backend`.
- `implfs.Backend.Save` calls `os.WriteFile`; it also returns a callback that calls `os.Remove`.
- `implmem.(*Backend).Save` changes only memory.
- `implrename.Backend.Save` calls `os.Rename`.
- `implunused.Backend.Save` calls `os.Remove`, but is never constructed/passed.
- `calls` includes arbitrary parameter, known concrete, conditional fs/memory/rename choice, field, returned backend, callback, and unknown function-value boundary cases.

The fixture's source was inspected before analysis. Go compilation independently established that fs, memory, rename, and unused types satisfy `Backend`; therefore all four are **contract-compatible**. Only values constructed/passed in `calls` are **program-level candidates**. An arbitrary parameter additionally permits external implementations not represented in the fixture.

## Direct effect discovery

Expected recognized operations were four, including the never-constructed implementation. All four appeared without requiring a caller. Repeated complete-module JSON output was byte-identical across three runs (SHA-256 `90f0d8a43aae0e6a6118ce5c22c2fc6e1260bb44e23dc14e99ba4481def3c7a2` for each effect-view run).

| Expected owner/API | Actual effect/site/location | Result |
|---|---|---|
| `implfs.(Backend).Save` / `os.WriteFile` | `fs.create`, `fs.write`; `site:fb5e…f1ec`; `implfs/fs.go:4:61` | Found, correct classification. |
| `implfs.Callback$1` / `os.Remove` | `fs.remove`; `site:88ef…9849`; `implfs/fs.go:6:70` | Found. |
| `implrename.(Backend).Save` / `os.Rename` | `fs.create`, `fs.remove`, `fs.rename`; `site:8ae8…36d5`; `implrename/rename.go:4:53` | Found. |
| `implunused.(Backend).Save` / `os.Remove` | `fs.remove`; `site:d582…fd46`; `implunused/unused.go:4:53` | Found despite no construction/caller. |

No tested false negative or classification false positive was found. This result concerns only recognized `os` filesystem APIs; it does not claim recall for unmodeled operations.

## Interface reachability and report consistency

| Call site / implementation | Contract-compatible | Program/value-flow result | Reffector representation |
|---|---|---|---|
| `api.Invoke(b)` | fs, memory, rename, unused, external | fs reaches `Invoke` through `Known`/`Returned`; other known fixture values are not supplied to that parameter | `Invoke` remains `interface_dispatch`; fs reverse edge is `candidate_vta`. |
| `calls.Arbitrary(b)` | fs, memory, rename, unused, external | no concrete fixture value flows to its parameter | unresolved only; no candidate implementation edge. |
| `calls.Conditional` | fs, memory, rename, unused, external | memory and rename are constructed; only rename appears in an effect path | rename edge is `candidate_vta`; no report surface enumerates the no-effect memory candidate. |
| `calls.Field` | same contract set | returned fs value flows through `Holder.B` | fs write path with `candidate_vta`. |
| `calls.Callback` | n/a function value | returned callback flows to invocation | remove path with `candidate_vta`. |
| unused implementation | contract-compatible | never constructed/passed | direct operation remains discoverable, but no candidate reverse path. |

This answers the required acceptance case negatively for an arbitrary interface parameter: an engineer can see `Backend.Save` as an unresolved contract boundary, but cannot ask Reffector to enumerate all contract-compatible implementations or learn from output alone that one may perform filesystem writes. That requires separate source/type search. This is a **missing possibility**, not a reason to invent a concrete execution edge.

Ordinary function summaries for `Known`, `Returned`, `Field`, `Conditional`, and `Callback` all contain no inherited filesystem effect and `uncertain: true`. In contrast, effect view shows candidate paths from `Field` to `os.WriteFile`, from `Conditional` to `os.Rename`, and from `Callback` to `os.Remove`. Thus an engineer could reasonably mistake ordinary summaries for a complete possible-effect inventory unless the candidate-exclusion rule is understood. The views are internally consistent, but the distinction is not prominent enough.

## Scope sensitivity

| Selected pattern | Selected packages | Direct sites emitted | Candidate edges | Key result |
|---|---:|---:|---:|---|
| `./calls` | 1 | 0 | 0 | Dependencies load enough to expose four unresolved interface sites, but implementation effect sites/candidates are absent. |
| `./calls ./api` | 2 | 0 | 0 | Same practical result. |
| caller + all implementation packages | 6 | 4 | 4 | All direct sites and candidate paths appear. |
| `./...` | 6 | 4 | 4 | Same as explicit complete selection. |

This is partly a legitimate root-selection rule: direct site inventory counts selected roots. But it is also a demonstrated information-use limit: implementation packages are loaded dependencies of `calls`, yet VTA is run only over selected source functions and candidate paths are not retained until implementation packages are selected. The output distinguishes selected versus dependency sites only after they are represented; it cannot reveal an unselected implementation's direct operations in this scenario.

## Restic

Pinned restic remained at `dd09b6bd01af220b1d5d95c0d4d7d472e05e05c4`, clean.

```sh
reffector -effect fs.rename -function saveUnpacked -max-depth 5 -max-sites 30 -max-edges 100 -json \
  ./internal/repository/... ./internal/backend/local/...
```

With local backend selected, output contains `(*Local).Save` → `os.Rename` and reverse edge `(*Repository).saveUnpacked → (*Local).Save [candidate_vta]`. `saveUnpacked`'s ordinary function summary remains empty/uncertain. The unresolved view separately identifies its `backend.(Backend).Save` site.

With only `./internal/repository/...` selected, the rename site and candidate edge disappear: 5 selected packages, 21 operation sites (7 selected/14 dependency), but no matching rename site. Adding `./internal/backend/local/...` produces 6 packages and 51 sites (38 selected/13 dependency), including the Local rename. Other `Backend.Save` implementations were not made contract-compatible implementation inventories by Reffector; only operation sites reachable within selected-package analysis are exposed.

Single query time/peak-memory samples were 0.53–0.76 seconds and 0.83–0.88 GB. They are environment samples, not a performance benchmark.

## Scorecard

| Dimension | Result |
|---|---|
| Direct effect recall | 4/4 expected recognized fixture operations found, including unused implementation. |
| Direct effect precision | No tested false positives/misclassifications. |
| Established edges | `Known`/`Returned → api.Invoke` are SSA-established; no unsupported established interface edge observed. |
| Candidate edges | Field, returned-interface, conditional rename, and callback relationships are reported as candidates, not executions. |
| Missing possibilities | Contract-compatible unused/memory implementations and external implementations are not discoverable as a contract set. No-effect candidates are especially invisible. |
| Uncertainty | Unknown interface and function-value boundaries remain explicit. |
| Reporting clarity | Material weakness: candidate effects appear in effect view but not ordinary summaries, with no summary-level indicator of candidate effects. |
| Scope sensitivity | Material weakness: selecting callers only hides already-loaded implementation effects/candidates. |

## Ranked recommendation

1. **Clearer presentation of known relationships.** Add an explicit candidate-effect section/count to function summaries, or clearly mark summaries as established-only and point to candidate effect-view paths. This is the smallest change addressing the demonstrated `Field`/restic ambiguity without weakening precision.
2. **Broader interface-contract reachability.** Add a separate, opt-in contract-compatible implementation inventory based on Go method sets. It must be labeled `contract_compatible`, never merged into SSA-established or VTA-candidate paths. For `Backend.Save`, show fs/memory/rename/unused candidates independently from flow-supported targets and retain external uncertainty.
3. **Better value-flow resolution/scope handling.** Evaluate VTA over loaded dependency functions or offer an explicit analysis-root policy, but only after measuring cost/precision. The caller-only fixture proved a limitation; it did not show that broadening is always affordable or correct.
4. **Effect discovery.** No next change justified by this audit.

If CHA is later adopted, it belongs only in the opt-in contract inventory: `contract_compatible` targets would coexist beside `established` and `candidate_vta`, never become reverse execution paths or inherited effects merely by implementing an interface.
