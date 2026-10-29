# Milestone 3 — candidate/unresolved category sample (iteration 16)

## Reproduction and sampling rule

From the repository root, build and run the current analyzer, then build the inventory with cap 100. Select the lexicographically smallest stable candidate ID having category `candidate-unresolved-relationships`:

```sh
go build -o /tmp/reffector-iter16 ./cmd/reffector
/tmp/reffector-iter16 -json -all -max-depth 100 . > /tmp/reffector-iter16-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
r = json.load(open('/tmp/reffector-iter16-effects.json'))
b = build(r, max_candidates=100)
cs = [c for c in b['candidates']
      if 'candidate-unresolved-relationships' in c['categories']]
c = min(cs, key=lambda c: c['id'])
print(b['counts'], len(cs), c['id'], c['site_id'])
PY
```

Build, analyzer, and builder succeeded. EffectReport counts: 59 sites, 99 nodes, 238 edges; inventory: 57 candidates, 57 packets emitted, 0 deferred. Six of 57 candidates (overlapping categories) have `candidate-unresolved-relationships`; selection is purposive feature coverage, not a random sample. Lowest stable ID selected: `candidate:20b086d14b27d28342c653af9d9d11b394f4399d1303cf1edb89d7ef06f1dfc9`, site `site:e606fe75ec52ec392f5c260a19cbcefac1380bbd5919179402852ba635e0fd6a`, `golang.org/x/tools/go/types/typeutil.(*MethodSetCache).MethodSet` / `sync.(*Mutex).Lock`, site line 32. Categories are multi-frontier reachability, candidate/unresolved relationships, branching/convergence, and cross-component shared operation site.

## Source and relation audit

Pinned dependency source in the local module cache, `golang.org/x/tools@v0.52.0/go/types/typeutil/methodsetcache.go:17-54`, documents `MethodSetCache.MethodSet` as thread-safe, handles a nil receiver by calling `types.NewMethodSet`, otherwise locks `cache.mu` and defers unlock around cache lookup/insertion. This supports the intended synchronization pattern; it is not evidence of a defect, deadlock, or race.

The candidate's retained 15 graph nodes and 21 relationships are contextual evidence, not 21 direct callers of the lock. Its owner node is `(*MethodSetCache).MethodSet`; the inventory marks it uncertain because graph context includes candidate VTA relations in Reffector's own `(*graph).selectedFunctions$1` closure (candidate links at `internal/analyzer/analyzer.go:687,708,715`). Source at `analyzer.go:680-716` shows the closure visits selected SSA functions, anonymous functions, function operands, and local methods reached through `MakeInterface` and `MethodSet`; the candidate VTA edges remain non-established. Other retained established edges include calls from analyzer context functions to the MethodSetCache method, not evidence that those candidate edges execute. Thus the category is a warning to preserve uncertainty in the surrounding graph, not a claim that this specific mutex operation has an unresolved direct caller.

## Interpretation and limits

Mechanical facts: category count 6/57; selected stable IDs above; source locks and defers unlock for the non-nil-cache path. Potential hazard/hypothesis: none supported by this bounded source review. False lead: the category label alone does not imply uncertainty at the lock's exact call site or a vulnerability. Unknowns: runtime contention, callers outside this analyzer scope, and behavior under concurrent workload were not tested. No code/test or selection change is justified. This adds one reproducible category sample but does not close milestone 3's diverse denominator/deferral gate: no candidates were deferred in this run, and selection-cap behavior remains untested. No model call or runtime experiment was made.
