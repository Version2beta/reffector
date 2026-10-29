# Milestone 3 source check — established async edge (iteration 7)

## Reproduction and denominator

From the repository root:

```sh
go build -o /tmp/reffector-iter7 ./cmd/reffector
/tmp/reffector-iter7 -json -all -max-depth 100 . > /tmp/reffector-iter7-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
r = json.load(open('/tmp/reffector-iter7-effects.json'))
b = build(r, max_candidates=100)
print(b['counts'])
for c in b['candidates']:
    if c['site_id'] == 'site:e11cf2fee919ccbd0dda8d1545b5ff8635073336b64a1b6a5aaf77f24a57507b':
        print(c['id'], c['operation'], c['categories'])
PY
```

Observed counts: 59 operation sites, 99 nodes, 238 edges, 57 candidates, 57 packets emitted, 0 deferred. The selected candidate is `candidate:3e9122ff83620e7d384964d82cb46bfc936ad9a1864ba6cb6e047535d4dffbbb`, effect site `site:e11cf2fee919ccbd0dda8d1545b5ff8635073336b64a1b6a5aaf77f24a57507b`, in `golang.org/x/sync/errgroup.(*Group).Go$1`, classified `concurrency.wait` and `async-edges`. This is a deterministic, targeted source check of one candidate in the complete current-repo inventory, not a representative sample.

## Established relation and source facts

The analyzer's relation `edge:6c689fe80e485827972f3de038f0431a1a9a7391349d814089f60f4324b8617d` is an **established, non-candidate** `go` edge from `(*Group).Go` to `(*Group).Go$1` at the pinned module-cache source `golang.org/x/sync@v0.24.0/errgroup/errgroup.go:78`. The closure's established `defer` relation to `(*Group).done` is `edge:2d0ee2ac81a43c7ca1ca4c7f53afe68b60ec0f5ea6763c692b0904704b0f789d` at line 79. The candidate effect site is a separate operation in that closure: `sync.(*Once).Do` at line 94.

Source lines 72-97 show `Go` optionally sends a semaphore token, increments the wait group, launches the closure, defers `g.done()`, invokes `f()`, and on non-nil error uses `g.errOnce.Do` to store the first error and optionally cancel the context. The effect site inventory marks the callback as uncertain (`uncertainty_reasons: ["callback"]`). These facts confirm that an established async edge reaches the function containing the candidate effect; they do not by themselves establish a defect. No ordering, race, or runtime behavior claim is made. No selection change is warranted by this single check.

## Validation and limits

`go build -o /tmp/reffector-iter7 ./cmd/reffector` succeeded; `/tmp/reffector-iter7 -json -all -max-depth 100 .` succeeded; Python builder produced the counts above. An initial inspection snippet used a nonexistent `emitted` result key and raised `KeyError`; corrected inspection used documented `counts` and completed. No code/tests changed, no model experiment or runtime test performed. This adds async source evidence but does not audit byte/count deferrals, unresolved non-candidate paths, or selection-cap behavior.
