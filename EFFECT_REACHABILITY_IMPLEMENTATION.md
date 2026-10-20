# Effect reachability implementation

## Outcome

Reffector now exposes VTA-only effects in ordinary function JSON and adds an opt-in `-contracts` inventory for matching interface calls. It does not convert VTA candidates or method-set-compatible types into established call paths.

Baseline was audit commit `482ff039a8a6488d139ffc3034d48e47a1eab11b`. The checkout started at that commit with only the requested audit report and the two pinned external checkouts untracked. The audit fixture and baseline raw outputs were read from `/tmp/reffector-effect-reachability-audit-20261020T184137Z/`. New raw outputs, commands, binaries, and timing data are in `/tmp/reffector-effect-implementation-20261020T185256Z/`.

## What changed

- `Record.PossibleEffects` (`possible_effects` in function JSON) reports recognized effects reachable only through at least one VTA candidate edge.
  - Existing `effects` is unchanged: `direct: true` means direct; `direct: false` means through established SSA calls.
  - Candidate effects never enter `effects`, never become established reverse edges, and do not clear `uncertain`.
- `-contracts` is an effect-view-only option. For each emitted matching interface-invocation site, `interface_contracts` lists methods from the loaded type universe with:
  - `established`: SSA supports this exact invocation;
  - `candidate_vta`: VTA supports it as possible flow;
  - `contract_compatible`: its type satisfies the invoked interface but no value-flow evidence reaches this invocation.
- Each implementation lead includes symbol, source location, selected status, and recognized effects reachable within its implementation. `open: true` explicitly records that external/unloaded implementations may remain.
- Contract inventory is opt-in, limited to emitted filtered call sites, sorted deterministically, and does not add execution graph edges. It inspects loaded SSA/type packages; it does not widen package loading.
- README, man page, CLI help, and minimal skill document direct/established/possible/contract-compatible/unresolved distinctions and the focused interface-investigation procedure.
- Added `cmd/reffector/contracts_test.go`. It verifies candidate effects stay out of established summaries, VTA and contract-compatible implementation categories remain separate, and external interface dispatch stays open.

The effect-report schema remains `schema_version: 1`: new JSON fields are additive and omitted when unused. Existing plain function-pair text remains unchanged for compatibility; use function JSON to obtain the three-way distinction.

## Fixture results

The independent audit fixture has fs-write, memory-only, rename, and never-used implementations plus arbitrary/known/conditional/field/returned/callback/generic cases.

| Function | Before function JSON | After function JSON | Relationship evidence |
|---|---|---|---|
| `Field` | empty `effects`, `uncertain: true` | `possible_effects: [fs.create, fs.write]`, empty `effects`, uncertain | VTA candidate to fs backend through returned value/field. |
| `Conditional` | empty established effects | possible `fs.create`, `fs.remove`, `fs.rename` | VTA candidate to rename backend; memory backend remains visible only through contracts. |
| `Known` / `Returned` | empty established effects | possible fs-create/fs-write | established call to `api.Invoke`, then VTA candidate fs backend. |
| `Callback` | empty established effects | possible `fs.remove` | VTA callback path. |
| `GenericUse` | established `fs.create`, `fs.write` | empty `possible_effects` | instantiated generic call resolved directly by SSA. |
| `Arbitrary` | empty, uncertain | empty, uncertain | no concrete flow is invented. |

The complete fixture direct inventory remains four recognized concrete sites, including the never-constructed implementation: `os.WriteFile`, callback `os.Remove`, `os.Rename`, and unused `os.Remove`. This is unchanged effect discovery, not a reachability result.

`-contracts -unresolved -reason interface_dispatch -operation '.Save' ./...` reports all four compatible implementations at each matching `Backend.Save` site. The passed fs and conditional rename/memory implementations are `candidate_vta` only at the calls whose VTA flow supports them; the unused implementation remains `contract_compatible`. No contract-compatible entry becomes a reverse execution edge.

Repeated complete-fixture contract output is byte-identical after deterministic aggregation (`6489bb36aed2a4fc6481e82e1eb96773e5445573bcf007dc797e6d156c832102` for both final runs). Earlier non-determinism in merged selected status was repaired by aggregating duplicate SSA variants deterministically.

## Scope limitations

The available type universe contains loaded package types, recorded as `available_packages`. This prevents the report from implying global completeness. It still depends on chosen package scope: callers-only fixture selection reports contract-compatible loaded leads but has no VTA candidate edge, because VTA intentionally runs only selected source functions. Selecting implementation packages makes those functions VTA roots and exposes candidate paths. Contract inventory does not silently load an entire repository.

This remains a real limitation: a loaded dependency can be contract-compatible but have neither a selected-root VTA candidate nor a normal direct-site inventory. The report labels it as a contract lead rather than an execution path.

## Restic acceptance

Pinned restic HEAD `dd09b6bd01af220b1d5d95c0d4d7d472e05e05c4` remained clean.

```sh
reffector -contracts -unresolved -reason interface_dispatch -operation '.Save' \
  -function saveUnpacked -max-depth 4 -max-sites 10 -max-edges 100 -json \
  ./internal/repository/... ./internal/backend/local/...
```

The result has one unresolved `(*Repository).saveUnpacked` call to `backend.(Backend).Save`, `open: true`, and six available implementations. `(*Local).Save` is `candidate_vta` with filesystem/resource effects including `fs.rename`; cache, dryrun, limiter, memory, and retry backends are `contract_compatible` leads. The ordinary summary now says:

```json
{"function":"(*Repository).saveUnpacked","effects":[],"possible_effects":["fs.create","fs.read","fs.remove","fs.rename","fs.write","resource.acquire","resource.release"],"uncertain":true}
```

So one focused query now provides the unresolved boundary, VTA supported implementation, candidate effects, contract alternatives, and scope caveat. It does not claim Local executes in every run.

The audit baseline non-contract query was 0.53 s / 869 MB peak (single sample). The final restic contract query was 1.10 s / 884 MB and 15.8 KB JSON. The additional method-set inventory is observable work; these are environment samples, not a benchmark. Function JSON after the implementation was 0.51 s / 852 MB for the same selected scope.

## False results and remaining gaps

- No tested direct effect false negative/positive changed.
- VTA candidate effects remain possible, not established; runtime conditions, reflection, plugins, and external implementations stay unresolved.
- Contract inventory can list wrappers/backends that satisfy a method but are not reachable at the site; that is intentional and explicitly labelled.
- The inventory only covers types available to Go loading/SSA under the selected build. `open: true` means it cannot establish absence of other implementations.
- Candidate effects through unselected implementation roots remain scope-sensitive. Addressing that would require a separately measured VTA root-policy decision, not a presentation change.
- Ordering, ownership, concurrency, and success/failure of effects are still outside this change.

## Validation

```sh
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go test ./...
git diff --check
git status --short
```

No commit or push was performed.
