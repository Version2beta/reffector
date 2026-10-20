# Supervisor evidence index — A

Exact tool calls and results remain in `events.jsonl`; this is a derived index.

## Tool calls

### 1. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/.agents/skills/reffector/SKILL.md", "offset": null, "limit": null}
```

### 2. bash; error=False; duration_ms=96
```sh
pwd; git rev-parse HEAD; git status --porcelain; printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"; shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version; env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help
```

### 3. read; error=False; duration_ms=2
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/timing.json", "offset": null, "limit": null}
```

### 4. bash; error=False; duration_ms=29
```sh
date -u '+%Y-%m-%dT%H:%M:%SZ'; env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -max-depth 0 -max-sites 65 -max-edges 0 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/survey.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/survey.stderr; printf 'analyzer_exit=%s\n' "$?"; rg --files -g '*AGENTS.md' -g '*CONTRIBUTING.md' -g '*_test.go' modules/caddyhttp .github | head -65
```

### 5. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/survey.stderr", "offset": null, "limit": null}
```

### 6. bash; error=False; duration_ms=1743
```sh
env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -max-depth 0 -max-sites 65 -max-edges 1 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/survey-valid.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/survey-valid.stderr; printf 'analyzer_exit=%s\n' "$?"; date -u '+%Y-%m-%dT%H:%M:%SZ'; rg --files -g 'AGENTS.md' -g 'go.mod' -g 'go.work' .
```

### 7. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/survey-valid.stderr", "offset": null, "limit": null}
```

### 8. read; error=False; duration_ms=2
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/survey-valid.json", "offset": 1, "limit": 160}
```

### 9. read; error=False; duration_ms=1
```json
{"path": "AGENTS.md", "offset": null, "limit": null}
```

### 10. read; error=False; duration_ms=2
```json
{"path": ".github/CONTRIBUTING.md", "offset": null, "limit": null}
```

### 11. bash; error=False; duration_ms=33
```sh
python3 - <<'PY'
import json
p='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/survey-valid.json'
d=json.load(open(p)); print('keys',list(d));
for s in d['sites']:
 print(s['id'],s['function'],s['api'],s.get('location'))
print('coverage',d.get('coverage')); print('truncated',d.get('truncated'))
PY
```

### 12. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/intercept/intercept.go", "offset": 80, "limit": 160}
```

### 13. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/responsewriter.go", "offset": 1, "limit": 350}
```

### 14. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/headers/headers.go", "offset": 350, "limit": 100}
```

### 15. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/reverseproxy/reverseproxy.go", "offset": 1250, "limit": 250}
```

### 16. write; error=False; duration_ms=2
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/candidates.md`

### 17. read; error=False; duration_ms=3
```json
{"path": "modules/caddyhttp/encode/encode.go", "offset": 240, "limit": 340}
```

### 18. bash; error=False; duration_ms=1881
```sh
env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -api 'net/http.(ResponseWriter).WriteHeader' -package /encode -max-depth 2 -max-sites 20 -max-edges 50 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/encode-headers.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/encode-headers.stderr; printf 'analyzer_exit=%s\n' "$?"; date -u '+%Y-%m-%dT%H:%M:%SZ'; rg -n 'func Test|wroteHeader|Close\(|Flush|ReadFrom|Content-Length|ContentLength|NoContent|NotModified|Switching' modules/caddyhttp/encode/encode_test.go modules/caddyhttp/encode/encode_conformance_test.go modules/caddyhttp/encode/encode_harness_test.go | head -95
```

### 19. bash; error=False; duration_ms=1664
```sh
env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect net.connect -max-depth 2 -max-sites 20 -max-edges 60 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/connect.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/connect.stderr; printf 'analyzer_exit=%s\n' "$?"; python3 - <<'PY'
import json
base='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/'
for f in ['encode-headers.json','connect.json']:
 d=json.load(open(base+f)); print(f, d['coverage'], 'truncation',d['truncated'])
 for s in d['sites']:print(s['id'],s['api'],s['function'],s['location'],s.get('uncertainty_reasons'))
 print('reverse',str(d['reverse'])[:6500])
PY
```

### 20. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/httptransport.go", "offset": 230, "limit": 110}
```

### 21. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/upstreams.go", "offset": 55, "limit": 245}
```

### 22. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/encode/encode_conformance_test.go", "offset": 130, "limit": 200}
```

### 23. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/encode/encode.go", "offset": 135, "limit": 110}
```

### 24. bash; error=False; duration_ms=12
```sh
rg -n 'type UpstreamResolver|func .*ParseAddresses|Resolver|DialTimeout|FallbackDelay|resolver' modules/caddyhttp/reverseproxy/upstreams_test.go modules/caddyhttp/reverseproxy/httptransport_test.go modules/caddyhttp/reverseproxy/dynamic_upstreams_test.go modules/caddyhttp/reverseproxy/httptransport.go modules/caddyhttp/reverseproxy/upstreams.go | head -115; date -u '+%Y-%m-%dT%H:%M:%SZ'
```

### 25. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/reverseproxy/upstreams.go", "offset": 370, "limit": 55}
```

### 26. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/reverseproxy/httptransport_test.go", "offset": 1, "limit": 180}
```

### 27. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/reverseproxy/dynamic_upstreams_test.go", "offset": 1, "limit": 170}
```

### 28. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/reverseproxy/upstreams_test.go", "offset": 1, "limit": 120}
```

### 29. write; error=False; duration_ms=1
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/candidates.md`

### 30. bash; error=False; duration_ms=129
```sh
rg -n 'ParseAddresses\(|UpstreamResolver|resolver.*addresses|dial_fallback_delay' modules/caddyhttp/reverseproxy --glob '*_test.go'; rg -n 'weakrand|NewTransport\(|func .*Provision|DialContext:|DialContext =' modules/caddyhttp/reverseproxy/httptransport.go; git status --porcelain; date -u '+%Y-%m-%dT%H:%M:%SZ'; python3 - <<'PY'
import json
p='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/connect.json'; d=json.load(open(p))
print('node sample', d['nodes'][:2]); print('edge sample',d['edges'][:2])
PY
```

### 31. read; error=False; duration_ms=3
```json
{"path": "modules/caddyhttp/reverseproxy/httptransport.go", "offset": 178, "limit": 57}
```

### 32. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/httptransport.go", "offset": 385, "limit": 44}
```

### 33. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/upstreams_bench_test.go", "offset": 1, "limit": 65}
```

### 34. bash; error=False; duration_ms=1700
```sh
rg -n 'ParseAddresses\(|\.netAddrs|\.resolver\s*=' modules/caddyhttp/reverseproxy; env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -api 'net.(*Dialer).DialContext' -max-depth 3 -max-sites 10 -max-edges 40 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/dial-sites.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/dial-sites.stderr; printf 'analyzer_exit=%s\n' "$?"; date -u '+%Y-%m-%dT%H:%M:%SZ'
```

### 35. bash; error=False; duration_ms=91
```sh
{ printf 'cwd='; pwd; printf 'HEAD='; git rev-parse HEAD; printf 'status porcelain follows (empty means clean):\n'; git status --porcelain; printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"; shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version; env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; } > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/provenance.txt; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/help.txt 2>&1; python3 - <<'PY'
import json,time
base='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/'
d=json.load(open(base+'dial-sites.json')); print(d['coverage']); print(d['truncated']); print([(s['function'],s['location']['line']) for s in d['sites']]);
t=json.load(open(base+'timing.json')); print('elapsed',time.time()-t['start_epoch'],'investigation_remaining',t['investigation_deadline_epoch']-time.time())
PY
```

### 36. write; error=False; duration_ms=1
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/report.md`

### 37. bash; error=False; duration_ms=82
```sh
wc -w /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/report.md; git status --porcelain; git diff --stat; python3 - <<'PY'
import time,json
p='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A/timing.json'
d=json.load(open(p)); print('elapsed_seconds',time.time()-d['start_epoch']); print('investigation_remaining',d['investigation_deadline_epoch']-time.time())
PY
```

### 38. bash; error=False; duration_ms=55
```sh
python3 - <<'PY'
from pathlib import Path
import json,time,datetime
base=Path('/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/A')
p=base/'candidates.md'
s=p.read_text().replace('Candidate: unexported UpstreamResolver resolver-construction method taking explicit durations, used only inside existing nonnil branches.', 'Final proposal: unexported newUpstreamResolver factory taking an explicit current-resolver accessor and durations, used only inside existing nonnil branches. The accessor retains existing late reads of each module Resolver field; a receiver-only factory would need an established immutability contract.')
s += '\nFinal gate: proposal only; no implementation or test success claimed. Full report saved. Final git status and diff were empty. No independent [I] finding promoted.\n'
p.write_text(s)
t=json.loads((base/'timing.json').read_text()); now=time.time()
(base/'completion.json').write_text(json.dumps({'case':'A','completed_at_utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'elapsed_seconds':round(now-t['start_epoch'],3),'before_investigation_deadline':now<t['investigation_deadline_epoch'],'before_report_deadline':now<t['report_deadline_epoch'],'status':'read-only evaluation complete; proposal unimplemented and untested','successful_analyzer_loads':4,'rejected_analyzer_invocations':1,'final_repository_status':'clean; git status --porcelain and git diff --stat empty','report':'report.md'},indent=2)+'\n')
print('report exists:',(base/'report.md').is_file(),'elapsed:',round(now-t['start_epoch'],3))
PY
```

## Analyzer artifact coverage

### `connect.json`
```json
{
  "coverage": {
    "packages_selected": 22,
    "source_functions": 1213,
    "operation_sites_identified": 136,
    "selected_operation_sites": 84,
    "dependency_operation_sites": 52,
    "unresolved_sites_identified": 7166,
    "sites_matching_filters": 10,
    "sites_emitted": 10
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": true
  },
  "emitted_sites": 10,
  "emitted_unresolved": 0
}
```

### `dial-sites.json`
```json
{
  "coverage": {
    "packages_selected": 22,
    "source_functions": 1213,
    "operation_sites_identified": 136,
    "selected_operation_sites": 84,
    "dependency_operation_sites": 52,
    "unresolved_sites_identified": 7166,
    "sites_matching_filters": 5,
    "sites_emitted": 5
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": false
  },
  "emitted_sites": 5,
  "emitted_unresolved": 0
}
```

### `encode-headers.json`
```json
{
  "coverage": {
    "packages_selected": 22,
    "source_functions": 1213,
    "operation_sites_identified": 136,
    "selected_operation_sites": 84,
    "dependency_operation_sites": 52,
    "unresolved_sites_identified": 7166,
    "sites_matching_filters": 5,
    "sites_emitted": 5
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": false
  },
  "emitted_sites": 5,
  "emitted_unresolved": 0
}
```

### `survey-valid.json`
```json
{
  "coverage": {
    "packages_selected": 22,
    "source_functions": 1213,
    "operation_sites_identified": 136,
    "selected_operation_sites": 84,
    "dependency_operation_sites": 52,
    "unresolved_sites_identified": 7166,
    "sites_matching_filters": 136,
    "sites_emitted": 65
  },
  "truncated": {
    "sites": true,
    "edges": false,
    "depth": true
  },
  "emitted_sites": 65,
  "emitted_unresolved": 0
}
```

