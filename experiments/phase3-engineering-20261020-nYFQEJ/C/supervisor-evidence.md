# Supervisor evidence index — C

Exact tool calls and results remain in `events.jsonl`; this is a derived index.

## Tool calls

### 1. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/.agents/skills/reffector/SKILL.md", "offset": null, "limit": null}
```

### 2. bash; error=False; duration_ms=65
```sh
pwd; git rev-parse HEAD; git status --porcelain; printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"; shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; go version; go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help
```

### 3. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/timing.json", "offset": null, "limit": null}
```

### 4. bash; error=False; duration_ms=1767
```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect http -max-depth 0 -max-sites 45 -max-edges 80 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/http-sites.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/http-sites.stderr; printf 'analyzer_exit=%s\n' "$?"; date +%s; rg --files -g AGENTS.md -g '*proxy*test*' -g '*stream*test*' modules/caddyhttp .github; rg -n 'WriteHeader|Write\(|Hijack|handleResponse|finalizeResponse|roundTrip|ServeHTTP|handleUpgrade' modules/caddyhttp/reverseproxy/reverseproxy.go modules/caddyhttp/reverseproxy/streaming.go
```

### 5. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/http-sites.json", "offset": 1, "limit": 180}
```

### 6. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/http-sites.stderr", "offset": null, "limit": null}
```

### 7. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/reverseproxy.go", "offset": 1180, "limit": 315}
```

### 8. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/streaming.go", "offset": 62, "limit": 135}
```

### 9. bash; error=False; duration_ms=55
```sh
date +%s; rg --files -g AGENTS.md; rg -n 'func .*|WriteHeader|http.Error|Accept|Header|refuseIncremental' modules/caddyhttp/reverseproxy/webtransport.go modules/caddyhttp/reverseproxy/response* modules/caddyhttp/reverseproxy/*incremental*; rg -n 'handleResponse|CopyResponse|WriteHeader|error|HandleError' modules/caddyhttp/server.go modules/caddyhttp/reverseproxy/copyresponse.go; python3 - <<'PY'
import json
p='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/http-sites.json'
d=json.load(open(p));print('TOP',d.keys());print('COVERAGE',d.get('coverage'));print('TRUNCATION',d.get('truncated'))
for s in d['sites']:
 if 'reverseproxy' in s['package']: print(s['id'],s['function'],s['api'],s['location'], 'uncertain',s.get('uncertain'))
PY
```

### 10. read; error=False; duration_ms=1
```json
{"path": "AGENTS.md", "offset": null, "limit": null}
```

### 11. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/reverseproxy/reverseproxy.go", "offset": 618, "limit": 250}
```

### 12. read; error=False; duration_ms=0
```json
{"path": "modules/caddyhttp/reverseproxy/copyresponse.go", "offset": 52, "limit": 130}
```

### 13. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/server.go", "offset": 760, "limit": 85}
```

### 14. write; error=False; duration_ms=1
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/candidates.md`

### 15. bash; error=False; duration_ms=1667
```sh
date +%s; rg -n 'func .*webTransportHijack|func .*refuseIncremental|func copyHeader|func statusError|WriteHeader|\.Upgrade\(' modules/caddyhttp/reverseproxy/*webtransport* modules/caddyhttp/reverseproxy/reverseproxy.go; rg -n '^func Test' modules/caddyhttp/reverseproxy/streaming_test.go; rg --files modules/caddyhttp/reverseproxy | head -55; export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -api 'net/http.(ResponseWriter).WriteHeader' -max-depth 4 -max-sites 40 -max-edges 150 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/writeheader-callers.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/writeheader-callers.stderr; printf 'analyzer_exit=%s\n' "$?"
```

### 16. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/reverseproxy/webtransport_transport.go", "offset": 1, "limit": 270}
```

### 17. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/reverseproxy.go", "offset": 1760, "limit": 25}
```

### 18. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/reverseproxy.go", "offset": 1850, "limit": 65}
```

### 19. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/reverseproxy.go", "offset": 2020, "limit": 100}
```

### 20. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/reverseproxy/forwardauth/caddyfile.go", "offset": 1, "limit": 230}
```

### 21. bash; error=False; duration_ms=2020
```sh
python3 - <<'PY'
import json
p='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/writeheader-callers.json';d=json.load(open(p));print('coverage',d['coverage']);print('truncation',d['truncated']);print('reverse sample',str(d['reverse'])[:1400]);print('nodes sample',str(d['nodes'])[:1800]);print('edges sample',str(d['edges'])[:2200]);
for s in d['sites']:
 if s['location']['file'].endswith('server.go'): print(s['id'],s['location']['line'])
PY
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -operation Upgrade -max-depth 3 -max-sites 12 -max-edges 60 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/upgrade-sites.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/upgrade-sites.stderr; printf 'analyzer_exit=%s\n' "$?"; rg -n 'webtransport-go|^go |^toolchain' go.mod; rg -n '^func Test|Header\(' modules/caddyhttp/reverseproxy/webtransport_transport_test.go | head -70; rg -n '^func Test' modules/caddyhttp/reverseproxy/extended_connect_test.go modules/caddyhttp/reverseproxy/hopheaders_test.go modules/caddyhttp/reverseproxy/client_disconnect_test.go modules/caddyhttp/reverseproxy/headers_test.go
```

### 22. bash; error=False; duration_ms=1760
```sh
date +%s; python3 - <<'PY'
import json
base='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/'
d=json.load(open(base+'upgrade-sites.json'));print('upgrade',d['coverage'],d['truncated']);
d=json.load(open(base+'writeheader-callers.json'))
for e in d['edges']:
 s=e.get('source',{})
 if 'reverseproxy' in s.get('caller',{}).get('package',''): print(s)
PY
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; go env GOMODCACHE; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -operation Upgrade -max-depth 3 -max-sites 12 -max-edges 60 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/upgrade-unresolved.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/upgrade-unresolved.stderr; printf 'analyzer_exit=%s\n' "$?"; rg -n 'WriteHeader|http.Error|func .*Upgrade' /Users/rob/go/pkg/mod/github.com/quic-go/webtransport-go@v0.13.0/server.go; rg -n 'func .*|WriteHeader|Write\(|Unwrap|Hijack|ReadFrom' modules/caddyhttp/headers/headers.go modules/caddyhttp/responsewriter.go | tail -65
```

### 23. read; error=False; duration_ms=2
```json
{"path": "/Users/rob/go/pkg/mod/github.com/quic-go/webtransport-go@v0.13.0/server.go", "offset": 383, "limit": 110}
```

### 24. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/headers/headers.go", "offset": 85, "limit": 50}
```

### 25. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/headers/headers.go", "offset": 398, "limit": 38}
```

### 26. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/reverseproxy/headers_test.go", "offset": 134, "limit": 120}
```

### 27. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/hopheaders_test.go", "offset": 1, "limit": 175}
```

### 28. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/webtransport_transport.go", "offset": 250, "limit": 42}
```

### 29. bash; error=False; duration_ms=75
```sh
date +%s; python3 - <<'PY'
import json
b='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/'
d=json.load(open(b+'writeheader-callers.json'))
for s in d['sites']:
 if 'webtransport' in s['package']: print(s)
u=json.load(open(b+'upgrade-unresolved.json'));print('unresolved upgrade',u['coverage']['sites_matching_filters'],u['truncated']);print(u['unresolved_sites'])
PY
rg -n 'WriteHeader|WriteResponse|ServeHTTP|ResponseRecorder|return next|return nil' modules/caddyhttp/intercept/intercept.go modules/caddyhttp/staticresp.go; rg -n 'copy_response|handle_response|header_down|early.hints|StatusEarlyHints' caddytest/integration --glob '*.go' | head -65; rg -n '^func Test|HandleResponse|StatusCode|Header|WriteHeader' modules/caddyhttp/reverseproxy/retries_test.go | head -65
```

### 30. read; error=False; duration_ms=1
```json
{"path": "caddytest/integration/reverseproxy_upgrade_handlers_test.go", "offset": 1, "limit": 200}
```

### 31. read; error=False; duration_ms=2
```json
{"path": "caddytest/integration/webtransport_test.go", "offset": 216, "limit": 155}
```

### 32. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/intercept/intercept.go", "offset": 115, "limit": 95}
```

### 33. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/staticresp.go", "offset": 181, "limit": 80}
```

### 34. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/streaming_test.go", "offset": 360, "limit": 165}
```

### 35. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/extended_connect_test.go", "offset": 67, "limit": 125}
```

### 36. bash; error=False; duration_ms=59
```sh
python3 - <<'PY'
import json
b='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/';d=json.load(open(b+'writeheader-callers.json'));n={x['id']:x for x in d['nodes']}
for g in d['reverse']:
 if g['site'] in ['site:63070b79b14c01fb5390068602aec185d5262b0e8155a598eca63b237e972aae','site:afda971af6b315c6470be2e69021b7d506d9cab2b784476fc4ca42e3f131a50a','site:d78e9c7941411d64be89f01d63efe276f19ced962d8bfc5aa1b6e78e455080e9']:
  print(g['site'],g['truncated']);print([(x['depth'],n[x['id']]['package'],n[x['id']]['function']) for x in g['nodes']])
with open(b+'site-index.md','w') as f:
 f.write('# Relevant analyzer sites\n\nSource: writeheader-callers.json; all use net/http.(ResponseWriter).WriteHeader with interface_dispatch uncertainty.\n\n')
 for s in d['sites']:
  if 'reverseproxy' in s['package'] or 'webtransport' in s['package'] or s['location']['file'].endswith('/caddyhttp/server.go'):
   f.write(f"- `{s['id']}` — `{s['function']}` at `{s['location']['file']}:{s['location']['line']}`\n")
PY
rg -n '100 Continue|StatusContinue|ExpectContinue' modules/caddyhttp/reverseproxy modules/caddyhttp/server.go; rg -n '^func Test|handle_response|copy_response' caddytest/integration/reverseproxy*test.go | tail -35; git status --porcelain; date +%s
```

### 37. write; error=False; duration_ms=1
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/candidates.md`

### 38. read; error=False; duration_ms=2
```json
{"path": "caddytest/integration/reverseproxy_test.go", "offset": 999, "limit": 112}
```

### 39. read; error=False; duration_ms=1
```json
{"path": "caddytest/integration/reverseproxy_extended_connect_test.go", "offset": 1, "limit": 175}
```

### 40. read; error=False; duration_ms=1
```json
{"path": "modules/caddyhttp/responsewriter.go", "offset": 217, "limit": 30}
```

### 41. read; error=False; duration_ms=2
```json
{"path": "modules/caddyhttp/reverseproxy/incremental_test.go", "offset": 420, "limit": 47}
```

### 42. bash; error=False; duration_ms=88
```sh
{ pwd; git rev-parse HEAD; git status --porcelain; printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"; shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly; go version; go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector; /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help; } > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/provenance.txt 2>&1; date +%s
```

### 43. write; error=False; duration_ms=1
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/commands.md`

### 44. write; error=False; duration_ms=1
Artifact: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/report.md`

### 45. bash; error=False; duration_ms=94
```sh
wc -w /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/report.md; date +%s; git rev-parse HEAD; git status --porcelain; python3 - <<'PY'
import json
b='/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/'
for name in ['http-sites','writeheader-callers','upgrade-sites','upgrade-unresolved']:
 d=json.load(open(b+name+'.json'));print(name,len(d['sites']),len(d['nodes']),len(d['edges']),d['truncated'])
PY
```

### 46. read; error=False; duration_ms=1
```json
{"path": "/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/report.md", "offset": null, "limit": null}
```

### 47. bash; error=False; duration_ms=51
```sh
python3 - <<'PY'
from pathlib import Path
p=Path('/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/report.md')
s=p.read_text();print(len(s.split()),len(s));import time;print(time.time())
PY
```

### 48. bash; error=False; duration_ms=85
```sh
python3 - <<'PY'
from pathlib import Path
import json,time
b=Path('/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C')
t=json.loads((b/'timing.json').read_text());now=time.time();elapsed=now-t['start_epoch']
p=b/'report.md'
with p.open('a') as f:
 f.write(f'\n**Completion:** read-only investigation and report complete at epoch {now:.3f}, {elapsed:.1f} seconds after supplied launch ({t["start_epoch"]}); within the 300-second cap. New source investigation stopped before the +240-second deadline. Final HEAD/status verification remained pinned and clean. Required-scope loading valid; runtime behavior and proposed tests remain unverified.\n')
print(f'report={p}; elapsed={elapsed:.1f}s; words={len(p.read_text().split())}')
PY
```

## Analyzer artifact coverage

### `http-sites.json`
```json
{
  "coverage": {
    "packages_selected": 22,
    "source_functions": 1213,
    "operation_sites_identified": 136,
    "selected_operation_sites": 84,
    "dependency_operation_sites": 52,
    "unresolved_sites_identified": 7166,
    "sites_matching_filters": 38,
    "sites_emitted": 38
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": true
  },
  "emitted_sites": 38,
  "emitted_unresolved": 0
}
```

### `upgrade-sites.json`
```json
{
  "coverage": {
    "packages_selected": 22,
    "source_functions": 1213,
    "operation_sites_identified": 136,
    "selected_operation_sites": 84,
    "dependency_operation_sites": 52,
    "unresolved_sites_identified": 7166,
    "sites_matching_filters": 0,
    "sites_emitted": 0
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": false
  },
  "emitted_sites": 0,
  "emitted_unresolved": 0
}
```

### `upgrade-unresolved.json`
```json
{
  "coverage": {
    "packages_selected": 22,
    "source_functions": 1213,
    "operation_sites_identified": 136,
    "selected_operation_sites": 84,
    "dependency_operation_sites": 52,
    "unresolved_sites_identified": 7166,
    "sites_matching_filters": 0,
    "sites_emitted": 0
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": false
  },
  "emitted_sites": 0,
  "emitted_unresolved": 0
}
```

### `writeheader-callers.json`
```json
{
  "coverage": {
    "packages_selected": 22,
    "source_functions": 1213,
    "operation_sites_identified": 136,
    "selected_operation_sites": 84,
    "dependency_operation_sites": 52,
    "unresolved_sites_identified": 7166,
    "sites_matching_filters": 29,
    "sites_emitted": 29
  },
  "truncated": {
    "sites": false,
    "edges": false,
    "depth": false
  },
  "emitted_sites": 29,
  "emitted_unresolved": 0
}
```

