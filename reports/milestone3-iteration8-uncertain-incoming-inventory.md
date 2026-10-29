# Milestone 3 inventory check — uncertain incoming relations (iteration 8)

## Reproduction and observed denominator

From the repository root:

```sh
go build -o /tmp/reffector-iter8 ./cmd/reffector
/tmp/reffector-iter8 -json -all -max-depth 100 . > /tmp/reffector-iter8-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
r = json.load(open('/tmp/reffector-iter8-effects.json'))
b = build(r, max_candidates=100)
print(b['counts'])
# For each emitted candidate, match exact package/function nodes and inspect
# their incoming graph edges. Candidate or non-established relations are
# explicitly not treated as established calls.
for c in b['candidates']:
    s = c['operation']
    targets = {n['id'] for n in r['nodes']
               if n['package'] == s['package'] and n['function'] == s['function']}
    incoming = [e for e in r['edges'] if e['callee'] in targets]
    uncertain = [e for e in incoming if e['candidate'] or
                 (e.get('source') or {}).get('resolution') != 'established']
    if uncertain:
        print(c['id'], s['id'], s['function'], s['api'],
              s.get('effects', s.get('possible_effects')), len(uncertain))
PY
```

Build and analyzer succeeded. Counts were 59 sites, 99 nodes, 238 edges, 57 candidates, 57 packets emitted, 0 deferred. In the 57 matched candidate-site containing functions, exactly one candidate had any candidate/non-established incoming relation; no *distinct* candidate with such a relation was present. This is a complete inventory check for this current-repo run, not a random sample, and does not cover candidates whose containing node cannot be matched by exact package/function (none were unmatched in the prior iteration 5 audit; that matching procedure was repeated here).

The sole relation repeats iteration 5's already audited candidate: `candidate:3e6912d4ffc8d1c4921236f8340e8003057cab19a109be352fb707e9aa01493f`, site `site:caabda1e10a0a2a60bb7e87599f7085384e6c85afe1e6891e1b604d234273737`, `internal/analyzer.loadConfig$3` / `os.RemoveAll`, effect `fs.remove`. Its relation is a candidate function-value edge from `AnalyzeReport`; it is not promoted to an established call edge. Source was previously inspected at `internal/analyzer/loading.go:78` (closure definition) and `internal/analyzer/analyzer.go:143-144` (deferred cleanup invocation); see `reports/milestone3-iteration5-site-fanin.md` for the source interpretation.

## Interpretation and limits

This run provides negative inventory evidence for the requested distinct candidate-only/unresolved incoming case; it does not create a new source sample. It neither demonstrates general unresolved incoming-path coverage nor audits provenance/SSA unresolved edges outside graph incoming relations. All 57 candidates emitted, so this run contains no byte/count-deferred candidate. The builder remains at cap 100. No selection change or defect inference follows.

A first ad-hoc display attempt assumed a nonexistent operation `effect` key and failed; a corrected query used `effects`/`possible_effects`. A subsequent diagnostic assumed an edge `kind` key and failed after printing the sole match; the final reproduction above avoids both schema assumptions. The build/analyzer/builder succeeded and the final scan completed.
