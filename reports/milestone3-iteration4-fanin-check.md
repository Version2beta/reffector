# Milestone 3 follow-up — fan-in metric check (iteration 4)

## Scope and reproducibility

This bounded check reruns the current-repository analyzer and candidate builder, then counts incoming graph edges whose `callee` is one of each candidate's `node_ids`. It does **not** treat this graph-wide count as operation-site fan-in: the edge endpoint set includes the candidate's whole investigation graph, so the metric can be inflated by graph size. Reproduce from repository root:

```sh
go build -o /tmp/reffector-iter4 ./cmd/reffector
/tmp/reffector-iter4 -json -all -max-depth 100 . > /tmp/reffector-iter4-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
r = json.load(open('/tmp/reffector-iter4-effects.json'))
b = build(r, max_candidates=100)
for c in sorted(b['candidates'], key=lambda c: sum(e['callee'] in c['node_ids'] for e in r['edges']), reverse=True)[:1]:
    es = [e for e in r['edges'] if e['callee'] in c['node_ids']]
    print(c['id'], c['site_id'], c['operation']['function'], c['operation']['api'],
          len(es), len({e['caller'] for e in es}), sum(e['candidate'] for e in es))
PY
```

Run result: 59 sites, 99 nodes, 238 edges, 57 candidates, all 57 emitted at cap 100, zero deferred. No packet byte deferrals. The highest graph-wide incoming-edge count was candidate `d61cb9715e8e6ca08aec1c8035f7e6f0442e92e3847afe0d09fae63da7c94205`, site `site:854353ab6c24302ab51c183d8f83d0f654b15e623dffbdc7f04e54419aa2217e`, `(*Package).Build` → `sync.(*Once).Do`: 165 incoming edges to any of its 52 graph nodes, 51 distinct caller nodes, 6 candidate edges. This is also the graph-rich async/unresolved candidate already described in `milestone3-iteration3-audit.md`; it does **not** constitute an independent high-fan-in sample.

## Source/evidence distinction

The operation source location is Go module cache `golang.org/x/tools@v0.52.0/go/ssa/builder.go:3172`; `go list -f '{{.Dir}}' golang.org/x/tools/go/ssa` resolves `/Users/rob/go/pkg/mod/golang.org/x/tools@v0.52.0/go/ssa`. At lines 3148–3157, `Package.Build` schedules package builds, calls `wg.Add(1)` and waits with `wg.Wait`; at line 3172 `Build` calls `p.buildOnce.Do(p.build)`. These source facts establish an async/wait structure and a once-guarded build operation. The report's `candidate` edges remain candidate relationships, not established calls.

## Audit conclusion / limitation

The attempted edge count shows why a valid high-fan-in sample needs a site-centered definition (e.g., count distinct established callers directly targeting the operation's containing node, with candidate/unresolved relations tabulated separately). The existing candidate packet's graph membership is not an adequate fan-in metric: it folds internal topology and relationship kinds together. This check found no demonstrated source or analyzer defect and motivates no code/selection change. A true site-centered candidate-only/unresolved audit, prior byte-deferred sample, and count-deferred denominator remain outstanding; zero deferrals in this small 57-candidate scope cannot test deferred-candidate quality.
