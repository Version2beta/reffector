# Supervisor evidence index — B

Exact tool calls and results remain in `events.jsonl`; this is a derived index.

## Tool calls

### 1. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/.agents/skills/reffector/SKILL.md", "offset": null, "limit": null}
```

### 2. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/timing.json", "offset": null, "limit": null}
```

### 3. bash; error=False; duration_ms=110
```sh
pwd; git rev-parse HEAD; git status --porcelain; printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"; date +%s; shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version; GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help
```

### 4. bash; error=False; duration_ms=55
```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect fs. -max-depth 0 -max-sites 45 -max-edges 0 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/fs-survey.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/fs-survey.stderr; printf 'analyzer_exit=%s\n' "$?"; date +%s; rg --files internal/repository internal/backend/local | head -70
```

### 5. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/fs-survey.stderr", "offset": null, "limit": null}
```

### 6. bash; error=False; duration_ms=643
```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect fs. -max-depth 0 -max-sites 45 -max-edges 50 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/fs-survey-valid.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/fs-survey-valid.stderr; printf 'analyzer_exit=%s\n' "$?"; date +%s
```

### 7. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/fs-survey-valid.stderr", "offset": null, "limit": null}
```

### 8. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/fs-survey-valid.json", "offset": 1, "limit": 250}
```

### 9. read; error=False; duration_ms=1
```json
{"path": "internal/backend/local/local.go", "offset": 1, "limit": 380}
```

### 10. bash; error=False; duration_ms=31
```sh
python3 - <<'PY'
import json
p='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/fs-survey-valid.json'
d=json.load(open(p)); print('keys', list(d));
for s in d['sites']:
 print(s['id'],s['function'],s['api'],s['location']['file'].split('/restic/')[-1],s['location']['line'])
print('metadata', {k:v for k,v in d.items() if k not in ['sites','groups','graph','nodes','edges']})
PY
```

### 11. read; error=False; duration_ms=2
```json
{"path": "internal/repository/upgrade_repo.go", "offset": null, "limit": null}
```

### 12. read; error=False; duration_ms=1
```json
{"path": "internal/backend/local/local_internal_test.go", "offset": null, "limit": null}
```

### 13. read; error=False; duration_ms=2
```json
{"path": "internal/backend/local/local_unix.go", "offset": null, "limit": null}
```

### 14. read; error=False; duration_ms=1
```json
{"path": "internal/repository/prune.go", "offset": 1, "limit": 250}
```

### 15. write; error=False; duration_ms=1
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/candidates.md`

### 16. bash; error=False; duration_ms=588
```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -operation Remove -reason interface_dispatch -max-depth 2 -max-sites 15 -max-edges 50 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/remove-boundaries.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/remove-boundaries.stderr; printf 'analyzer_exit=%s\n' "$?"; rg -n 'func decidePackAction|func .*Execute|repo.List|func Test|Save\(|Sync|Readonly|UpgradeRepo' internal/repository/prune.go internal/repository/upgrade_repo_test.go internal/backend/local/local_test.go; date +%s
```

### 17. read; error=False; duration_ms=2
```json
{"path": "internal/repository/prune.go", "offset": 370, "limit": 350}
```

### 18. read; error=False; duration_ms=1
```json
{"path": "internal/repository/upgrade_repo_test.go", "offset": null, "limit": null}
```

### 19. read; error=False; duration_ms=2
```json
{"path": "internal/backend/local/local_test.go", "offset": null, "limit": null}
```

### 20. read; error=False; duration_ms=2
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/remove-boundaries.stderr", "offset": null, "limit": null}
```

### 21. bash; error=False; duration_ms=73
```sh
python3 - <<'PY'
import json
p='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/remove-boundaries.json'; d=json.load(open(p));print('truncated',d['truncated']);
for s in d['unresolved_sites']:print(s['id'],s.get('function'),s.get('api'),s.get('location'),s.get('uncertainty_reasons'))
for r in d['reverse']:
 print('reverse',r['site'],r['truncated']);
for n in d['nodes']:print('node',n)
for e in d['edges']:print('edge',e)
PY
rg -n 'decidePackAction|calculateTargetPacksize|func Test|PlanPrune|Lock' internal/repository/prune_internal_test.go internal/repository/prune_test.go cmd/restic/cmd_prune.go internal/repository/repository.go internal/backend/backend.go; date +%s
```

### 22. read; error=False; duration_ms=0
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/remove-boundaries.json", "offset": 1, "limit": 240}
```

### 23. bash; error=False; duration_ms=1095
```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -site site:9a8cbcbfba99c06fc0ee41fa4c19d4b53c29a2e634c66fca321f022a51a01a3c -max-depth 2 -max-sites 5 -max-edges 30 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/save-rename.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/save-rename.stderr; printf 'rename_exit=%s\n' "$?"; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -operation List -function decidePackAction -max-depth 2 -max-sites 8 -max-edges 30 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/prune-list.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/prune-list.stderr; printf 'prune_exit=%s\n' "$?"; date +%s
```

### 24. read; error=False; duration_ms=1
```json
{"path": "internal/repository/prune_internal_test.go", "offset": null, "limit": null}
```

### 25. read; error=False; duration_ms=1
```json
{"path": "internal/repository/prune_test.go", "offset": 1, "limit": 200}
```

### 26. read; error=False; duration_ms=1
```json
{"path": "internal/repository/lock_file.go", "offset": 230, "limit": 160}
```

### 27. read; error=False; duration_ms=2
```json
{"path": "cmd/restic/cmd_prune.go", "offset": 160, "limit": 100}
```

### 28. bash; error=False; duration_ms=43
```sh
python3 - <<'PY'
import json
base='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/'
for fn in ['save-rename','prune-list','remove-boundaries']:
 d=json.load(open(base+fn+'.json'));print('\nREPORT',fn,'coverage',d['coverage'],'truncated',d['truncated'])
 for s in d['sites']:
  print('SITE',s['id'],s['function'],s['api'],s['location']['line'])
 print('NODES')
 for n in d['nodes']:print(n)
 print('EDGES')
 for e in d['edges']:print(e)
PY
```

### 29. bash; error=False; duration_ms=61
```sh
python3 - <<'PY'
p='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/candidates.md'
with open(p,'a') as f:f.write('''
## 3 update — strongest conditional recommendation
[R] prune-list.json reports shared Repository.List -> backend.Backend.List boundary repository.go:963, site:cb4d488e0c3add7444053d993ca299849465a2f6afb55666531c107fa0850912, with decidePackAction caller prune.go:399. Interface dispatch unresolved; do not invent a concrete Local.List edge.
[F/I] prune.go:409-470 embeds indexed-pack safety check, compression rules and remove/keep/candidate/small-candidate classification in a backend listing callback alongside statistics, progress and index-map mutation. Small candidate helper could classify one indexed pack from value inputs synchronously, leaving listing, stats, diagnostics and mutation in the existing callback. No new inventory snapshot, no delayed executable command plan. Existing PlanPrune/Execute split already owns consequential deletion order. Proposed benefit: small table tests for precedence instead of creating repository data. Not implemented or verified.
[F] cmd/restic/cmd_prune.go:174-183 rejects no-lock except dry run and holds exclusive lock across planning/execution; this does not imply external library callers hold it.
[I] prune tests inspected: integration-style TestPrune, TestPruneSmall and TestPruneMaxUnusedDuplicate exist. No direct classifier tests identified in those files; test source inspection is not test execution.

## 4. Lock refresh protocol — rejected
[R] remove-boundaries.json reports separate RemoveUnpacked sites lock_file.go:321, :351, :357; refresh and refreshStaleLock share adoptReplacementLock, while cleanup branches are independent sites.
[F] lock_file.go:325-361 deliberately checks old lock existence both before and after replacement creation and a wait. Moving observations into a precomputed plan would obscure their temporal role. Keep protocol ownership/order together; no independent policy extraction demonstrated.

## 1 update
[R] save-rename.json depth 2 has one origin, no edges, no truncation; static absence of concrete callers is not evidence Save is unused (interface dispatch).
[F] local_internal_test.go TestNoSpacePermanent injects temp creation ENOSPC and checks permanent-error identity. local_test.go invokes generic backend suite, not inspected in depth. Keep Save lifecycle intact.

## 2 update
[F] upgrade_repo_test.go has success and induced new-upload plus restore-upload failure test; preserved backup path checked nonempty then removed. No assertion read of backup contents in inspected test. Extraction of version assignment remains rejected as trivial and obscuring recovery ownership.
''')
PY
rg -n 'func \(r \*Repository\) List|func \(.*\) Config|List\(ctx|not called concurrently|serial|concurrent' internal/repository/repository.go internal/backend/backend.go; date +%s
```

### 30. read; error=False; duration_ms=3
```json
{"path": "internal/repository/repository.go", "offset": 950, "limit": 42}
```

### 31. read; error=False; duration_ms=2
```json
{"path": "internal/backend/backend.go", "offset": 15, "limit": 95}
```

### 32. read; error=False; duration_ms=3
```json
{"path": "internal/backend/local/local_windows.go", "offset": 1, "limit": 140}
```

### 33. bash; error=False; duration_ms=54
```sh
rg -n 'ErrSizeNotMatching|ErrPacksMissing|decidePackAction|RepackUncompressed|missing.*pack|pack.*missing' internal/repository/*test.go cmd/restic/*prune*test.go; git status --porcelain; date +%s
```

### 34. bash; error=False; duration_ms=629
```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -site site:cb4d488e0c3add7444053d993ca299849465a2f6afb55666531c107fa0850912 -max-depth 2 -max-sites 3 -max-edges 45 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/list-callers.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/list-callers.stderr; printf 'list_callers_exit=%s\n' "$?"; date +%s
```

### 35. read; error=False; duration_ms=2
```json
{"path": "cmd/restic/cmd_prune_integration_test.go", "offset": 80, "limit": 160}
```

### 36. bash; error=False; duration_ms=118
```sh
python3 - <<'PY'
import json
p='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/list-callers.json'; d=json.load(open(p)); print('coverage',d['coverage']);print('truncated',d['truncated']);print('counts',len(d['sites']),len(d['nodes']),len(d['edges']))
n={x['id']:x['function'] for x in d['nodes']}
for e in d['edges']:
 if n[e['caller']] in ['PlanPrune','decidePackAction'] or n[e['callee']] == '(*Repository).List': print(n[e['caller']], '->', n[e['callee']], e.get('source',{}).get('location'))
PY
{ printf 'cwd='; pwd; printf 'HEAD='; git rev-parse HEAD; printf 'status follows (empty is clean):\n'; git status --porcelain; printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"; shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version; GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; } > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/provenance.txt
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/help.txt 2>&1; date +%s
```

### 37. bash; error=False; duration_ms=49
```sh
python3 - <<'PY'
p='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/candidates.md'
with open(p,'a') as f:f.write('''
## Final refinement and counterevidence
[F] backend/backend.go:50-58 guarantees List callbacks run in the invoking goroutine, at most once per file on failure; repository.go:963 parses IDs then forwards size. A synchronous value-only classifier needs no new concurrency or resource ownership.
[R] list-callers.json removes function-root restriction: one shared List site, 12 nodes/13 edges, depth truncated but no site/edge truncation. PlanPrune :136 -> decidePackAction :399 -> Repository.List :963 is explicit. RepairIndex, Checker.Packs, Init, resolveBlobsForPacks and searchKey also share this site; this is not a reason to move prune policy into Repository.List.
[I] cmd/restic/cmd_prune_integration_test.go TestPruneWithDamagedRepository checks ErrPacksMissing; TestEdgeCaseRepos includes missing-unused-data, mixed-packs and duplicates. Counterevidence reduces claims of missing coverage: recommendation is cheaper direct rule tests, not absent existing safety tests.
Selected smallest scope: extract only indexed-pack compression/action choice at prune.go:442-469; keep size-mismatch guard :409-415, unindexed handling :400-407, statistics, progress and map deletion in callback. Helper returns keep/remove/repack-candidate/small-candidate plus mustCompress, using packInfo, observed packSize, captured version, targetPackSize and relevant option fields; no repository/printer/callback invocation. Missing-pack handling and global repack limits stay in decidePackAction. Benefit remains a proposal, not verified. Reject if action-result plumbing duplicates policy or makes guard precedence harder to see.
No other lead examined beyond listed areas. Repository remains clean at final provenance capture; no tests executed, no app run, no edits.
''')
PY
```

### 38. write; error=False; duration_ms=1
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/commands.md`

### 39. write; error=False; duration_ms=1
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/report.md`

### 40. bash; error=False; duration_ms=66
```sh
date +%s; wc -w /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/report.md; git status --porcelain; python3 - <<'PY'
import json,time
b='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B/'
t=json.load(open(b+'timing.json')); now=time.time(); print('elapsed',now-t['start_epoch'],'investigation_remaining',t['investigation_deadline_epoch']-now,'report_remaining',t['report_deadline_epoch']-now)
PY
```

### 41. bash; error=False; duration_ms=76
```sh
python3 - <<'PY'
import json,time,pathlib
b=pathlib.Path('/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/B')
t=json.loads((b/'timing.json').read_text()); now=time.time()
r={'case':'B','start_epoch':t['start_epoch'],'completed_epoch':now,'elapsed_seconds':round(now-t['start_epoch'],3),'report_before_deadline':now<t['report_deadline_epoch'],'status':'complete_read_only_proposal','source_changed':False,'tests_executed':False,'analyzer_loads_succeeded':5,'cli_rejections_before_load':1,'report_words':len((b/'report.md').read_text().split()),'last_repository_status':'clean at final check'}
(b/'completion.json').write_text(json.dumps(r,indent=2)+'\n')
print(json.dumps(r,indent=2))
print('Artifacts:', ', '.join(sorted(p.name for p in b.iterdir() if p.is_file())))
PY
```

## Analyzer artifact coverage

### `fs-survey-valid.json`
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
    "sites_emitted": 33
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": true
  },
  "emitted_sites": 33,
  "emitted_unresolved": 0
}
```

### `list-callers.json`
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
  "emitted_unresolved": 1
}
```

### `prune-list.json`
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

### `remove-boundaries.json`
```json
{
  "coverage": {
    "packages_selected": 6,
    "source_functions": 446,
    "operation_sites_identified": 51,
    "selected_operation_sites": 38,
    "dependency_operation_sites": 13,
    "unresolved_sites_identified": 1021,
    "sites_matching_filters": 9,
    "sites_emitted": 9
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": true
  },
  "emitted_sites": 9,
  "emitted_unresolved": 9
}
```

### `save-rename.json`
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

