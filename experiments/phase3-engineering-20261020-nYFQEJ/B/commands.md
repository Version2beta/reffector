# Literal analyzer commands and execution record

Working directory for every invocation: `/Users/rob/Development/reffector/experiments/restic`.
Before every analyzer invocation and Go provenance command:
```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly
```
All analysis used default darwin/arm64 build selection, no tags and no `-tests`. Source test files were read, not executed. Successful analyzer loading served as dependency/type-loading preflight; no separate go list, downloads or repairs.

Commands in execution order (stdout/stderr saved to same-named `.json`/`.stderr` files under B; the initial rejected command used `fs-survey`):

```sh
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect fs. -max-depth 0 -max-sites 45 -max-edges 0 -stats -json ./internal/repository/... ./internal/backend/local/...
# exit 2: CLI rejected nonpositive edge limit; no load, not a scope-load failure
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect fs. -max-depth 0 -max-sites 45 -max-edges 50 -stats -json ./internal/repository/... ./internal/backend/local/...
# fs-survey-valid; exit 0
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -operation Remove -reason interface_dispatch -max-depth 2 -max-sites 15 -max-edges 50 -stats -json ./internal/repository/... ./internal/backend/local/...
# remove-boundaries; exit 0
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -site site:9a8cbcbfba99c06fc0ee41fa4c19d4b53c29a2e634c66fca321f022a51a01a3c -max-depth 2 -max-sites 5 -max-edges 30 -stats -json ./internal/repository/... ./internal/backend/local/...
# save-rename; exit 0
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -operation List -function decidePackAction -max-depth 2 -max-sites 8 -max-edges 30 -stats -json ./internal/repository/... ./internal/backend/local/...
# prune-list; exit 0
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -unresolved -site site:cb4d488e0c3add7444053d993ca299849465a2f6afb55666531c107fa0850912 -max-depth 2 -max-sites 3 -max-edges 45 -stats -json ./internal/repository/... ./internal/backend/local/...
# list-callers; exit 0
```

Provenance commands (also in session transcript; observations retained in provenance.txt and help.txt):
```sh
pwd
git rev-parse HEAD
git status --porcelain
printf 'PI_PROVIDER=%s\nPI_MODEL=%s\nPI_REASONING_LEVEL=%s\nPI_SESSION_ID=%s\nPI_SESSION_FILE=%s\n' "$PI_PROVIDER" "$PI_MODEL" "$PI_REASONING_LEVEL" "$PI_SESSION_ID" "$PI_SESSION_FILE"
shasum -a 256 /Users/rob/.agents/skills/reffector/SKILL.md /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector
go version
go version -m /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector
/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -help
```
Go commands used the export above, including the first explicit environment-prefixed invocations. Help was inspected initially and captured again without loading packages. Raw JSON is intact; compact Python summaries were used for inspection, with no analyzer output rewritten. Exact redirections and source search/read commands are retained in the session transcript. One exploratory JSON-summary script mistakenly iterated unresolved_sites as objects; it failed locally and was corrected by inspecting sites. This did not affect analyzer results.
