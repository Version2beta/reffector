# Milestone 3 source sample — concurrency lock (iteration 13)

## Reproduction and deterministic selection

From the repository root:

```sh
go build -o /tmp/reffector-iter13 ./cmd/reffector
/tmp/reffector-iter13 -json -all -max-depth 100 . > /tmp/reffector-iter13-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
b = build(json.load(open('/tmp/reffector-iter13-effects.json')), max_candidates=100)
rows = []
for c in b['candidates']:
    s = c['operation']
    effects = set(s.get('effects', [])) | set(s.get('possible_effects', []))
    if 'concurrency.lock' in effects:
        rows.append((c['id'], s['id'], s['package'], s['function'], s['api'], s['location']))
print('counts:', b['counts'])
print('concurrency.lock candidates:', len(rows))
for row in sorted(rows): print(row)
PY
```

The current inventory is 59 sites, 99 nodes, 238 edges, 57 candidates; all 57 packets emitted, none deferred. Seven candidates have `concurrency.lock` in `effects` or `possible_effects` (labels may overlap). Selection rule: lowest stable candidate ID lexicographically among those seven. Selected candidate `candidate:0a60be656e03b315ce258a2b34233d56aa6477c754652bfa693ca51e0401261b`, site `site:9f8e629cb09ae3e57ebe1df580702135190e2d1e59723bbaa96422224dc8be02`, `golang.org/x/tools/go/ssa.(*canonizer).List` / `sync.(*Mutex).Lock`, line 299. Reproduction's source path is unavailable because no source root/manifest was supplied; source was reviewed in the module cache at `golang.org/x/tools@v0.52.0/go/ssa/util.go`.

## Source observations

Inspected `util.go:255-304`.

- **Mechanical facts:** `canonizer` documents itself as thread-safe and holds a mutex alongside the `types` and `lists` maps. `List` returns nil for an empty input; for nonempty input it removes top-level aliases only when present, locks `c.mu`, defers unlock, and calls `c.lists.rep(l)` while holding the lock. `Type` uses the same mutex around access to `c.types` (lines 286-304).
- **Potential hazard / false lead:** This operation is synchronization for shared mutable canonicalization state; seeing a lock by itself does not indicate a deadlock, contention issue, or race. The code's deferred unlock bounds the critical section to the representation lookup/creation. No contention profile or concurrent stress test was performed.
- **Hypothesis status:** No defect or selection-order change is supported by this local source review. This is a distinct `concurrency.lock` category sample, not evidence that all locks are correctly handled.
- **Unknowns:** The internal behavior/cost of `typeListMap.rep` and broader call-site lock interactions were not examined; this bounded review does not assess runtime contention or lock-order behavior elsewhere.

## Validation and limits

- `go build -o /tmp/reffector-iter13 ./cmd/reffector` — passed.
- `/tmp/reffector-iter13 -json -all -max-depth 100 .` — passed.
- Deterministic category scan — 7 `concurrency.lock` matches out of 57 candidates; selected the minimum stable ID.
- No code/tests changed, no model call, and no runtime experiment. This purposive sample does not close milestone 3's remaining diversity/deferral gate.
