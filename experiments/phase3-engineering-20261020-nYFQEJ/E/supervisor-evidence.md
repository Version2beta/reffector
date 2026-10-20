# Supervisor evidence index — E

Exact tool calls and results remain in `events.jsonl`; this is a derived index.

## Tool calls

### 1. read; error=False; duration_ms=3
```json
{"path": "/Users/rob/.agents/skills/reffector/SKILL.md", "offset": null, "limit": null}
```

### 2. read; error=False; duration_ms=3
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/timing.json", "offset": null, "limit": null}
```

### 3. bash; error=False; duration_ms=69
```sh
pwd; git rev-parse HEAD; git status --porcelain; printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"; date -u '+%Y-%m-%dT%H:%M:%SZ'; shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version; GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help
```

### 4. bash; error=False; duration_ms=820
```sh
git log -18 --format='%H %s' -- internal/repository internal/backend/local; git ls-files '*AGENTS.md'; date +%s; GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect fs. -max-depth 0 -max-sites 25 -max-edges 30 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/survey.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/survey.stderr; printf 'analyzer_exit=%s\n' "$?"
```

### 5. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/survey.stderr", "offset": null, "limit": null}
```

### 6. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/survey.json", "offset": 1, "limit": 210}
```

### 7. bash; error=False; duration_ms=52
```sh
git show --stat 8baffc40273bb3aa4f6c7826d582ebe576f4c90c; git show --format=fuller --no-ext-diff 8baffc40273bb3aa4f6c7826d582ebe576f4c90c -- internal/repository; git log -8 --format='%H %s' -- internal/backend/local; git show --format=fuller --no-ext-diff 8baffc40273bb3aa4f6c7826d582ebe576f4c90c > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/historical.diff
```

### 8. write; error=False; duration_ms=2
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/candidates.md`

### 9. read; error=False; duration_ms=4
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/repository/key.go", "offset": 1, "limit": 280}
```

### 10. read; error=False; duration_ms=4
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/repository/repository.go", "offset": 130, "limit": 175}
```

### 11. bash; error=False; duration_ms=1184
```sh
date +%s; rg -n 'CiphertextLength|func .*Open|func .*LoadRaw|openKey\(|LoadUnpacked\(' internal/repository/crypto internal/repository/key.go internal/repository/repository.go internal/repository/repository_internal_test.go; GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -function LoadUnpacked -max-depth 3 -max-sites 15 -max-edges 60 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/loadunpacked.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/loadunpacked.stderr; printf 'analyzer_exit=%s\n' "$?"; GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -function openKey -max-depth 3 -max-sites 20 -max-edges 60 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/openkey.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/openkey.stderr; printf 'analyzer_exit=%s\n' "$?"
```

### 12. bash; error=False; duration_ms=30
```sh
python3 - <<'PY'
import json
from pathlib import Path
p=Path('/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E')
for name in ['loadunpacked','openkey']:
 d=json.loads((p/(name+'.json')).read_text()); print('\n',name,'keys',d.keys()); print('stats',(p/(name+'.stderr')).read_text());
 for s in d.get('sites',[]): print(s['id'], s.get('function'),s.get('api'), s.get('location'),s.get('uncertainty_reasons'))
 print('reverse',json.dumps(d.get('reverse',d.get('graph',{})))[:10000]); print('tail',json.dumps(d)[-1800:])
PY
```

### 13. read; error=False; duration_ms=2
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/repository/raw.go", "offset": null, "limit": null}
```

### 14. read; error=False; duration_ms=2
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/repository/crypto/buffer.go", "offset": null, "limit": null}
```

### 15. read; error=False; duration_ms=2
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/repository/crypto/crypto.go", "offset": 230, "limit": 135}
```

### 16. bash; error=False; duration_ms=1119
```sh
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -site site:2dad08db8468a86e9929ce0b2a30fe12e8900ba02af1715e96b1cc4cd5c1cb3e -max-depth 7 -max-sites 1 -max-edges 100 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/cache-callers.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/cache-callers.stderr; printf 'analyzer_exit=%s\n' "$?"; GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -reason interface_dispatch -function LoadRaw -max-depth 2 -max-sites 8 -max-edges 40 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/raw-boundaries.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/raw-boundaries.stderr; printf 'analyzer_exit=%s\n' "$?"; git merge-base --is-ancestor 8baffc40273bb3aa4f6c7826d582ebe576f4c90c HEAD; printf 'ancestor_exit=%s\n' "$?"; date +%s
```

### 17. bash; error=False; duration_ms=61
```sh
python3 - <<'PY'
import json
from pathlib import Path
p=Path('/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E')
for n in ['survey','cache-callers','raw-boundaries']:
 d=json.loads((p/(n+'.json')).read_text()); print(n,'truncated',d['truncated'],'coverage',d['coverage'])
 if n=='survey': continue
 for s in d['sites']: print('SITE',s['id'],s['api'],s['location'],s.get('uncertainty_reasons'))
 for e in d['edges']:
  s=e.get('source',{}); print(s.get('caller'),'=>',s.get('callee'),s.get('location'))
PY
rg -n 'ivSize|macSize|Extension|NonceSize|func .*KDF|func Test.*(Key|Config|Unpacked)|searchKey\(' internal/repository/crypto/crypto.go internal/repository/crypto/kdf.go internal/repository/key_test.go internal/repository/repository_test.go internal/repository/repository_internal_test.go internal/repository/config.go internal/repository/repository.go; git show 8baffc40273bb3aa4f6c7826d582ebe576f4c90c:changelog/unreleased/pull-21977; rg -n 'func .*Load|func .*openReader|OpenFile|Open\(' internal/backend/local/local.go; date +%s
```

### 18. read; error=False; duration_ms=2
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/repository/repository_internal_test.go", "offset": 440, "limit": 195}
```

### 19. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/repository/repository_test.go", "offset": 330, "limit": 115}
```

### 20. read; error=False; duration_ms=2
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/repository/repository.go", "offset": 850, "limit": 85}
```

### 21. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/backend/local/local.go", "offset": 200, "limit": 81}
```

### 22. read; error=False; duration_ms=2
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/repository/crypto/kdf.go", "offset": 1, "limit": 110}
```

### 23. read; error=False; duration_ms=2
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/backend/cache/file.go", "offset": 120, "limit": 46}
```

### 24. bash; error=False; duration_ms=49
```sh
rg -n 'canBeCached|func DefaultLoad|func LoadConfig|Test.*(SearchKey|Key)' internal/backend/cache internal/backend/util internal/restic/config.go internal/repository --glob '*test.go'; rg -n 'canBeCached|func DefaultLoad|func LoadConfig' internal/backend/cache internal/backend/util internal/restic; date +%s; git diff --quiet; printf 'diff_exit=%s\n' "$?"; git status --porcelain
```

### 25. read; error=False; duration_ms=3
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/backend/cache/file.go", "offset": 1, "limit": 36}
```

### 26. read; error=False; duration_ms=3
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/backend/util/defaults.go", "offset": 1, "limit": 90}
```

### 27. read; error=False; duration_ms=2
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/restic/config.go", "offset": 56, "limit": 72}
```

### 28. bash; error=False; duration_ms=685
```sh
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -operation os.Open -package internal/backend/local -max-depth 3 -max-sites 5 -max-edges 25 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/local-read.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/local-read.stderr; printf 'analyzer_exit=%s\n' "$?"; { printf 'cwd='; pwd; git rev-parse HEAD; git status --porcelain; printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"; shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version; GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help; } > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/provenance.txt 2>&1; date +%s
```

### 29. bash; error=False; duration_ms=74
```sh
python3 - <<'PY'
import json
from pathlib import Path
p=Path('/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E')
d=json.loads((p/'local-read.json').read_text()); print('coverage',d['coverage'],'truncated',d['truncated'])
for s in d['sites']: print(s['id'],s['api'],s['function'],s['location'])
for e in d['edges']: print(e.get('source'))
PY
rg -n 'cacheLayoutPaths' internal/backend/cache; git show '8baffc40273bb3aa4f6c7826d582ebe576f4c90c^:internal/repository/crypto/crypto.go' | python3 -c 'import sys; a=sys.stdin.readlines(); print("".join(f"{i+1}: {s}" for i,s in enumerate(a) if i<27 or 205<=i<218 or 284<=i<312))'; date +%s
```

### 30. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/restic/internal/backend/cache/cache.go", "offset": 35, "limit": 23}
```

### 31. write; error=False; duration_ms=1
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/candidates.md`

### 32. write; error=False; duration_ms=1
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/commands.md`

### 33. bash; error=False; duration_ms=88
```sh
date -u '+%Y-%m-%dT%H:%M:%SZ'; date +%s; git rev-parse HEAD; git status --porcelain; shasum -a 256 /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/historical.diff; python3 - <<'PY'
import json
from pathlib import Path
p=Path('/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E')
for name in ['survey','loadunpacked','openkey','cache-callers','raw-boundaries','local-read']:
 d=json.loads((p/(name+'.json')).read_text())
 print(name,'sites',len(d['sites']),'edges',len(d['edges']),'truncation',d['truncated'])
PY
```

### 34. write; error=False; duration_ms=1
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/report.md`

### 35. bash; error=False; duration_ms=72
```sh
python3 - <<'PY'
import json,time
from pathlib import Path
p=Path('/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E')
t=json.loads((p/'timing.json').read_text()); now=time.time(); result={'completed_epoch':now,'elapsed_seconds':round(now-t['start_epoch'],3),'before_investigation_deadline':now<t['investigation_deadline_epoch'],'before_report_deadline':now<t['report_deadline_epoch'],'status':'completed_read_only_source_review','runtime_tests_executed':False,'report_words':len((p/'report.md').read_text().split())}; (p/'completion.json').write_text(json.dumps(result,indent=2)+'\n'); print(json.dumps(result))
PY
git status --porcelain
```

## Analyzer artifact coverage

### `cache-callers.json`
```json
{
  "coverage": {
    "packages_selected": 6,
    "source_functions": 446,
    "operation_sites_identified": 51,
    "selected_operation_sites": 38,
    "dependency_operation_sites": 13,
    "unresolved_sites_identified": 1021,
    "sites_matching_filters": 1,
    "sites_emitted": 1
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": true
  },
  "emitted_sites": 1,
  "emitted_unresolved": 0
}
```

### `loadunpacked.json`
```json
{
  "coverage": {
    "packages_selected": 6,
    "source_functions": 446,
    "operation_sites_identified": 51,
    "selected_operation_sites": 38,
    "dependency_operation_sites": 13,
    "unresolved_sites_identified": 1021,
    "sites_matching_filters": 1,
    "sites_emitted": 1
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": false
  },
  "emitted_sites": 1,
  "emitted_unresolved": 0
}
```

### `local-read.json`
```json
{
  "coverage": {
    "packages_selected": 6,
    "source_functions": 446,
    "operation_sites_identified": 51,
    "selected_operation_sites": 38,
    "dependency_operation_sites": 13,
    "unresolved_sites_identified": 1021,
    "sites_matching_filters": 4,
    "sites_emitted": 4
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": false
  },
  "emitted_sites": 4,
  "emitted_unresolved": 0
}
```

### `openkey.json`
```json
{
  "coverage": {
    "packages_selected": 6,
    "source_functions": 446,
    "operation_sites_identified": 51,
    "selected_operation_sites": 38,
    "dependency_operation_sites": 13,
    "unresolved_sites_identified": 1021,
    "sites_matching_filters": 109,
    "sites_emitted": 20
  },
  "truncated": {
    "sites": true,
    "edges": false,
    "depth": true
  },
  "emitted_sites": 20,
  "emitted_unresolved": 20
}
```

### `raw-boundaries.json`
```json
{
  "coverage": {
    "packages_selected": 6,
    "source_functions": 446,
    "operation_sites_identified": 51,
    "selected_operation_sites": 38,
    "dependency_operation_sites": 13,
    "unresolved_sites_identified": 1021,
    "sites_matching_filters": 1,
    "sites_emitted": 1
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": false
  },
  "emitted_sites": 1,
  "emitted_unresolved": 1
}
```

### `survey.json`
```json
{
  "coverage": {
    "packages_selected": 6,
    "source_functions": 446,
    "operation_sites_identified": 51,
    "selected_operation_sites": 38,
    "dependency_operation_sites": 13,
    "unresolved_sites_identified": 1021,
    "sites_matching_filters": 33,
    "sites_emitted": 25
  },
  "truncated": {
    "sites": true,
    "edges": false,
    "depth": true
  },
  "emitted_sites": 25,
  "emitted_unresolved": 0
}
```

