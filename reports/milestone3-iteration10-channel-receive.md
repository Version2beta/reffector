# Milestone 3 source sample — task completion receive (iteration 10)

## Reproduction and deterministic selection

From the repository root:

```sh
go build -o /tmp/reffector-iter10 ./cmd/reffector
/tmp/reffector-iter10 -json -all -max-depth 100 . > /tmp/reffector-iter10-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
b = build(json.load(open('/tmp/reffector-iter10-effects.json')), max_candidates=100)
print('counts:', b['counts'])
rows = []
for c in b['candidates']:
    s = c['operation']
    effects = set(s.get('effects', [])) | set(s.get('possible_effects', []))
    if 'channel.receive' in effects:
        rows.append((c['id'], s['id'], s['package'], s['function'], s['api'], s['location']))
print('channel.receive candidates:', len(rows))
print(*sorted(rows), sep='\n')
PY
```

Observed denominator: 59 operation sites, 99 nodes, 238 edges; 57 candidates, all 57 emitted, 0 deferred. There are 3 `channel.receive` candidates in the complete builder candidate inventory. Selection rule: choose the lexicographically lowest stable candidate ID among candidates with `channel.receive`. This selects `candidate:40af433ac26b26fe47db1761a0e96d831c0dcfdf73405350b1c19f9a34b5eacd`, site `site:5354a18d78c89283178a741dd1d809b9602ac430d87957723ace4969eaba2b1e`, `golang.org/x/tools/go/ssa.(*task).wait`, `ssa.Receive`, reported line 85. The two other rows are distinct receive candidates in `(*Program).Build$1` and `(*Group).done`. This is a deterministic purposive category sample, not representative sampling; effect labels can be overlapping and the three records are not necessarily independent designs.

## Source observations

Inspected the versioned module source `golang.org/x/tools@v0.52.0/go/ssa/task.go:22-25,31-43,51-63,68-95` (module cache source, corresponding to the analyzer's package/version and reported line).

- **Mechanical facts:** A task has a `done` channel and predecessor-edge map. `addEdge(x,y)` represents that waiting on `x` must also wait for `y`; it ignores self/already-transitively-done dependencies and checks that `x` is not done. `markDone` closes `x.done` for non-nil `x`. `wait` returns early if already transitively done; otherwise breadth-first traverses dependencies, receives from each not-yet-transitively-done task's `done` channel, adds its outgoing dependencies once, then marks traversed non-nil tasks transitively done using atomic stores.
- **Potential hazard / false lead:** At the candidate receive, blocking is the intended completion synchronization, not evidence of a deadlock by itself. The source comment requires all `addEdge` calls before `markDone`; the traversal relies on that contract. Nothing in this isolated candidate establishes that all callers honor the ordering or that a reachable task cannot remain unfinished.
- **Hypothesis status:** no defect demonstrated; no selection or ordering change is warranted from this single source check. It provides a channel-receive category outside the prior x/sync errgroup samples, although it is from x/tools task internals and this category's broader behavior is not audited.
- **Unknowns:** no runtime schedule/fault test, caller-wide contract audit, or CFG/provenance re-evaluation was performed. Static source and the effect record do not establish runtime reachability or whether every caller satisfies the documented ordering.

## Validation and limits

- `go build -o /tmp/reffector-iter10 ./cmd/reffector` — passed.
- `/tmp/reffector-iter10 -json -all -max-depth 100 .` — passed; builder counts and sample inventory are recorded above.
- Source was read from the local Go module cache at the stated version; it is not a modification to any dependency tree.
- No code or tests changed; no model call or runtime experiment was made. This sample does not close the milestone 3 diversity/deferral audit gate.
