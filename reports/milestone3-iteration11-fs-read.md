# Milestone 3 source sample — managed skill read (iteration 11)

## Reproduction and deterministic selection

From repository root, the inventory was regenerated with:

```sh
go build -o /tmp/reffector-iter11 ./cmd/reffector
/tmp/reffector-iter11 -json -all -max-depth 100 . > /tmp/reffector-iter11-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
b = build(json.load(open('/tmp/reffector-iter11-effects.json')), max_candidates=100)
print('counts:', b['counts'])
rows = []
for c in b['candidates']:
    s = c['operation']
    effects = set(s.get('effects', [])) | set(s.get('possible_effects', []))
    if 'fs.read' in effects:
        rows.append((c['id'], s['id'], s['package'], s['function'], s['api'], s['location']))
print('fs.read candidates:', len(rows))
print(*sorted(rows), sep='\n')
PY
```

Counts: 59 sites, 99 nodes, 238 edges; 57 candidates, all emitted, 0 deferred. There were 9 `fs.read` candidates. Selection was the lexicographically lowest stable candidate ID among those: `candidate:30e6fc25eb2b4ea28859312d6f365713a459ac5e8f64727fe3a6c11d93b887c1`, site `site:79102a0ebd0cf3eddbf9e9cc1dd220fa71000e10b4f8b0f7a49748df7cb07c84`, `internal/skillinstall.managedCopy` / `os.ReadFile`, reported line 61. This is a deterministic purposive category sample, not a representative sample; effect labels may overlap. Source file paths were unavailable in the EffectReport because no source root/manifest was supplied, so source was checked directly in the current checkout.

## Source observations

Inspected `internal/skillinstall/install.go:27-66`.

- **Mechanical facts:** `Install` reads the existing skill file. If it exists and differs from desired content, it calls `managedCopy` with the state-marker path and existing bytes. `managedCopy` reads the marker, treats a missing marker as unmanaged, wraps other read errors, and compares the trimmed marker content to SHA-256 of the current skill bytes. Install refuses to replace content not matching the marker. A matching file skips the content write; the marker is then written/refreshed with the desired-content digest and mode `0600`.
- **Potential question / false lead:** The marker is an integrity/ownership record, not cryptographic authentication; it is locally writable. This code's guard alone does not establish safety against a malicious writer able to alter both files. Conversely, that threat model does not prove an exploitable defect. File replacement and marker update are separate writes, so interruption can leave mismatched state; that is a possible recovery/partial-completion question, not evidence of corruption or a bug.
- **Hypothesis status:** No defect demonstrated and no candidate selection/order change justified. This sample adds an `fs.read` operation to the category audit, although it shares `Install` with previously sampled writes.
- **Unknowns:** No concurrent installation, filesystem permission/symlink, crash/recovery, or adversarial modification test was performed. Static inspection does not prove runtime behavior under those conditions.

## Validation and limits

- `go build -o /tmp/reffector-iter11 ./cmd/reffector` — passed.
- `/tmp/reffector-iter11 -json -all -max-depth 100 .` — passed; inventory and counts recorded above.
- `go test ./internal/skillinstall` — passed (cached).
- No code/tests changed, no model call, and no runtime experiment. This single purposive source check does not close milestone 3's diversity/deferral gate.
