# reffector

`reffector` starts an investigation with **effect category → distinct direct
operation site → reverse callers**. It statically reports possible effects in Go
without executing functions, initializers or tests, or modifying analyzed source.

```sh
go install github.com/Version2beta/reffector/cmd/reffector@latest
reffector ./...
reffector -effect fs.write -max-depth 2 ./internal
reffector -api os.ReadFile -json ./internal
reffector -view functions -tests -json example.com/project/billing
reffector skill install
man "$PWD/man/man1/reffector.1"
```

It requires Go 1.26+ and the Go toolchain on `PATH`. Package patterns use
`golang.org/x/tools/go/packages`. Local directory arguments (including the default
`.`) recursively select Go subpackages. Use `-exact .` for just the root package.
Explicit patterns (`./...`) and import paths are preserved even with `-exact`.
Dependencies are inspected for propagation; the effect view includes operation
origins in reachable dependencies and labels selected versus dependency nodes.
Function summaries contain selected source functions. `-tests` includes internal
and external test-file functions.

## Optional agent skill

The canonical [`skills/reffector/SKILL.md`](skills/reffector/SKILL.md) is a short,
task-neutral guide to retrieving effect sites, reverse callers and uncertainty.
For one Pi session without installation or settings changes, use
`pi --no-skills --skill /absolute/path/to/reffector/skills/reffector/SKILL.md`,
then `/skill:reffector` with your question and package scope.

Optionally, `reffector skill install` installs the bundled skill at
`~/.agents/skills/reffector/SKILL.md`, a location Pi also discovers. Rebuild the
CLI from the current checkout before installing an updated skill. The skill
does not add analyzer subcommands or require a refactoring workflow; the
analyzer remains fully useful without it. The verification commands below cover
build, test, and static analysis for the release tree.

Repeated installation updates a copy previously installed by reffector. A
pre-existing or locally modified skill is refused rather than overwritten.

## Investigate independent responsibilities

1. Start with `reffector -max-depth 0 ./billing` to discover effect groups and
   independent operation sites. Pick a category, then an exact API or site.
2. Expand reverse callers with `-effect fs.write -max-depth 3` or `-site ID`.
   Multiple callers of one helper share one operation; independent calls remain
   separate even when they use the same API. A multi-effect call has one ID.
3. Read the cited source, controlling policy, authorization, failure paths and
   cleanup. Compare independent paths, not just similar category names.
4. Propose simplification only when justified: identify which old independent
   responsibilities or bypasses would actually disappear, where effects remain,
   and how behavior will be verified. Shared categories alone do **not** justify
   consolidation. Completing analysis is **not** completing a review.

## Output and compatibility

The default is a readable effect report: category entries show each site's API,
source location, owner and canonical ID. Shared site definitions add uncertainty
and reverse caller nodes/edges. The default shows recognized operation sites; `-unresolved` switches
to effect-free unresolved leads so they cannot crowd out effect groups. Local `n1` labels are compact references shared across the report,
not durable query IDs. `caller -> callee at file:line:column` is source evidence.
Shared/revisited nodes and cycles are explicit; no paths are enumerated.

**Deliberate default change:** use `-view functions` for the old sorted
`package.Function effect` pairs, or `-view functions -json` for the unchanged
function-record JSON array. `-json` alone now emits the effect report object.
`-details` retains the earlier function/witness JSON compatibility object; it
implies the function view and JSON, and cannot be combined with `-view effects`.

`-all -json` adds an **unfiltered** `inventory` of selected source functions to
the effect report, even under effect/API/site/package/function filters. Use
`-view functions -all -json` for the compatible array (its summary filters still
apply). Synthetic package init functions, wrappers, lowered iterator helpers and
test executables are excluded; explicit init declarations and source closures
are included. Test variants and generic instantiations merge only in the source
inventory and function summaries, not in traversal. Effect text also displays
`-all` inventory; function-pair text cannot represent empty summaries.

Coverage is always in the effect report; `-stats` also writes coverage JSON to
**stderr**. It identifies selected package paths and retains source-function counts across filters and
counts `effectful`, `direct`, `inherited`, `uncertain`, `unresolved_only`, and
`no_effects_no_uncertainty` functions. Direct/inherited/uncertain may overlap;
the last category is **not purity**. `emitted`/`omitted` describe the compatible
summary projection, not graph size. Site counts distinguish total identified
operations (selected vs dependency), unresolved-only leads, filter matches and
emitted sites after output limits. Counts reflect this analyzer, not all possible
operations in the program. Stats also include host OS/architecture, `GOOS`/`GOARCH`
(environment override or host fallback), selected module paths/versions and
package roots, test-selection mode, successful package-load status, and configured
site/depth/edge caps. Git revision/content identity, custom build tags, and detailed per-package failure
status are unavailable. Package-load failures remain command errors and do not
emit fabricated partial counts. Effect coverage distinguishes emitted established
and candidate edges and includes graph truncation flags. The `-stats` JSON surface
also reports measured analysis duration without making ordinary output
nondeterministic. The whole-source function denominator is explicitly unavailable,
with a reason; root counts are not source-code coverage.

The investigation packet builder accepts optional JSONL observations via
`--events`. Its compatible allowlist includes Reffector invocations, source reads
and searches, candidate/reproduction/operation-group records, termination, tool
invocation/results, test commands, session metadata, worker lifecycle, and
supervisor decisions. See `reports/investigation-observability-implementation.md`
and `tools/effect_investigation/INVESTIGATION_PROTOCOL.md`. For saved Pi 1.4
JSONL, `tools/effect_investigation/pi_event_import.py` writes this ledger plus a
separate coverage report; optional `--run-metadata` and
`--supervisor-decision` inputs are allowlisted and validated. Counts describe
only emitted/supplied ledger rows, not completeness or true zero activity.
The importer preserves references, omits tool output/prompts and Bash arguments,
and records only recognized executable labels; requested read ranges are not
verified returned extents. Its simple Bash parser accepts quoted search
metacharacters and only exact safe Go environment prefixes; pipelines and unknown
shell forms remain unclassified. Packet-builder excerpts remain separate from
investigator reads.

`-view functions -json` emits records with this unchanged schema:

```json
{
  "package": "example.com/app",
  "function": "Load",
  "effects": [{"identifier":"fs.read","direct":true}],
  "uncertain": false
}
```

`effects` is an array (possibly empty). `direct` says that the function itself
calls a summarized operation; otherwise the effect was reached through an
established SSA call. `possible_effects`, when present, lists effects reachable
only through one or more candidate relationships (`candidate_vta` or
`candidate_function_value`); it is never a guaranteed
execution or an inherited `effects` entry. `uncertain` means an unresolved or
opaque operation is reachable. It is deliberately
separate from effects: uncertainty is not an operation and is never printed in
function-pair text output. A function with no recognized effects and
`uncertain: true` remains visible in function JSON; its grouped origins are leads
in the effect view. Neither an empty effect list nor
`uncertain: false` proves a function is pure.

### Bounded dependency VTA

`-dependency-vta-functions N -dependency-vta-edges N` opts into bounded VTA
expansion through loaded dependency bodies. VTA relationships remain
`candidate_vta`; they are never promoted to established calls. With this opt-in,
possible effects are also evaluated per selected source root. For interface
calls, concrete receiver types proven through direct calls and parameter
forwarding can exclude incompatible candidate implementations for that root.
Open parameters and flows Reffector cannot resolve (such as memory loads,
returns, and closure captures) retain all VTA candidates, and the unresolved
interface site remains. This is a bounded refinement, not generally
context-sensitive call resolution; the global candidate relationship remains
available as candidate evidence.

The per-root context-state budget is the sum of the two supplied VTA budgets.
If it is exceeded, context refinement is abandoned for the whole analysis and
conservative global VTA propagation is used instead. Opt-in coverage reports
`context_states`, `context_budget`, `context_truncated`, and
`context_roots_fallback`; the latter counts selected SSA root contexts (including
variants) using the fallback. Dependency function/edge truncation is reported
separately. Without both dependency-VTA flags, existing default behavior is
unchanged.

In function view, `-effect` matches both `effects` and `possible_effects`,
filtering each list without changing its evidence label. `-direct` reports only
recognized operations in the function itself, never inherited or candidate-only
effects; uncertainty still propagates. In effect view, `-direct` restricts
origins to the selected roots, excluding operations reached through helpers
(including VTA candidates). It does not suppress explicitly requested unresolved
leads or filter the unfiltered inventory and contract-implementation summaries.

## Unresolved-operation audit inventory

```sh
reffector -audit-unresolved -stats ./... > unresolved.jsonl
reffector -audit-unresolved -audit-include-known -all -stats ./... > audit.jsonl
```

This opt-in JSON Lines projection uses the **same** SSA graph, summaries and VTA
candidates, without reverse graph construction, per-site whole-program scans or
site limits (including the ordinary 1000-site default). SSA loading/building is
still in memory; only output encoding is one record at a time. Stdout contains
records only; stderr is empty on success unless `-stats` requests coverage JSON.
Errors return nonzero; output-write failures can leave partial JSONL.

Default records have `record_type: "site"`, `schema_version: 1`, `uncertain: true`
and empty established `effects`. `-audit-include-known` also includes sites with
recognized effects **and** uncertainty. A site may have VTA-only `possible_effects`
even in the default export. Sites retain the ordinary canonical IDs, API
alternatives, operation evidence, all reasons and source positions. `primary_reason`
is the lexicographically first reason (or explicit `unknown`), a deterministic
sampling key, not a severity ranking. `unknown_api` explicitly marks an unknown
operation target, even when other alternatives have names; no API is guessed.
`origin`/`selected`, `ssa_nodes` (original node IDs and variants),
`dispatch_kinds` (`interface`, `function_value`, `other`, `unknown`) and
`vta_candidates`/`vta_candidate_count` expose the available evidence. A missing
call instruction, including a declaration boundary, has unknown dispatch.

`possible_effects` unions transitive summaries of this call instruction's
existing VTA candidates accepted by the analyzer's effect propagation, minus
the site's established effects. With bounded dependency VTA, proven incompatible
interface inputs can exclude candidate effects for a selected root; unknown
inputs retain them. It never copies unrelated effects from the owner. Candidates
include identities/variants and `effect_propagation_supported`; raw VTA
candidates excluded from propagation contribute no effects. Candidate summaries
are not proof of execution or exhaustiveness. Bound method wrappers
can have unknown API identity; their SSA identity remains available. Source-
equivalent generic/test sites merge their evidence, not graph nodes or edges.
`kind: "declaration"` denotes a body/summary boundary, **not** an actual call;
existing declaration evidence can be uncertainty-only even when the function's
summary recognizes effects. Synthetic/non-source/invalid-position origins remain
excluded, exactly as in the ordinary site universe. Counts are not all program
operations or all uncertain functions.

`-all` appends `record_type: "function"` records for the full selected source
inventory, including owner `locations`, established `effects`, optional
`possible_effects`, and `uncertain`. This reuses the same analysis for controls:
select only functions with **empty effects, empty possible_effects and false
uncertainty**; this does not prove purity. `-stats` distinguishes site counts,
declaration counts, function-record counts and function coverage. The legacy
`no_effects_no_uncertainty` coverage category alone does not exclude possible
effects. Selected package paths are included in audit coverage.

Only `-audit-include-known`, `-all`, `-tests`, `-exact`, and `-stats` may accompany
`-audit-unresolved`. Explicit view, JSON, graph-limit and filter flags are rejected
rather than silently ignored; filter JSONL after export. `-audit-include-known`
without audit mode is also an error. Existing effect and function views are unchanged.

## Effect JSON, source provenance and queries

```sh
reffector -json -function '=Top' -effect fs.read .
reffector -api os.ReadFile -max-depth 4 -max-edges 100 ./internal
reffector -site 'site:<ID copied from output>' -max-depth 2 ./internal
reffector -unresolved -max-depth 1 ./internal
reffector -unresolved -reason interface_dispatch -operation ').Save' ./internal
reffector -all -json -effect fs.read .
reffector -view functions -all -json .
reffector -details -function '=Top' -max-paths 100 .
```

The versioned effect JSON object (`schema_version: 1`) contains:

- `groups`: effect identifiers and references to canonical `sites` IDs.
- `sites`: one definition per source origin; `unresolved_sites` references only
  effect-free unresolved leads, never confirmed operations.
- `reverse`: one bounded graph per site, with variant-specific `origins`,
  node ID/depth references and edge ID references, `revisit`, `cycle`, `truncated`.
- `nodes`: shared SSA-node definitions (source symbol, variant, selected/dependency,
  direct/inherited uncertainty). Never join nodes merely by displayed symbol.
- `edges`: shared variant-specific caller/callee IDs and exact `source` call
  evidence. Synthetic forwarding has `synthetic: true` and no fabricated source
  location. Source evidence has a separate canonical source-edge ID.
- `coverage`, report-level `truncated`, optional unfiltered `inventory`, and
  opt-in `interface_contracts`.

Use `-contracts` with an interface API/site query to list available method-set
implementations. Each lead is labelled `established`, `candidate_vta`, or
`contract_compatible`. The last means the type satisfies the called interface
but has no value-flow evidence to that invocation. Contract leads are not call
edges; `open: true` means implementations outside the loaded universe may
remain. A practical sequence is: find `interface_dispatch` with `-unresolved
-reason interface_dispatch`, rerun the focused query with `-contracts`, then
inspect candidate effects and source before making an execution claim.
Implementation `effects` are established-call effects; `possible_effects` (when
present, shown as `possible=` in text) require candidate edges. These lists are sorted,
and possible-only effects exclude those established by any aggregated variant.
Contracts at one source site combine the strongest relationship and effect
evidence across generic/test SSA variants; graph nodes and execution edges stay
separate. An open boundary in any variant remains open.

### Architectural characterization harness

Run `python3 -B -m tools.archchar prepare REPO --out DIR [--reffector PATH]`
with repeatable `--pattern` options (default `./...`) to create offline
Reffector/source evidence packets and a Go AST structural index. Token-aware
preparation can use a count-only command to partition oversized packages by
source declarations without dropping evidence. Import saved evidence with
`--effects FILE --audit FILE`. The resumable phases are `investigate`,
`aggregate`, `verify`, and paired `benchmark`; generation is optional and is
blocked unless a separate token-count preflight fits the configured runtime
budget. Preparation and tests make no generation calls. See
`tools/archchar/README.md` for token-count protocols, offline configuration,
retrieval/context limits, evidence scope, artifacts, and benchmark
interpretation. Package and namespace boundaries are starting points, not
inferred architecture.

A site is the same source-based object in both views. It groups recognized
operations at source calls or SSA-native instructions, with stable IDs,
containing package/function, types-based
API identities, exact file/line/column, effect categories, recognition and
uncertainty. `apis` lists exact API identities supported at the site; `api` is
the sole identity, or empty when multiple alternatives exist (or no target is
known). A function-value phi may have multiple APIs at one source site, not
several independent sites. Alternative summarized APIs carry optional
`possible_effects` on the site and each operation, separate from established
`effects`. Effect groups put those site IDs in optional `possible_sites`, not
`sites`; coverage separately counts `possible_operation_sites_identified`.
These fields are absent when unused. Open alternatives retain their unresolved
`function_value` boundary and remain queryable with `-unresolved`. Site IDs do not depend on effects, reasons or filters.
Site `kind` is `call`, `go`, or `defer`; `declaration` denotes a missing-body
or opaque-boundary origin. Go/defer values distinguish invocation forms
without changing source-site identity. Call positions refer to the SSA
instruction (usually the opening parenthesis for calls). Each source call edge
has `kind: "call"`, `"go"`, or `"defer"`, preserving whether the relationship
is an ordinary call, goroutine launch, or deferred invocation. This additive
field leaves edge IDs and schema version unchanged. An established `go` edge
identifies a possible spawned body, not completion; a `defer` edge identifies
work registered for a later function exit, not immediate execution. Selecting a
summarized standard-library function does not fabricate a call to itself. IDs
use package/source identity, not traversal order.

`operations` keeps each API alternative's effects, recognition and uncertainty
reasons together; the site-wide fields are their union. Known semantic interface
contracts and unresolved implementation dispatch can coexist, without inventing
a concrete target. Calls into summarized standard-library APIs are direct sites;
effects reached through application helpers are inherited.

Reverse depth 0 shows origins only; depth 1 adds immediate callers, and larger
depths expand further. All supported relationships among discovered nodes are
retained within limits, including multiple source call sites, diamonds and cycle
edges. Synthetic forwarding costs zero source-call depth at positive depths.
Cycles/revisits are references, not infinite recursive output. Traverse node IDs,
not source-symbol names: merging generic/test variants before traversal could
fabricate paths. Edges between boundary nodes may close cycles without increasing
node depth. Ordering and source-based IDs are deterministic (IDs change if source
locations change; they are not refactoring-stable identifiers).

`-details` is the legacy object with `records`, `stats`, `details.sites`,
`details.call_edges`, `details.uncertainty` and `truncated`. Its `witnesses` retain
one shortest forward path per selected root/site, not all relationships. Use it
for concise forward explanations; use the default reverse graph to compare
alternate callers. Reverse traversal limits live in `reverse[].truncated`.
Both projections describe supported static reachability, not execution.

Uncertainty origins are sites too, including sites with no recognized effects.
Their `uncertainty_reasons` are direct at the containing function:

| Reason | Meaning |
|---|---|
| `interface_dispatch` | An interface target (including a `sync.Locker` method) is unresolved; a semantic summary is not a concrete target |
| `function_value` | Open function value/callback provenance, including unsupported parameters, memory, captures, opaque results, cycles or exhausted bounds |
| `missing_body` | No analyzable Go body (for example assembly or cgo) |
| `unrecognized_stdlib` | Standard-library call has no summary |
| `reflection_opaque` | Reflection or opaque reflective invocation |
| `callback` | Summary boundary may invoke unmodeled hooks, drivers or callbacks |
| `argument_condition` | A summarized operation's effect depends on argument values that static evidence cannot establish |
| `state_condition` | A summarized operation's effect depends on prior state that static evidence cannot establish |
| `unsupported_resolution` | Other unsupported operation, including builtin print/println |

`details.uncertainty` gives selected function identities with independent `direct`
and `inherited` booleans; both can be true. Inherited origins are traced through
site witnesses, not duplicated into every function. Ordinary `uncertain` remains
compatible and describes all reachable uncertainty even with `-direct`. Origins
may be hidden by effect/API filters; use `-unresolved` without those filters to
investigate effect-free origins. Known-effect origins retain their own uncertainty.

Bounds default to `-max-depth 6`, `-max-sites 1000`, and `-max-edges 1000`
**unique emitted reverse edges across sites**. Shared edges consume the budget
once. Report/per-site `truncated.depth` and `truncated.edges`, and report
`truncated.sites`, state omissions; cycle/revisit flags do not mean omitted
relationships. Limits bound explanations, not loading/SSA cost. Compatibility
`-details` uses `-max-paths 1000` instead of `-max-edges`, with its original
`depth`/`paths`/`sites` flags.

Filters are case-sensitive. `-effect` matches category substrings; `-api` selects
an exact package/receiver-qualified API, while explicitly substring-based
`-operation` retains the earlier API search. `-site` selects a canonical site ID;
`-unresolved` switches from recognized operation sites to unresolved sites without
recognized effects. These
exact selectors require the effect view. `-reason` selects uncertainty-reason
substrings in either recognized or unresolved-only sites, without changing site
metadata or coverage. For example, `-unresolved -reason interface_dispatch`
removes opaque standard-library logging calls but does **not** establish that the
remaining calls are consequential. Narrow further with `-api` or `-operation`,
then inspect the cited interface contract. Application-defined persistence calls
retain their exact interface method identity, location and reverse callers; a
method named `Save` alone is not evidence of a filesystem or database effect.
Use `-unresolved` without filters for all effect-free unresolved leads, and the
default view with `-reason interface_dispatch` for recognized open contracts.
To revisit an unresolved-only site by ID, keep `-unresolved -site ID`.
`-package` and `-function` select source
root substrings, then retain reachable helper origins (possibly in dependencies).
For **one function's reachable effects**, prefix its full local name with `=`,
for example `-function '=(*App).Start'`. A substring `Start` also selects source
closures such as `Start$1` as independent roots; an exact root does not assume
that creating a closure invokes it. Reverse callers are limited to that root's
reachable subgraph; remove the root filter to see other callers. `-direct`
restricts origins to roots. Function summary filters retain their earlier meaning;
API/operation/site selectors never rewrite source inventory or site metadata.

## Recognized effects

| Identifier | Meaning |
|---|---|
| `fs.read` | Read filesystem contents or metadata |
| `fs.write` | Modify filesystem contents or metadata |
| `fs.create` | Create filesystem entries |
| `fs.remove` | Remove filesystem entries |
| `fs.rename` | Rename or move a filesystem entry |
| `net.connect` | Initiate a network connection |
| `net.listen` | Listen for or accept connections |
| `net.close` | Close a connection/listener or change its close state (including half-close) |
| `http.response` | Write HTTP response body or status through `http.ResponseWriter` |
| `resource.acquire`, `resource.release` | Acquire owned handles or release resources via recognized APIs |
| `process.exec` | Start an external process |
| `process.control` | Send a signal to or kill a process through `os.Process` |
| `process.signal.register` | Register a channel for signal notifications through `os/signal.Notify` or `NotifyContext` |
| `process.signal.unregister` | Stop signal relaying through `os/signal.Stop` or a proven `NotifyContext` stop function |
| `process.signal.ignore` | Request that `os/signal.Ignore` suppress selected signals |
| `process.signal.reset` | Request that `os/signal.Reset` restore selected signal handlers |
| `process.exit` | Terminate the process with `os.Exit` |
| `context.cancel` | Invoke a proven standard-context cancellation function or `NotifyContext` stop function |
| `env.read`, `env.write` | Read or modify process environment values |
| `db.query`, `db.exec` | Database query or command operations |
| `concurrency.goroutine` | Start a goroutine (`go` statement, recognized `sync.WaitGroup.Go`, or a possible `NotifyContext` watcher) |
| `channel.send`, `channel.receive` | Send to or receive from a channel |
| `channel.select` | Execute a channel `select` operation |
| `channel.close` | Invoke Go's built-in channel close operation |
| `concurrency.lock`, `concurrency.unlock` | Lock/unlock recognized `sync.Mutex` and `sync.RWMutex` values, or release/reacquire the `sync.Locker` at exact `sync.Cond.Wait` calls |
| `concurrency.task` | Adjust or start a task tracked by a recognized `sync.WaitGroup` |
| `concurrency.wait` | Wait on a recognized `sync.WaitGroup` counter, `sync.Once` callback barrier, or exact `sync.Cond.Wait` call, including invocations of locally proven functions returned by `sync.OnceFunc`, `sync.OnceValue`, and `sync.OnceValues` |
| `concurrency.condition.signal`, `concurrency.condition.broadcast` | Call exact `sync.Cond.Signal` (one waiter) or `Broadcast` (all waiters); waiter state remains uncertain |
| `concurrency.sleep` | Delay the current goroutine through exact `time.Sleep` when its duration may be positive |
| `concurrency.atomic` | Call an exact `sync/atomic` memory-operation API |
| `concurrency.map.read`, `concurrency.map.write` | Read or mutate an exact `sync.Map`; state-dependent mutations and `Range` retain uncertainty |
| `concurrency.pool.get`, `concurrency.pool.put` | Call exact `sync.Pool.Get` or `Put`; `Get` retains possible `New` callback uncertainty |
| `crypto.x509.certpool.add` | Attempt to add certificates through exact `crypto/x509.CertPool` APIs; PEM parsing and constraint callbacks retain uncertainty |
| `crypto.x509.certpool.system` | Request a copy of the platform system certificate pool through exact `crypto/x509.SystemCertPool` calls |
| `concurrency.callback.register` | Schedule asynchronous callback work through `context.AfterFunc` or `time.AfterFunc` |
| `concurrency.callback.stop` | Invoke a proven `context.AfterFunc` stop function; the callback may already have started |
| `concurrency.timer.schedule` | Schedule time notifications through exact `time.After`, `NewTimer`, `NewTicker`, or `Tick` calls |
| `concurrency.timer.stop` | Call exact `time.Timer.Stop` or `time.Ticker.Stop`; Timer state/success may be unknown |
| `concurrency.timer.reset` | Call exact `time.Timer.Reset` or `time.Ticker.Reset`; timer state or interval may be unknown |

SSA concurrency sites are recognized independently of API summaries:
`ssa.Go`, `ssa.Defer`, `ssa.Send`, channel-receive `ssa.UnOp`, `ssa.Select`, and
exact built-in `close` calls. The instruction `api` identities are those SSA
instruction names; `close` uses `builtin.close`. Source locations and IDs
use the existing source-site scheme. A `go` site has `kind: "go"`; a site for
a summarized deferred operation has `kind: "defer"`; ordinary calls retain
`kind: "call"`. Deferral does not add a separate operation category. A
goroutine site records a launch, not guaranteed completion of its body. A
defer site records registration for later execution, not an immediate
invocation. The analyzer also follows statically established callee bodies as
possible reachable work; this is not a runtime schedule or proof of execution.
Channel operations mark coordination points only: a receive/send/select does
not prove blocking, communication completion, or deadlock. `channel.close`
identifies an exact built-in `close` call, not a successful state transition;
closing a nil or already closed channel panics, and no receiver completion is
inferred. These categories do not report arbitrary memory writes.

Recognized `sync.Mutex.Lock` / `Unlock` and `sync.RWMutex.Lock`, `RLock`,
`Unlock`, and `RUnlock` calls report `concurrency.lock` or
`concurrency.unlock`. These identify synchronization calls, not lock ownership,
critical-section safety, successful completion, or deadlock.

Exact `sync.WaitGroup.Add`, `Done`, and `Go` methods report `concurrency.task`;
`Wait` reports `concurrency.wait`. `Add` adjusts the counter; `Add` or `Done`
can panic if it becomes negative. Positive additions to an empty group must
happen before `Wait`. `Done` decrements the counter. These identify task-counter
operations, not correct accounting or proof that associated work completed. `Wait` can
block until the counter reaches zero, but that does not prove tasks were
correctly registered or kept active for their full work. `WaitGroup.Go` also
reports `concurrency.goroutine`; its callback must not panic and remains
uncertain because the analyzer does not establish a synchronous call edge to
the callback body. Exact `sync.Once.Do` calls also report `concurrency.wait`:
all calls return only after the one callback finishes, and concurrent calls may
wait for it. The callback remains uncertain and its effects are not inherited
without a proven call edge. Calls through returned functions are also recognized
when local SSA proves every value path comes from the same exact
`sync.OnceFunc`, `sync.OnceValue`, or `sync.OnceValues` constructor. The wait is
reported at invocation; construction alone is not a wait. Arbitrary, mixed,
helper-returned, and memory-loaded function values remain unresolved. Callback
bodies are not inherited without a proven call edge. These summaries do not
establish callback execution per call, ordering beyond the API contract, or task
completion. Same-named methods on other types are not summarized.

Exact `sync.Cond.Wait` reports `concurrency.wait`, `concurrency.unlock`, and
`concurrency.lock`: it releases the associated `sync.Locker` while waiting and
reacquires it before returning. Because the locker is an interface, its concrete
method dispatch remains unresolved and is retained as `interface_dispatch`
uncertainty. The report does not identify a concrete lock or inherit effects
from its methods.
Callers should recheck their condition in a loop; a wake does not establish that
an application predicate is true. Exact `sync.Cond.Signal` and `Broadcast`
report distinct one-waiter and all-waiter notification attempts with
`state_condition` uncertainty because the current waiter set is unknown. These
effects do not establish which goroutine is awakened or that it runs promptly;
the analysis does not pair a notification with the `Wait` calls it unblocks or
report their conditional synchronizes-before relation. Same-named methods and
open interfaces are not treated as exact Cond operations.

Exact `sync/atomic` functions and typed methods report `concurrency.atomic`.
Go defines these atomic operations as sequentially consistent; when one
operation observes another, the first synchronizes before the second. A site
identifies the API operation only: it does not establish that its value is
shared, which operations observe each other, that a synchronization protocol is
correct, or that the operation completed successfully. Ordinary local stores
and map updates are not classified as atomic operations.

Exact `sync.Map` methods report `concurrency.map.read` and/or
`concurrency.map.write`: `Load` and `Range` read; `Store`, `Delete`, and
`Clear` write; and `Swap` reads and writes. `LoadAndDelete`, `LoadOrStore`,
`CompareAndSwap`, and `CompareAndDelete` report both with `state_condition`
because their mutation depends on the current mapping. `Range` also retains
`callback` uncertainty: its callback is not traversed, and the iteration is not
a consistent snapshot. These categories identify exact map operations. The read/write labels denote
access and update intent, not the formal Go memory-model classification or a
synchronization edge. They do not establish a shared instance, a successful
state change, observed synchronization, or a correct concurrent protocol. Go's
`sync.Map` contract permits concurrent use; the report does not establish which
calls use the same map or observe each other.

Exact `sync.Pool.Get` and `Put` calls report `concurrency.pool.get` and
`concurrency.pool.put`. `Get` may return an arbitrary pooled value, treat the
pool as empty, and invoke `New`; it retains `callback` uncertainty because the
analyzer does not establish pool contents or the `New` callback. `Put` may add
a value that the runtime later removes without notice. The report identifies
these API calls, not item reuse, ownership transfer, or a guaranteed state
change. Go's memory-model relation applies only when `Get` returns the same
value passed to `Put`; the analyzer does not establish shared pool identity or
that relation for any pair of sites.

Exact `crypto/x509.(*CertPool).AddCert`, `AppendCertsFromPEM`, and
`AddCertWithConstraint` calls report `crypto.x509.certpool.add`. This identifies
an attempted addition to an in-memory certificate pool, not proof the pool is
used for TLS trust, the certificate is valid or trusted, or the addition
succeeded. `AppendCertsFromPEM` retains `argument_condition` uncertainty because
its input determines whether certificates are parsed and appended.
`AddCertWithConstraint` retains `callback` uncertainty: its constraint may run
later, zero or more times during certificate verification, and can run
concurrently. Callback effects are not inherited at registration. Exact
`x509.NewCertPool` is treated as an effect-free boundary because it creates an
empty in-memory pool. Exact `x509.SystemCertPool` calls report
`crypto.x509.certpool.system`, identifying a request for a copy of the platform
system certificate pool. The call can return an error, and environment variables
or platform APIs can affect the source of roots; the effect does not establish
which roots were returned, whether the call succeeded, or whether the pool is
used for TLS verification. This is not a general filesystem-read summary.

The centralized table in `internal/effects/effects.go` recognizes common `os`
(including `File` and `Root`), `path/filepath`, `io/ioutil`, `net`, `crypto/tls`,
`net/http`, `os/exec`, and `database/sql` APIs. It recognizes selected in-memory operations such as basic-argument
`fmt.Sprintf`, nil-qualifier `go/types.TypeString`, and `bytes.Buffer` methods
as effect-free summary boundaries, preventing their implementation plumbing
from being interpreted as I/O. Formatting or type-string callbacks that could
execute application code are marked uncertain instead. `os.File.Write` remains `fs.write`, but
that is a resource-kind approximation: a file descriptor may be a pipe, device,
or standard output. `File.Close` is `resource.release`, **not** `fs.write`.

`os.Rename` and `os.(*Root).Rename` report `fs.rename`, in addition to their
existing `fs.create`/`fs.remove` entry-change categories. Use `-effect rename` or
`-api os.Rename` to find them; `fs.rename` is a distinct category, not an alias
for `fs.write`. The categories describe possible entry changes, not separate
create/remove calls or proof that the destination was new. No method-name-only
rename matching is performed. This is an additive vocabulary change; JSON
structures and source IDs are unchanged.

`ResponseWriter.Write` and `WriteHeader` have a named-interface semantic summary
(`http.response`), even for an open receiver. This says a response write is possible,
not that a particular implementation or network target was found; open dispatch
remains uncertain and adds no concrete call edge. The response could be recorded
in memory. Unknown `io.Writer.Write` remains unclassified, never `fs.write` just
because some file type implements it.

`os.Process.Kill` and `os.Process.Signal` report `process.control`;
`os.Process.Wait` and `os.Process.Release` report `resource.release`. These
identify process-control and lifecycle operations, not successful termination
or completion; `Wait` may block until the process exits. Exact
`os/signal.Notify` calls can report `process.signal.register`: this identifies
channel registration only, not arrival of a signal or execution of code that
receives it. A statically known nil channel is excluded because Notify panics
before registering it. Statically known custom `os.Signal` types that the
platform implementation does not recognize are also excluded. When static
evidence cannot establish a non-nil channel or a supported signal value, the
registration effect remains marked uncertain with `argument_condition`; an
unknown signal argument may or may not cause registration. Even `os.Interrupt`
and `syscall.Signal` values retain uncertainty because this summary does not
prove their runtime value or platform support (`os.Interrupt` is a mutable
package variable). With no signal arguments, Notify requests all signals.
Exact `os/signal.Stop` calls report possible `process.signal.unregister` with
`state_condition` uncertainty: a call site does not establish that the channel
was previously registered. `Stop(nil)` is excluded because it is a no-op; a
non-nil channel may still have no registration to remove. The analyzer does not
track registrations across calls, even when a matching `Notify` appears nearby.
`NotifyContext` reports possible registration and a possible watcher-goroutine
launch when local SSA does not prove a nil parent or a list of unsupported
signals. It retains `argument_condition` uncertainty: the parent may be nil,
explicit signal support may depend on platform/runtime values, and the watcher
is not started if the returned context is already done. It does not establish
signal delivery or cancellation by an arriving signal.

A call through the stop function is summarized only when local SSA directly
traces the `context.CancelFunc` to the second result of `os/signal.NotifyContext`.
That invocation reports `context.cancel` and `process.signal.unregister`: the
contract cancels the returned context and stops relaying signals for its
internal channel, releasing associated resources. If no supported signals were
registered, there may be nothing to unregister. It does not prove a signal was
registered or delivered. Arbitrary, mixed, and helper-returned function
values remain unresolved; their named function type alone is not evidence.
Exact `os/signal.Ignore` and `Reset` calls can report distinct possible policy
operations. With no signal arguments (including a nil or proven empty slice
passed with `...`), each applies to all signals. A single nil `os.Signal` value
is unsupported. Signal-list inspection requires a single-use local array slice
with proven bounds and initialization; helper uses, escaping aliases and dynamic
bounds retain `argument_condition` uncertainty instead of assuming the initial
values survive. Statically unsupported concrete signal types in proven lists
are excluded, while syscall signal values and unknown argument flows retain
`argument_condition` uncertainty. These effects do not establish signal
delivery, correlate prior registrations, or prove a particular runtime signal
state changed.

`context.cancel` is also reported when local SSA proves that the invoked
`context.CancelFunc` was extracted from `context.WithCancel`, `WithDeadline`,
`WithDeadlineCause`, `WithTimeout`, or `WithTimeoutCause`, or that a
`context.CancelCauseFunc` came from `WithCancelCause`. These named function
types can wrap arbitrary application code, so other values—including values
merged through unsupported memory flow or returned by helpers—remain
unresolved rather than being assumed to cancel a context.

Exact `context.AfterFunc` calls report `concurrency.callback.register` with
`callback` uncertainty: they arrange for the callback to run asynchronously
when the context is canceled, but it may never run, and its body is not followed
as a synchronous call. Exact `time.AfterFunc` calls report the same effect with
`callback` uncertainty: they schedule the supplied callback asynchronously
when the duration elapses; callback execution and completion are not inferred.
Exact `time.After` and `time.NewTimer` calls report
`concurrency.timer.schedule`: they arrange a one-shot time notification, but do
not establish when a receive occurs or that a receiver runs. Exact
`time.NewTicker` and `time.Tick` calls report the same effect for recurring
notifications. A statically known non-positive `NewTicker` interval is reported
as uncertain with `argument_condition` and no scheduling effect because the API
panics; an unknown interval retains possible scheduling with the same reason.
`time.Tick` returns nil for a statically known non-positive interval, so that
call has no scheduling effect. Unknown `Tick` intervals retain possible
scheduling with `argument_condition`.

Exact `time.Timer.Stop` and `Reset` calls report `concurrency.timer.stop` and
`concurrency.timer.reset` with `state_condition` uncertainty. The report records
the method attempt, not whether the timer was active, whether Stop prevented
firing, or whether Reset rescheduled work. Exact `time.Ticker.Stop` reports
`concurrency.timer.stop`: the API prevents future ticks after the call
completes, but does not close the ticker channel. The report identifies the
method call, not proof the receiver was non-nil or the call completed.
`time.Ticker.Reset` reports `concurrency.timer.reset` for a known
positive interval; an unknown interval retains `argument_condition`, and a
statically non-positive interval is reported as unresolved because Reset
panics. On an AfterFunc timer, Reset may schedule another callback while a
prior callback is still running; neither method is treated as waiting for
callback completion. These summaries do not establish timer delivery, receive,
or callback execution. Same-named application timer methods are not matched.

Exact `time.Sleep` calls with a statically positive duration report
`concurrency.sleep`. A statically non-positive duration has no sleep effect;
an unknown duration retains possible `concurrency.sleep` with
`argument_condition` uncertainty because `Sleep` returns immediately for
non-positive durations. This identifies a possible delay of the current
goroutine only; it does not establish the duration or scheduling behavior.

Local SSA traces the returned stop function only when
all value paths lead directly to `context.AfterFunc`. Invoking it reports
`concurrency.callback.stop` with `state_condition` uncertainty: it returns true
only if it stopped the callback before it ran; false can mean the callback has
started or the association was already stopped. It does not wait for callback
completion or cancel the context. Arbitrary, mixed, and helper-returned stop
functions remain unresolved; same-named application methods are not matched.

Acquisition summaries include caller-owned file/root/pipe handles, concrete
network dial/listen/accept operations, and selected SQL handles. Convenience APIs
such as `ReadFile` do not expose an owned handle. Concrete network closes and
named `net.Conn`/`PacketConn`/`Listener.Close` contracts report `net.close` and
`resource.release`; half-close reports only `net.close`. TLS/HTTP shutdown
summaries retain callback uncertainty where appropriate. These are possible
operations, not successful acquisitions, leak checks, pairing, or lifetime proofs.

`io.Closer.Close` (including embedded contracts such as `io.ReadWriteCloser`)
reports `resource.release`, with `interface_dispatch` uncertainty when the concrete
target is unresolved. It does not imply `net.close` or any particular resource,
successful cleanup, or implementation edge. Structurally similar application
interfaces that independently declare `Close` remain unclassified. Bound method
calls retain the same contract and uncertainty.

`os.OpenFile` uses a constant flag argument when SSA provides one: read-only
reports `fs.read`; write/create flags report their applicable effects. Unknown
flags conservatively report read, write, and create.

## Analysis model and limits

Direct effects come from recognized calls and the listed SSA operations.
Transitive effects are propagated through ordinary calls, methods, closures,
known local function values, bounded returned-function provenance, generics,
`go`, and `defer`; recursion
uses a stable worklist. In detailed and effect-view edge evidence, `kind`
distinguishes ordinary `call`, `go`, and `defer` relationships. The latter two
describe possible reachable work, not synchronous execution: a goroutine body
may not complete, and a deferred call is registered for a later exit path. An
empty effect set is not proof of purity.

### Function-value provenance

For function-typed results of static calls into loaded application/dependency
bodies, bounded symbolic return summaries follow assignments, phi values,
closures, tuple results and helper layers. Function parameters in return
summaries are substituted with that call's arguments (including generic and
named function types). A closed, non-nil singleton establishes the actual
invocation target. Known finite alternatives instead have
`candidate_function_value` edges and only `possible_effects`; they are not VTA
proofs or several certain calls. Even common effects of summarized alternatives
stay possible-only: normal invocation is not proven. Unknown alternatives retain
uncertainty. Known alternatives with opaque work inside their bodies also retain
uncertainty. `-effect` and both graph views include candidate evidence without
promoting it. Source IDs and go/defer invocation forms are preserved.

Each queried value has a memoized analysis bounded to 4,096 work steps and 64
nested evaluator levels. Cycles, missing bodies, opaque standard-library results
and budget exhaustion stay open; incomplete summaries are not shared between
queries. No execution paths are enumerated. Summaries model return identity, not
execution of a returned/retained callback or a closure environment. Captured
ordinary data never establishes paths/permissions/arguments; captured function
loads, aliases, struct/interface memory and external callback parameters stay
uncertain. For callback-invoking helpers such as `execute`, function arguments
observed at statically known calls can add `candidate_function_value` targets at
helper's invocation site. They remain candidates: observed callers do not prove
caller completeness, and context-insensitive summaries can include targets from
other callers. Thus `execute(makeWriter(path))` remains uncertain rather than
proving that `makeWriter` is its sole target. For callback-argument candidates,
dependency bodies contribute only when they are in the selected or
budget-expanded VTA scope. This does not specialize arbitrary callbacks or make
open parameters globally certain. A factory with a defer may have SSA
result-cell loads; these remain unsupported
memory flow. Known stdlib function-value summaries retain their existing narrow
provenance contracts rather than inspecting standard-library bodies.

Interface dispatch is intentionally evidence-based. Standard-library packages
are identified from Go loader module metadata, not import-path spelling. A
receiver made from a known concrete SSA value is followed to that method. An interface parameter,
memory-loaded interface, or other open receiver is **not** expanded to every
loaded implementation (for example every `io.Writer`); it is marked uncertain
instead. This avoids inventing filesystem writes from unrelated `*os.File`
implementations while retaining the fact that analysis is incomplete.

Known standard-library summaries stop traversal. An unrecognized standard
library call, reflection invocation, cgo, assembly, missing body, or unsupported
function-value flow is uncertain rather than guessed. Branch feasibility,
resource destinations, permissions, plugins, all build tags/platforms, shared
memory mutation, and secondary driver/callback effects are not modeled. Lowered range-over-function
iterator callbacks and SSA-generated code without source locations can lack a
complete detailed explanation; generated yield helpers are not source inventory.
Output limits bound explanations, not Go loading or SSA construction cost. Results
are possible static reachability, not proof of execution or purity.

Module loading uses `-mod=readonly`. Workspace metadata is copied to a temporary
workspace so analyzed `go.mod`, `go.sum`, and `go.work.sum` are not repaired or
modified. The Go toolchain may still update its own caches.

## Verification

```sh
go build ./...
go test ./... -count=1
go vet ./...
```

The release CI matrix targets Go 1.26+ on Linux, macOS, and Windows. Local
acceptance evidence is environment-specific; it does not imply that hosted CI
has run successfully. Keep private benchmark workspaces, source checkouts,
sessions and evaluation outputs outside this repository. Local source checkouts
live at `/Users/rob/Development/benchmark-sources`; historical evaluation material
is retained privately outside the release tree. Do not recreate `experiments/`
inside this repository.

The executable integration suite analyzes fixtures without running them. It
covers recursive/exact/pattern/test selection, source inventory/counts and JSON
compatibility, API/source provenance, independent/shared sites, forward witnesses
and reverse callers, cycles/depth/limits, generic variant isolation, grouped
uncertainty reasons and inheritance, HTTP/resource semantics, known/open
interfaces, memory and file writers, formatting/type strings, OpenFile flags,
determinism, and source/workspace preservation.
