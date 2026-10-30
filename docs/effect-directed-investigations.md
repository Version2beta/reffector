# Effect-directed investigations

Use Reffector to find statically recognized operation sites and inspect the
source/caller evidence around a bounded question. For example:

```sh
reffector -effect fs.write -max-depth 3 -max-sites 100 -max-edges 300 ./internal/analyzer
```

Start with `-max-depth 0` when discovering operation groups; then narrow by
`-api`, `-site`, `-function`, or `-effect` and increase reverse-caller depth only
as needed. Check the report's selected package scope and coverage/truncation
fields (sites, edges, and depth) before drawing conclusions. A result may be
bounded, filtered, or incomplete; remove filters or raise limits when the
question requires broader coverage.

For each relevant site, read the cited source and its callers. Follow the
behavior that controls the operation: inputs, policy and contracts, error
handling, ordering, cancellation, and cleanup. Check relevant interfaces and
API contracts, and use focused safe tests where they can clarify behavior.
Keep established call evidence separate from candidate relationships and
unresolved leads. Record what the source establishes, what is only a candidate,
and what remains unresolved; do not turn possibility into certainty.

Report the query and scope, the sites and source/caller evidence reviewed,
relevant tests and their results, limitations or omitted coverage, and the
remaining uncertainties. Static effects and reachability do not prove that a
function or operation executes at runtime, nor do they establish a defect.

## Copyable investigation prompt

```text
Investigate this bounded behavior question using the Go source and Reffector:
<question>

Scope: <package or Go pattern>

Run a direct bounded query, for example:
reffector -max-depth 2 -max-sites 100 -max-edges 300 <scope>

Check which packages were selected and whether loading, site, edge, or depth
limits truncated the result. Read the cited operation source, relevant callers,
contracts, and tests. Distinguish established relationships, candidate leads,
and unresolved evidence; do not present candidates as established behavior.
Use focused safe tests if useful. Do not edit files unless I explicitly ask.

Report the query and scope, evidence inspected, test commands/results, findings
and uncertainties, and coverage or other limitations. Static effects and
reachability do not prove runtime execution or a defect.
```
