# Supervisor evidence index — D

Exact tool calls and results remain in `events.jsonl`; this is a derived index.

## Tool calls

### 1. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/.agents/skills/reffector/SKILL.md", "offset": null, "limit": null}
```

### 2. bash; error=False; duration_ms=83
```sh
pwd; git rev-parse HEAD; git status --porcelain; printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"; shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; go version; go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help
```

### 3. read; error=False; duration_ms=3
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/timing.json", "offset": null, "limit": null}
```

### 4. bash; error=False; duration_ms=32
```sh
find . -name AGENTS.md -print; ls internal/backend/local; ls internal/repository; date -u +%Y-%m-%dT%H:%M:%SZ
```

### 5. bash; error=False; duration_ms=557
```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect fs. -max-depth 0 -max-sites 24 -max-edges 50 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/survey.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/survey.stderr; printf 'analyzer_exit=%s\n' "$?"; date -u +%Y-%m-%dT%H:%M:%SZ
```

### 6. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/survey.json", "offset": 1, "limit": 220}
```

### 7. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/survey.stderr", "offset": null, "limit": null}
```

### 8. read; error=False; duration_ms=1
```json
{"path": "internal/backend/local/local.go", "offset": 1, "limit": 340}
```

### 9. bash; error=False; duration_ms=31
```sh
python3 - <<'PY'
import json
p='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/survey.json'
d=json.load(open(p));print('top keys',d.keys());print('truncation',[(k,v) for k,v in d.items() if 'trunc' in k or 'coverage' in k]);
for s in d['sites']:
 if '/local' in s['package']:print(s['id'],s['function'],s['api'],s['location'],s.get('uncertain'))
PY
rg -n 'Save|tempFile|ENOSPC|Sync|Rename|retry|Retry|Atomic|Length|short' internal/backend/local/*test.go internal/backend/backend.go internal/backend/retry* internal/repository/upgrade_repo*; date -u +%Y-%m-%dT%H:%M:%SZ
```

### 10. write; error=False; duration_ms=2
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/candidates.md`

### 11. bash; error=False; duration_ms=656
```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -site site:9a8cbcbfba99c06fc0ee41fa4c19d4b53c29a2e634c66fca321f022a51a01a3c -max-depth 3 -max-sites 4 -max-edges 40 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/rename.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/rename.stderr; printf 'rename_exit=%s\n' "$?"
```

### 12. read; error=False; duration_ms=6
```json
{"path": "internal/backend/local/local_internal_test.go", "offset": null, "limit": null}
```

### 13. read; error=False; duration_ms=5
```json
{"path": "internal/backend/local/local_test.go", "offset": 1, "limit": 300}
```

### 14. read; error=False; duration_ms=7
```json
{"path": "internal/backend/local/local_unix.go", "offset": null, "limit": null}
```

### 15. read; error=False; duration_ms=7
```json
{"path": "internal/backend/retry/backend_retry.go", "offset": 97, "limit": 102}
```

### 16. read; error=False; duration_ms=7
```json
{"path": "internal/repository/upgrade_repo.go", "offset": 1, "limit": 160}
```

### 17. bash; error=False; duration_ms=49
```sh
python3 - <<'PY'
import json
p='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/rename.json';d=json.load(open(p));print(json.dumps(d,indent=2))
PY
rg -n 'func .*Test|Atomic|Save|Error|Reader|Replace|Overwrite' internal/backend/test/*go | head -140
rg -n 'func .*Test|UpgradeRepo' internal/repository/upgrade_repo_test.go
ls internal/backend/*reader*; date -u +%Y-%m-%dT%H:%M:%SZ
```

### 18. read; error=False; duration_ms=1
```json
{"path": "internal/backend/test/tests.go", "offset": 475, "limit": 175}
```

### 19. read; error=False; duration_ms=2
```json
{"path": "internal/backend/retry/backend_retry_test.go", "offset": 1, "limit": 96}
```

### 20. read; error=False; duration_ms=1
```json
{"path": "internal/repository/upgrade_repo_test.go", "offset": 1, "limit": 180}
```

### 21. read; error=False; duration_ms=1
```json
{"path": "internal/backend/backend.go", "offset": 1, "limit": 110}
```

### 22. read; error=False; duration_ms=1
```json
{"path": "internal/backend/rewind_reader.go", "offset": 1, "limit": 170}
```

### 23. read; error=False; duration_ms=1
```json
{"path": "internal/backend/local/local_windows.go", "offset": 1, "limit": 150}
```

### 24. bash; error=False; duration_ms=1225
```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect fs. -package /internal/backend/local -function Save -max-depth 2 -max-sites 12 -max-edges 30 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/save-effects.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/save-effects.stderr; printf 'save_effects_exit=%s\n' "$?"; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -reason interface_dispatch -function Save -max-depth 1 -max-sites 10 -max-edges 25 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/save-unresolved.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/save-unresolved.stderr; printf 'save_unresolved_exit=%s\n' "$?"; date -u +%Y-%m-%dT%H:%M:%SZ
```

### 25. bash; error=False; duration_ms=73
```sh
python3 - <<'PY'
import json
base='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/'
for f in ['save-effects.json','save-unresolved.json']:
 d=json.load(open(base+f)); print('\n',f,'coverage',d['coverage'],'limits',d['truncated'])
 for s in d['sites']+d['unresolved_sites']: print(s['id'],s['function'],s.get('api'),s['location'],s.get('uncertainty_reasons'))
 for r in d['reverse']:print('reverse',r['site'], 'nodes',r['nodes'],'edges',r['edges'],'limits',r['truncated'])
 print('nodes',[(n['id'],n['package'],n['function']) for n in d['nodes']]);print('edges',d['edges'])
PY
rg -n 'AtomicReplace|overwrite|replace|Overwrite|Replace' internal/backend/test internal/backend/local internal/backend/retry/backend_retry_test.go; rg -n '\.Save\(' internal/repository/raw.go internal/repository/repository.go internal/repository/packer_uploader.go; date -u +%Y-%m-%dT%H:%M:%SZ
```

### 26. read; error=False; duration_ms=1
```json
{"path": "internal/repository/repository.go", "offset": 470, "limit": 80}
```

### 27. read; error=False; duration_ms=1
```json
{"path": "internal/backend/test/suite.go", "offset": 1, "limit": 205}
```

### 28. bash; error=False; duration_ms=23
```sh
rg -n 'New|Save|Permanent|Remove' internal/backend/retry/backend_retry.go | head -25; rg -n 'func .*Test|SaveError|Save' internal/backend/local/*test.go; date -u +%Y-%m-%dT%H:%M:%SZ
```

### 29. bash; error=False; duration_ms=610
```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -reason interface_dispatch -operation '.Save' -package /internal/repository -max-depth 2 -max-sites 8 -max-edges 30 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/backend-save-boundary.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/backend-save-boundary.stderr; printf 'boundary_exit=%s\n' "$?"; python3 - <<'PY'
import json
b='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/'
d=json.load(open(b+'backend-save-boundary.json'));print('coverage',d['coverage'],'limits',d['truncated']);print('unresolved',json.dumps(d['unresolved_sites'],indent=2));print('nodes',[(n['id'],n['function']) for n in d['nodes']]);print('edges',json.dumps(d['edges'],indent=2))
PY
date -u +%Y-%m-%dT%H:%M:%SZ
```

### 30. bash; error=False; duration_ms=69
```sh
python3 - <<'PY'
import json
b='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/'
d=json.load(open(b+'backend-save-boundary.json'))
for s in d['sites']:print(s['id'],s['function'],s['api'],s['location'],s['uncertainty_reasons'])
PY
rg -n 'Atomic|Replace|SaveError|Save' internal/backend/test/tests.go | tail -35; rg -n 'Preallocate|func' internal/fileio/preallocate*; date -u +%Y-%m-%dT%H:%M:%SZ
```

### 31. write; error=False; duration_ms=0
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/candidates.md`

### 32. bash; error=False; duration_ms=116
```sh
{ printf 'cwd='; pwd; printf 'HEAD='; git rev-parse HEAD; printf 'status (empty means clean):\n'; git status --porcelain; printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"; shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; go version; go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help; date -u +%Y-%m-%dT%H:%M:%SZ; } > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/provenance.txt 2>&1; date -u +%Y-%m-%dT%H:%M:%SZ
```

### 33. write; error=False; duration_ms=2
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/report.md`

### 34. bash; error=False; duration_ms=43
```sh
wc -w /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D/report.md; date -u +%Y-%m-%dT%H:%M:%SZ; git status --porcelain
```

### 35. bash; error=False; duration_ms=41
```sh
python3 - <<'PY'
import json,time,pathlib
base=pathlib.Path('/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/D')
timing=json.loads((base/'timing.json').read_text());now=time.time();elapsed=now-timing['start_epoch']
p=base/'report.md';s=p.read_text();s=s.replace('Investigation completed and report written about 200 seconds after the recorded launch, before both deadlines; timing is approximate pending final clock check.',f'Investigation and report complete at {elapsed:.1f} seconds after the recorded launch, before the 240-second investigation and 300-second report deadlines. Final repository status remained clean.')
p.write_text(s)
check={'checked_epoch':now,'elapsed_seconds':elapsed,'investigation_deadline_epoch':timing['investigation_deadline_epoch'],'report_deadline_epoch':timing['report_deadline_epoch'],'analyzer_exit_statuses':{'survey':0,'rename':0,'save-effects':0,'save-unresolved':0,'backend-save-boundary':0},'tests_executed':False,'source_edits':False,'note':'One display-only Python summary failed by treating unresolved site IDs as objects; corrected summary read the sites table. Analyzer loads all succeeded.'}
(base/'completion.json').write_text(json.dumps(check,indent=2)+'\n')
print('elapsed_seconds',round(elapsed,1),'report_words',len(s.split()))
PY
```

## Analyzer artifact coverage

### `backend-save-boundary.json`
```json
{
  "coverage": {
    "packages_selected": 6,
    "source_functions": 446,
    "operation_sites_identified": 51,
    "selected_operation_sites": 38,
    "dependency_operation_sites": 13,
    "unresolved_sites_identified": 1021,
    "sites_matching_filters": 8,
    "sites_emitted": 8
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": true
  },
  "emitted_sites": 8,
  "emitted_unresolved": 8
}
```

### `rename.json`
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

### `save-effects.json`
```json
{
  "coverage": {
    "packages_selected": 6,
    "source_functions": 446,
    "operation_sites_identified": 51,
    "selected_operation_sites": 38,
    "dependency_operation_sites": 13,
    "unresolved_sites_identified": 1021,
    "sites_matching_filters": 7,
    "sites_emitted": 7
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": false
  },
  "emitted_sites": 7,
  "emitted_unresolved": 0
}
```

### `save-unresolved.json`
```json
{
  "coverage": {
    "packages_selected": 6,
    "source_functions": 446,
    "operation_sites_identified": 51,
    "selected_operation_sites": 38,
    "dependency_operation_sites": 13,
    "unresolved_sites_identified": 1021,
    "sites_matching_filters": 50,
    "sites_emitted": 10
  },
  "truncated": {
    "sites": true,
    "edges": false,
    "depth": true
  },
  "emitted_sites": 10,
  "emitted_unresolved": 10
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
    "sites_emitted": 24
  },
  "truncated": {
    "sites": true,
    "edges": false,
    "depth": true
  },
  "emitted_sites": 24,
  "emitted_unresolved": 0
}
```

