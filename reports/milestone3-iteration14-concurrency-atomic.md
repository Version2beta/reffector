# Milestone 3 source sample — atomic scope ID (iteration 14)

## Reproduction and deterministic selection

From the repository root:

```sh
go build -o /tmp/reffector-iter14 ./cmd/reffector
/tmp/reffector-iter14 -json -all -max-depth 100 . > /tmp/reffector-iter14-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
b = build(json.load(open('/tmp/reffector-iter14-effects.json')), max_candidates=100)
rows = []
for c in b['candidates']:
    s = c['operation']
    effects = set(s.get('effects', [])) | set(s.get('possible_effects', []))
    if 'concurrency.atomic' in effects:
        rows.append((c['id'], c['site_id'], s['package'], s['function'], s['api'], s['location']))
print('counts:', b['counts'])
print('concurrency.atomic candidates:', len(rows))
for row in sorted(rows): print(row)
PY
```

The complete current inventory has 59 operation sites, 99 nodes, 238 edges, 57 candidates, 57 packets emitted, and 0 deferred. The scan found 3 `concurrency.atomic` candidates of 57; category labels are potentially overlapping. Selection rule: lowest stable candidate ID lexicographically among those 3. Selected candidate `candidate:0aa418d3952efbbd0629b4b7c73010ff09edd238003adaac7609ae8b64a761fe`, site `site:bf355e0016f60bb796bc61cb11ee83a7ea0234093e1323507177ce85573fbe0d`, `golang.org/x/tools/go/callgraph/vta/internal/trie.newScope` / `sync/atomic.(*Int32).Add`, reported line 22. Analyzer source location did not resolve to a repository source excerpt because no source root/manifest was supplied. Reviewed pinned module cache source at `golang.org/x/tools@v0.52.0/go/callgraph/vta/internal/trie/scope.go:11-23`.

## Source observations

- **Mechanical facts:** `Scope` carries an `int32` ID and its comment defines same-scope maps as equal and different-scope maps as distinct. `nextScopeId` is package-global `atomic.Int32`; `newScope` calls `nextScopeId.Add(1)` and stores the result in the returned `Scope`. The comment says each `Builder` creates maps within a unique `Scope`.
- **Potential hazard / false lead:** The atomic increment is consistent with assigning concurrent unique IDs; its presence alone is not evidence of a race or defect. No defect is established by this local inspection.
- **Hypothesis status:** No selection-order change is supported by this sample. The source comment describes the uniqueness intent, not dynamic proof that every Builder uses a distinct scope under all lifetime/overflow conditions.
- **Unknowns:** Did not inspect all callers/lifetimes of `newScope`, exercise concurrent Builder creation, or assess counter exhaustion/wraparound. No runtime test or performance measurement was performed.

## Validation and limits

- `go build -o /tmp/reffector-iter14 ./cmd/reffector` — passed.
- `/tmp/reffector-iter14 -json -all -max-depth 100 .` — passed; resulting report was consumed by the deterministic scan.
- Deterministic category scan — 3 `concurrency.atomic` candidates out of 57; selected minimum stable candidate ID.
- No code/tests changed, no additional model call, and no runtime experiment. This purposive category sample does not close milestone 3's deferral/diversity gate.
