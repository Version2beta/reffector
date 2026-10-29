# Milestone 3 deferral and reduction inventory audit (iteration 15)

## Reproduction

From the repository root, with the existing default packet byte limit and explicit candidate cap:

```sh
go build -o /tmp/reffector-iter15 ./cmd/reffector
/tmp/reffector-iter15 -json -all -max-depth 100 . > /tmp/reffector-iter15-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from collections import Counter
from investigate import build
b = build(json.load(open('/tmp/reffector-iter15-effects.json')), max_candidates=100)
print(b['counts'])
print(Counter(c['packet_status'] for c in b['candidates']))
print(Counter(c['reduction_strategy'] for c in b['candidates']))
print(Counter(label for c in b['candidates'] for label in c['categories']))
PY
```

Build, analyzer, and builder completed successfully. Current report denominator: 59 sites, 99 nodes, 238 edges, 57 candidates, 57 emitted packets, 0 deferred packets. Builder limits: `max_candidates=100`, `max_packet_bytes=48000`; selection is deterministic candidate-ID order and does not vary with packet size.

## Complete inventory results

- Packet statuses: 57 `emitted`; no `deferred_max_candidates` and no `deferred_over_budget`. Thus there are no currently deferred candidate rows to source-audit or count by category. This result is scoped to this 57-candidate current-repository inventory and these builder limits, not a claim that deferrals are impossible.
- Reduction strategy: 56 `expanded`; 1 `progressive`. All 57 have a non-null final packet size; none has a minimum-packet size recorded for failure.
- Category counts (overlapping labels, denominator 57): `cross-component-shared-operation-site` 57, `multi-frontier-reachability` 52, `branching-convergence` 52, `async-edges` 7, and `candidate-unresolved-relationships` 6. Every category member in this run was emitted. These are label counts, not a disjoint partition; no category-specific deferral was observed.
- Expanded packet sizes ranged from 4,935 to 122,757 bytes. The sole progressive case is the already audited `(*Package).Build` / `sync.(*Once).Do` candidate `candidate:d61cb9715e8e6ca08aec1c8035f7e6f0442e92e3847afe0d09fae63da7c94205`, site `site:854353ab6c24302ab51c183d8f83d0f654b15e623dffbdc7f04e54419aa2217e`. Its packet reduced from 122,757 to 47,601 bytes (under the 48,000-byte cap), with strategy `progressive` and steps `minimum_investigation_packet`, `caller_relationships`, `extended_audit_evidence`. It emitted; this is successful reduction, not a deferred candidate.
- Reduction report for that packet says 40/52 node refs and 72/165 relationship refs are represented in the packet, leaving 12 node refs and 93 relationship refs explicitly deferred to the canonical evidence store; 6 unresolved edges are indicated. Source excerpts were not included and 53 source items were unavailable because a source root/manifest was not supplied. No analyzer truncation is reported. These packet-level omissions are tracked reduction/retrieval references and must not be mistaken for candidate packet deferral.

## Conclusion and limitations

This completes the bounded status/metadata scan requested for iteration 15. It verifies no over-budget or candidate-count deferrals under this exact run, while identifying one successfully reduced packet and its recorded evidence omissions. It provides no source coverage of deferred candidates (there are none), does not exercise the 100-candidate cap, and does not establish behavior above current inventory or under different limits. No selection/cap changes are justified by these data. No code or tests were changed; no model calls or runtime defect experiments were made.
