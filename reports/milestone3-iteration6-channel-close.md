# Milestone 3 source check — channel-close candidate (iteration 6)

## Reproduction and denominator

From repository root:

```sh
go build -o /tmp/reffector-iter6 ./cmd/reffector
/tmp/reffector-iter6 -json -all -max-depth 100 . > /tmp/reffector-iter6-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
r = json.load(open('/tmp/reffector-iter6-effects.json'))
b = build(r, max_candidates=100)
print(len(r['sites']), len(r['nodes']), len(r['edges']),
      len(b['candidates']), len(b['emitted']), len(b['deferred']))
for c in b['candidates']:
    if c['operation'].get('effects') == ['channel.close']:
        print(c['id'], c['operation'])
PY
```

Build and analyzer succeeded. Denominator is 59 sites, 99 nodes, 238 edges; 57 candidates selected at cap 100, all 57 emitted, 0 deferred. This inventory has one `channel.close` candidate: `candidate:7e0e418708736a0b8a047e3aaf63a2a168ff7a968169af0e85b53f95190c093b`, site `(*task).markDone` / `builtin.close` in `golang.org/x/tools/go/ssa`.

## Source facts vs interpretation

Pinned dependency version from module cache: `golang.org/x/tools@v0.52.0`. At `go/ssa/task.go:22-25`, `task.done` is a channel with a comment stating it is closed when the task is done. At `:51-55`, `markDone` conditionally closes that channel only for non-nil receiver. At `:57-63`, `wait` returns immediately when `isTransitivelyDone()` is true; otherwise it waits for done notifications. At `:31-44`, `addEdge` rejects self/already-done tasks and comments that calls must precede `markDone`.

These source facts support the mechanical classification as channel close and explain a synchronization role. The safety invariant involving concurrent `addEdge`/`markDone` operations is a potential investigation question, not evidence of a race or defect. Static effect detection does not prove runtime ordering; no runtime experiment or defect claim was made. This case is an additional effect category beyond the previously sampled process execution and cleanup cases, but remains a purposive source check, not a representative sample. Inventory still contains no count- or byte-deferred cases, and no selection change is justified.
