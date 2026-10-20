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
analyzer remains fully useful without it. See
[`PHASE2_SKILL_VERIFICATION.md`](PHASE2_SKILL_VERIFICATION.md) for tested commands
and local loading instructions.

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
operations in the program.

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
calls a summarized operation; otherwise the effect was inherited. `uncertain`
means an unresolved or opaque operation is reachable. It is deliberately
separate from effects: uncertainty is not an operation and is never printed in
function-pair text output. A function with no recognized effects and
`uncertain: true` remains visible in function JSON; its grouped origins are leads
in the effect view. Neither an empty effect list nor
`uncertain: false` proves a function is pure.

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
- `coverage`, report-level `truncated`, and optional unfiltered `inventory`.

A site is the same source-based object in both views. It groups operations at
actual source calls, with stable IDs, containing package/function, types-based
API identities, exact file/line/column, effect categories, recognition and
uncertainty. `apis` lists exact API identities supported at the site; `api` is
the sole identity, or empty when multiple alternatives exist (or no target is
known). A function-value phi may have multiple APIs at one source site, not
several independent sites. Site IDs do not depend on effects, reasons or filters.
Site `kind` is `call`, or `declaration` for missing-body/opaque-boundary origins.
Call positions refer to the SSA instruction (usually the opening parenthesis).
Selecting a summarized standard-library function does not fabricate a call to
itself. IDs use package/source identity, not traversal order.

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
| `interface_dispatch` | Receiver implementation is not resolved; any named semantic summary is not a concrete target |
| `function_value` | Function value/callback flows through an unsupported parameter, memory or result |
| `missing_body` | No analyzable Go body (for example assembly or cgo) |
| `unrecognized_stdlib` | Standard-library call has no summary |
| `reflection_opaque` | Reflection or opaque reflective invocation |
| `callback` | Summary boundary may invoke unmodeled hooks, drivers or callbacks |
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
| `process.exit` | Terminate the process with `os.Exit` |
| `env.read`, `env.write` | Read or modify process environment values |
| `db.query`, `db.exec` | Database query or command operations |

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

Direct effects come from a recognized call. Transitive effects are propagated
through ordinary calls, methods, closures, known local function values, phi
merges, generics, `go`, and `defer`; recursion uses a stable worklist.

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
go test ./...
go vet ./...
```

The executable integration suite analyzes fixtures without running them. It
covers recursive/exact/pattern/test selection, source inventory/counts and JSON
compatibility, API/source provenance, independent/shared sites, forward witnesses
and reverse callers, cycles/depth/limits, generic variant isolation, grouped
uncertainty reasons and inheritance, HTTP/resource semantics, known/open
interfaces, memory and file writers, formatting/type strings, OpenFile flags,
determinism, and source/workspace preservation.
