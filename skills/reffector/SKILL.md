---
name: reffector
description: Retrieve and interpret Go effect sites, reverse callers, and unresolved boundaries with Reffector. Use for effect investigation, architecture, change impact, test planning, or code review; effects are not defect findings.
metadata:
  version: "phase2-1"
---

# Reffector — investigate operations

Reffector reports recognized possible operations and static reachability, not
execution or defects. Use evidence to answer the user's question; do not impose
a refactoring workflow. Shared categories do not justify consolidation, and
separate implementations may be appropriate. Investigation does not authorize
source edits, dependency repair, downloads, or running the analyzed application.

## Scope and loading

Use a verified binary and Go 1.26+. To build the **current Reffector checkout**:

```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
bin_dir=$(mktemp -d)
go build -o "$bin_dir/reffector" ./cmd/reffector
export PATH="$bin_dir:$PATH"
```

Check `reffector -help`; a preinstalled binary may differ. Run analysis from the
target Go module/workspace with an explicit, narrow package scope. `./...`
selects recursive subpackages; local directories also recurse. `-exact .`
selects only the root package, but does not override an explicit `./...` pattern.
Respect existing workspaces, replacements and build settings; `-tests` includes
test-file functions without executing tests.

Preflight dependencies with `go list -mod=readonly -deps ./...`, using the same
scope/environment (add `-test` for `-tests`). In a workspace prefer Reffector's
own loading as the preflight: raw Go tooling can write `go.work.sum`, whereas
Reffector uses temporary workspace metadata. The settings above keep Go and
Reffector loading offline; missing cached dependencies/toolchains are blockers.
Dependency listing is not type checking: Reffector must also load successfully.
Loading errors invalidate the requested analysis, not establish zero effects;
report the blocker rather than silently narrowing scope or repairing modules.

## Focused queries

Adapt the explicit scope to the question; these examples use the target module's
`./...`. Choose a query rather than treating these as mandatory steps.

```sh
# Locate filesystem-write sites without expanding callers.
reffector -effect fs.write -max-depth 0 -max-sites 20 ./...

# Examine an exact response-output API and its reverse callers.
reffector -api 'net/http.(ResponseWriter).WriteHeader' -max-depth 2 -max-sites 20 -max-edges 50 -json ./...

# Explore callers of unresolved interface boundaries, excluding other reasons.
reffector -unresolved -reason interface_dispatch -max-depth 2 -max-sites 20 -max-edges 50 ./...
```

Start with consequential categories → direct operation sites → source locations
and reverse callers. `-effect` matches category substrings (`fs.rename` and
`resource.release` are distinct from `fs.write`); `-api` is exact, `-operation`
is API-substring search. Copy a canonical ID into `-site ID` to expand one site;
keep `-unresolved` for an unresolved-only site. IDs survive filtering, not source
location changes. Multiple callers can share one direct site; independent calls
to the same API remain separate sites. Multi-category sites are not duplicates.

Depth 0 shows origins; 1 adds immediate callers. `-max-sites` and `-max-edges`
bound output, not loading cost. Check report and per-site `truncated` flags;
cycles/revisits are references, not omissions. Graphs show supported reachability,
not all execution paths. Package/function filters restrict roots and reachable
callers; remove them for broader caller context. Prefix a full local function
name with `=` for one exact root, excluding uncalled closures as separate roots.

## Interpret and verify

Default text and `-json` describe the same effect view. JSON groups reference
sites; reverse graphs reference nodes and edges. Follow IDs, not same-spelled symbols
across generic/test variants. Sites carry API identities, locations, recognition
and uncertainty; `operations` preserves API-specific evidence. A declaration
boundary is not a source call. Inherited reachability is not a new direct site.
Reverse edges have `resolution: established` when SSA directly supports the
call, or `candidate_vta` when Go VTA follows values through assignments,
arguments, returns, fields, or callbacks. A candidate is a conservative
possible callee, not proof that it executes; unresolved boundaries remain open.

Uncertainty is separate from effects. `interface_dispatch` means the concrete
implementation remains unresolved: a named contract summary (such as response
output or `io.Closer` release) supplies no concrete target or resource destination.
`-unresolved` selects only effect-free unresolved leads; known-effect uncertain
sites remain in normal groups. `-reason` filters existing reasons in either
selection; it does not classify significance. Remove filters to inspect other
origins, including opaque calls and callbacks. Never invent implementation edges.

Check coverage: selected packages/functions, selected versus dependency sites,
identified versus matching versus emitted counts. `-all -json` adds unfiltered
source inventory; `-stats` also writes coverage to **stderr**. Empty matches,
omissions and `uncertain: false` do not prove absence or purity. Analysis covers
the current build and a limited API vocabulary, not arbitrary dataflow or mutation.

Read cited source and relevant callers/contracts for ordering, locks, authority
checks, ownership, failure handling and policy. Categories establish none of
these; creating a closure is not evidence of invoking it. Candidate VTA edges
are useful leads for consequential implementation sites, but validate
configuration, dispatch conditions and runtime behavior before treating them as
execution. Distinguish analyzer-reported evidence, source observations,
conclusions and open questions.

In an investigation report, record the actual binary/source revision and dirty
state, working directory, package scope/build settings, exact commands and output
limits, loading status, coverage/truncation, site/API/file:line evidence and caller
relationships. State remaining questions and what the evidence cannot establish;
do not turn an operation or an unresolved boundary into a defect claim.
