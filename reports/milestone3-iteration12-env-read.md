# Milestone 3 source sample — package loader environment snapshot (iteration 12)

## Reproduction and deterministic selection

From the repository root:

```sh
go build -o /tmp/reffector-iter12 ./cmd/reffector
/tmp/reffector-iter12 -json -all -max-depth 100 . > /tmp/reffector-iter12-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
b = build(json.load(open('/tmp/reffector-iter12-effects.json')), max_candidates=100)
rows = []
for c in b['candidates']:
    s = c['operation']
    effects = set(s.get('effects', [])) | set(s.get('possible_effects', []))
    if 'env.read' in effects:
        rows.append((c['id'], s['id'], s['package'], s['function'], s['api'], s['location']))
print('counts:', b['counts'])
print('env.read candidates:', len(rows))
print(*sorted(rows), sep='\n')
PY
```

Inventory: 59 sites, 99 nodes, 238 edges, 57 candidates; all 57 packets emitted and 0 deferred. There are 4 `env.read` candidates (the label can overlap with other effects). Selected the lexicographically lowest stable candidate ID among the four: `candidate:30468cfc1474c7d65e45162f4f6a1cc04d114176e3064e1070e78f2843d3f59b`, site `site:293955da6befda80a58b04aee28e372d754fdc4c3031262ae75f1e844e22bc45`, `golang.org/x/tools/go/packages.newLoader` / `os.Environ`, reported line 749. This is a purposive, deterministic category sample, not a representative sample. The EffectReport did not resolve source paths; source was reviewed directly in the Go module cache at `golang.org/x/tools@v0.52.0/go/packages/packages.go`.

## Source observations

Inspected `packages.go:261-266,727-752`.

- **Mechanical facts:** `packages.Load` constructs a loader with `newLoader(cfg)` and passes the resulting config to `defaultDriver`. `newLoader` copies a non-nil config; only if the copied `Env` is nil does it populate that field with `os.Environ()`. A caller-supplied non-nil environment therefore avoids this specific environment snapshot. An empty-but-non-nil slice is still non-nil under this check.
- **Potential question / false lead:** The environment is passed to downstream driver configuration and can affect package loading; this source alone does not identify sensitive variables, disclose them, or establish a security boundary violation. Capturing ambient environment as a default is expected library behavior, not by itself a defect.
- **Hypothesis status:** No defect, selection-order change, or severity conclusion is justified by this single operation. It adds a distinct `env.read` category sample; no runtime or adversarial environment test was performed.
- **Unknowns:** Which environment entries the selected driver consumes and how external drivers handle the config require tracing beyond this bounded sample. Static review does not establish process-level behavior for every driver.

## Validation and limits

- `go build -o /tmp/reffector-iter12 ./cmd/reffector` — passed.
- `/tmp/reffector-iter12 -json -all -max-depth 100 .` — passed; inventory/counts above.
- Category scan over `build(..., max_candidates=100)` — completed; 4 `env.read` candidates.
- No code/tests changed, no model call, and no runtime experiment. This purposive source check does not close milestone 3's diversity/deferral gate.
