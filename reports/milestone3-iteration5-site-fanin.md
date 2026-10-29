# Milestone 3 follow-up — site-centered fan-in (iteration 5)

## Reproduction and denominator

From the repository root:

```sh
go build -o /tmp/reffector-iter5 ./cmd/reffector
/tmp/reffector-iter5 -json -all -max-depth 100 . > /tmp/reffector-iter5-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
r = json.load(open('/tmp/reffector-iter5-effects.json'))
b = build(r, max_candidates=100)
# For each candidate, match graph nodes by the operation site's exact
# package/function, then classify edges targeting those nodes by edge metadata.
for c in b['candidates']:
    s = c['operation']
    targets = {n['id'] for n in r['nodes']
               if n['package'] == s['package'] and n['function'] == s['function']}
    incoming = [e for e in r['edges'] if e['callee'] in targets]
    established = [e for e in incoming if not e['candidate'] and
                   (e.get('source') or {}).get('resolution') == 'established']
    callers = {e['caller'] for e in established}
    uncertain = [e for e in incoming if e['candidate'] or
                 (e.get('source') or {}).get('resolution') != 'established']
PY
```

Analyzer and builder completed successfully. EffectReport: 59 sites, 99 nodes, 238 edges; builder at cap 100: 57 candidates, 57 emitted, 0 deferred. Thus every candidate site matched a containing graph node (57/57); none had a count- or byte-deferred packet in this run. Candidate selection is an analyzer-produced purposive inventory, not a random sample.

Fan-in here means distinct caller nodes on established incoming edges to a node whose package and function exactly match the effect site's package and function. Candidate and non-established incoming edge relations are counted separately; they are not included as established callers. Maximum direct established fan-in was 4 distinct callers. Three candidates had that count; the first by descending established-edge count then stable candidate ID was `sync/atomic.(*Bool).Load` in `(*task).isTransitivelyDone` (4 distinct callers, 5 established incoming edges). `(*Package).Build` / `sync.(*Once).Do`, previously reported as graph-wide large, has 3 distinct established callers and 3 incoming edges at the containing node, demonstrating why its former 51-caller graph-wide figure was not site fan-in.

Exactly one of 57 candidates had a candidate/non-established incoming edge at the containing node: candidate `candidate:3e6912d4ffc8d1c4921236f8340e8003057cab19a109be352fb707e9aa01493f`, `loadConfig$3` / `os.RemoveAll` (site `site:caabda1e10a0a2a60bb7e87599f7085384e6c85afe1e6891e1b604d234273737`). It has one candidate edge marked `candidate_function_value`, from `AnalyzeReport`; there are zero direct established incoming callers for this operation node. It is not a demonstrated call edge.

## Source check and limits

In `internal/analyzer/loading.go:78`, `loadConfig` defines the closure `cleanup := func() { _ = os.RemoveAll(tmp) }`. In `internal/analyzer/analyzer.go:143-144`, `AnalyzeReport` receives that cleanup function and executes `defer cleanup()` before package loading. This source grounds the cleanup behavior and shows an indirect function-value/defer path. The graph's candidate relation alone does not prove the call; source supplies the control-flow fact. The effect's source location in generated JSON is unavailable because the analyzer was run without a source manifest/root, so references use repository file/line locations from direct source inspection instead.

This check found candidate-only incoming relationships as a distinct case but does not establish a general unresolved-path sample: its one uncertain direct incoming relation is the candidate function-value edge just described, and no separate non-candidate unresolved-resolution edge was found among these 57 matched sites. No candidates were byte- or count-deferred, so prior-deferred candidate quality and cap behavior remain unevaluated. Fan-in counts are a static graph measure and not runtime reachability or vulnerability severity. No selection change is supported by this sample.
