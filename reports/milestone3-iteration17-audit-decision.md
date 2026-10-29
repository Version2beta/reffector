# Milestone 3 bounded candidate-quality audit decision (iteration 17)

## Decision

**PASS: complete (bounded current-repo audit). No deterministic selection change is warranted by observed evidence.** This closes the explicitly bounded gate; it is not a defect-detection-rate, severity-ranking, or false-positive estimate.

## Reproducible inventory

From the repository root:

```sh
go build -o /tmp/reffector-iter17 ./cmd/reffector
/tmp/reffector-iter17 -json -all -max-depth 100 . > /tmp/reffector-iter17-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from collections import Counter
from investigate import build
r = json.load(open('/tmp/reffector-iter17-effects.json'))
b = build(r, max_candidates=100)
print('report', len(r['sites']), len(r['nodes']), len(r['edges']), r.get('truncated'))
print('counts', b['counts'])
print('status', Counter(c['packet_status'] for c in b['candidates']))
print('categories', Counter(x for c in b['candidates'] for x in c['categories']))
print('reduction', Counter(c['reduction_strategy'] for c in b['candidates']))
PY
```

Observed: EffectReport 59 sites, 99 nodes, 238 edges; all truncation flags false. Builder: 57 candidates, 57 emitted, 0 deferred. Categories overlap: cross-component 57, multi-frontier 52, branching/convergence 52, async 7, candidate/unresolved 6. Reduction strategies: 56 expanded, one progressive. This default current-checkout inventory does not exercise candidate-count or byte deferrals.

## Evidence synthesis and sample limits

The prior source reviews are purposive and substantially overlapping, not independent draws:

- `reports/milestone3-iteration3-audit.md` defines an explicit three-role sample: largest graph, first async+candidate/unresolved, and first process execution. It reports source-grounded false leads (literal `go env` invocation is not evidence of command injection; graph size/`candidate_vta` is not runtime reachability; cleanup relation does not itself prove invocation or leak).
- `reports/milestone3-iteration4-fanin-check.md` corrected the graph-wide fan-in interpretation; `reports/milestone3-iteration5-site-fanin.md` then matched all 57 sites to containing nodes and counted **direct established callers at those site functions**. Maximum was four distinct established callers; `(*Package).Build`/`sync.(*Once).Do` has three direct callers, not the earlier graph-wide 51. Its one candidate function-value incoming relation (`loadConfig$3`/`os.RemoveAll`) was source-checked as a closure/defer, not promoted to an established call.
- `reports/milestone3-iteration6-channel-close.md`, `reports/milestone3-iteration7-async-edge.md`, and `reports/milestone3-iteration9-skillinstall-write.md` through `reports/milestone3-iteration14-concurrency-atomic.md` record targeted effect-category source checks. They show legitimate operations and possible questions, not observed defects; iteration 7 grounds one established `go`/defer path and retains callback uncertainty.
- `reports/milestone3-iteration8-uncertain-incoming-inventory.md` scanned the candidate inventory; its sole uncertain incoming case is the same iteration-5 cleanup function-value relation, so it is not an independent source case. `reports/milestone3-iteration16-candidate-unresolved-sample.md` selects a second category example (lowest stable ID), but its category evidence is surrounding candidate VTA context, not a claim of an unresolved direct lock caller; source documents the intended lock/unlock.
- `reports/milestone3-iteration15-deferral-reduction-audit.md` audits all packet statuses and reduction metadata: all 57 emitted, no count/byte deferrals; the `(*Package).Build` packet progressively shrank 122,757 to 47,601 bytes under 48,000 and retained references for omitted evidence. These evidence omissions are packet reduction, not deferred candidates.

Thus the denominator is a complete current-repo inventory; the source sample is purposive and overlapping, including repeated checks of the same large graph and cleanup relation. It cannot establish representative defect-detection performance, prevalence, severity ordering, false-positive rate, candidate quality for omitted/deferred candidates, or selection behavior above 100. No source-manifest excerpts were available in these packet builds. The available rclone checkout remains different from pinned revision `81e7372f6fbedfa80592e9a564593d63a8dc32f3`; it was not substituted or analyzed as the pin.

## Selection decision and next gate

No reviewed candidate demonstrates a deterministic false selection or other specific ranking/selection error. The broad and overlapping labels, candidate-vs-established relation distinction, source-grounded benign examples, and packet reduction metadata provide enough evidence for the bounded decision not to change selection. The 100-candidate cap remains outside scope and untested; no candidate-count change is made.

Validation for this decision: iteration-17 build, analyzer and builder command above succeeded; observed output is recorded in this report. No code or tests changed; no model calls or runtime experiments. Next acceptance gate is milestone 4: use one or two reproducible adversarial provenance/CFG fixtures to test conservative handling of candidate-only relations or unsupported control claims, then document a narrow regression/fix or evidence-backed no-change conclusion.
