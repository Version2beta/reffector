# Commands and execution evidence

Working directory for every command: `/Users/rob/Development/reffector/experiments/restic`.
Every analyzer load exited 0. No `-tests`, `-all`, GOOS/GOARCH override, or build tags. Native Go build: darwin/arm64, Go 1.27.2. JSON is stdout, coverage stderr. Six analyzer loads; no tests/app execution. Shell inspection commands and exit statuses also appear in session transcript.

Literal analyzer command lines (as executed):

```sh
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect fs. -max-depth 0 -max-sites 25 -max-edges 30 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/survey.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/survey.stderr
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -function LoadUnpacked -max-depth 3 -max-sites 15 -max-edges 60 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/loadunpacked.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/loadunpacked.stderr
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -function openKey -max-depth 3 -max-sites 20 -max-edges 60 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/openkey.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/openkey.stderr
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -site site:2dad08db8468a86e9929ce0b2a30fe12e8900ba02af1715e96b1cc4cd5c1cb3e -max-depth 7 -max-sites 1 -max-edges 100 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/cache-callers.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/cache-callers.stderr
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -reason interface_dispatch -function LoadRaw -max-depth 2 -max-sites 8 -max-edges 40 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/raw-boundaries.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/raw-boundaries.stderr
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -operation os.Open -package internal/backend/local -max-depth 3 -max-sites 5 -max-edges 25 -stats -json ./internal/repository/... ./internal/backend/local/... > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/local-read.json 2> /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/local-read.stderr
```

Provenance commands (output retained in `provenance.txt`; also ran initially):

```sh
pwd
git rev-parse HEAD
git status --porcelain
printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"
shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help
```

Historical selection and preservation:

```sh
git log -18 --format='%H %s' -- internal/repository internal/backend/local
git show --stat 8baffc40273bb3aa4f6c7826d582ebe576f4c90c
git show --format=fuller --no-ext-diff 8baffc40273bb3aa4f6c7826d582ebe576f4c90c -- internal/repository
git log -8 --format='%H %s' -- internal/backend/local
git show --format=fuller --no-ext-diff 8baffc40273bb3aa4f6c7826d582ebe576f4c90c > /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/E/historical.diff
git merge-base --is-ancestor 8baffc40273bb3aa4f6c7826d582ebe576f4c90c HEAD
```

Ancestry exit 0. Historical parent `internal/repository/crypto/crypto.go` viewed using `git show` and a Python line-range printer; changelog viewed using `git show COMMIT:changelog/unreleased/pull-21977`. Source read tools, bounded rg searches, and JSON extraction are in transcript. One rg command mentioned nonexistent `key_test.go`/`config.go`; this was a search-path mistake, not an analyzer loading failure. No repairs or scope narrowing followed.
