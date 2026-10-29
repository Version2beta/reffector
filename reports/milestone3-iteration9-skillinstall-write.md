# Milestone 3 source sample — skill installer state write (iteration 9)

## Reproduction and selection

From the repository root, run:

```sh
go build -o /tmp/reffector-iter9 ./cmd/reffector
/tmp/reffector-iter9 -json -all -max-depth 100 . > /tmp/reffector-iter9-effects.json
PYTHONPATH=tools/effect_investigation python3 - <<'PY'
import json
from investigate import build
b = build(json.load(open('/tmp/reffector-iter9-effects.json')), max_candidates=100)
rows = []
for c in b['candidates']:
    s = c['operation']
    effects = set(s.get('effects', [])) | set(s.get('possible_effects', []))
    if s['package'].endswith('/internal/skillinstall') and effects & {'fs.create', 'fs.write'}:
        rows.append((c['id'], s['id'], s['api'], s['location']['line']))
print(b['counts'])
print('matching candidates:', len(rows))
print(*sorted(rows), sep='\n')
PY
```

Observed denominator: EffectReport had 59 sites, 99 nodes, and 238 edges; the builder found 57 candidates, emitted 57 packets, and deferred none. Selection rule: among candidates in `internal/skillinstall` with `fs.create` or `fs.write`, select the lexicographically lowest stable candidate ID. There were 3 matches: two `os.WriteFile` sites and one `os.MkdirAll`; the selected record is candidate `candidate:0e7b553cf964bd941b8c63d11b376eaa6a0a038ca85e62af9f0db66229887670`, site `site:338c89c8bb18cbc6736841f6f78008ba0a12167059de69945be48c3df3aef493`, `Install` / `os.WriteFile`, line 54. This is a deterministic purposive sample, not random or representative.

## Source observations

Current source: `internal/skillinstall/install.go:25-57,60-68`.

- **Mechanical facts:** `Install` rejects empty home/name/content; constructs `SKILL.md` and `.reffector-sha256` beneath the selected skill directory; creates the directory; reads an existing skill; and, if content differs, requires `managedCopy` to match the old content's SHA-256 before overwriting. It writes the state file after the skill write (or after skipping an unchanged skill write), using `os.WriteFile(..., 0600)`, and returns an error if that metadata write fails. `managedCopy` treats a missing state file as unmanaged and compares trimmed stored text to the digest.
- **Potential correctness question:** if the content write succeeds but the subsequent state write fails, the operation returns an error after changing `SKILL.md`; callers may observe a partially completed installation. Whether this requires rollback/recovery depends on caller expectations and error handling. This sequence alone does not establish a bug.
- **False lead / counterevidence:** the existing-content guard is specifically intended to refuse overwriting unmanaged or modified content; the state marker is updated only after the content write succeeds. Avoid concluding from `os.WriteFile` alone that arbitrary files are overwritten: the generated paths and the preceding guard are relevant source context.
- **Unknowns:** no fault-injection run was done for a state-file write failure; no filesystem concurrency, symlink/path attack, crash consistency, or caller behavior was evaluated. Analyzer source locations identify the operation but do not prove its runtime success or reachability.

No selection change is warranted by this single source sample. No model call or additional runtime experiment was made.

## Validation

- `go build -o /tmp/reffector-iter9 ./cmd/reffector` — passed.
- `/tmp/reffector-iter9 -json -all -max-depth 100 .` — passed; report/builder counts above.
- `go test ./internal/skillinstall` — passed (`ok`, cached).
