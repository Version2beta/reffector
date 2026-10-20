# Commands and execution boundary

Cwd for every command: `/Users/rob/Development/reffector/experiments/caddy`.
Every Go/Reffector invocation used:

```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly
```

Analyzer invocations (all exited 0; stdout and stderr preserved separately):

```sh
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect http -max-depth 0 -max-sites 45 -max-edges 80 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/http-sites.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/http-sites.stderr
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -api 'net/http.(ResponseWriter).WriteHeader' -max-depth 4 -max-sites 40 -max-edges 150 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/writeheader-callers.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/writeheader-callers.stderr
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -operation Upgrade -max-depth 3 -max-sites 12 -max-edges 60 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/upgrade-sites.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/upgrade-sites.stderr
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -operation Upgrade -max-depth 3 -max-sites 12 -max-edges 60 -stats -json ./modules/caddyhttp/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/upgrade-unresolved.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/C/upgrade-unresolved.stderr
```

Provenance commands (initial and retained repeat in provenance.txt):

```sh
pwd
git rev-parse HEAD
git status --porcelain
printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"
shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector
go version
go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help
go env GOMODCACHE
```

Source reads/searches and exact harness tool calls are in the session transcript. No `-tests` loads, test runs, application execution, dependency downloads, builds, module edits or worktree edits. No history changes made; Case E historical-diff instruction is not applicable to Case C.
