# Phase 2 — minimal Pi skill verification

## Deliverable and versions

Canonical skill: `skills/reffector/SKILL.md`, metadata version `phase2-1`.
It replaces the prior mandatory `find`/refactoring workflow with task-neutral
evidence retrieval and interpretation. No competing skill, wrappers, analyzer
changes, dependencies or benchmark findings were added.

Validated against the **current dirty checkout**, HEAD
`5553a1d13011bf638148661f6a23bf561c3eef5a`, after Phase 1; not a published release.
Reffector has no CLI version flag. Built with `go1.27.2 darwin/arm64` using the
skill's temporary-directory build instructions. Tested executable:
`/var/folders/vw/fh_b6s2s4l154vl9dl21lx480000gn/T/tmp.AVBU7B4xoh/reffector`.
SHA-256: `77990d8161bec737ac33636c71f6bcd0078219b6657ef4a73476728e2d4e4f78`.
Skill SHA-256: `ea99f629183bfafaf5764bc9eee86902f5e574a39a2a55ebdf41284146173297`.

Pi version: **1.3.0** (`pi --version`). Read the installed `docs/skills.md`,
`settings.md`, `packages.md`, `configuration.md` and `cli.md`. Pi uses a directory
with `SKILL.md`, name/description frontmatter, and supports explicit `--skill`
paths plus `.pi/skills/` and `.agents/skills/` discovery. Its native explicit-path
loader loaded exactly this skill with no diagnostics; no model call or global
configuration change was needed. This checks loading, not future agent behavior.

## Tested commands

Evidence, baseline status/diff/source hashes, fixture and output are retained at
`/tmp/reffector-phase2.eWRrM3/`. Built from the repository root:

```sh
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
bin_dir=$(mktemp -d)
go build -o "$bin_dir/reffector" ./cmd/reffector
export PATH="$bin_dir:$PATH"
reffector -help
```

All three skill examples were run verbatim from that evidence directory's
`fixture/`, a two-package `example.com/skillsmoke` module using only cached stdlib
dependencies. Its source mirrors existing fixtures: `ops.Write` calls
`os.WriteFile`; `Left`/`Right` share it; `ops.Respond` calls `WriteHeader` with a
forwarding caller; `ops.Save` logs and calls an application `Store.Save` interface
with a forwarding caller; `ops.Close` calls `io.Closer.Close`. Nothing was executed
except Go tooling and Reffector.

```sh
go list -mod=readonly -deps ./...
go list -mod=readonly -deps -test ./...
reffector -effect fs.write -max-depth 0 -max-sites 20 ./...
reffector -api 'net/http.(ResponseWriter).WriteHeader' -max-depth 2 -max-sites 20 -max-edges 50 -json ./...
reffector -unresolved -reason interface_dispatch -max-depth 2 -max-sites 20 -max-edges 50 ./...
```

All exited 0 with empty stderr (help intentionally writes to stderr). Inspected
and asserted output: two selected packages/eight source functions; one write
site with depth truncation; one uncertain response-contract site with one caller
edge and no truncation; one effect-free unresolved Save site with one caller edge,
excluding logging. Additional successful checks selected the actual response
`-site` ID, inspected unfiltered `-unresolved` (two sites), and exercised
`-all -stats -json -exact .` (four root-package inventory entries; stats on stderr).

`HOME=<fresh temporary directory> reffector skill install` installed bytes
identical to the canonical skill. Focused installer/fixture tests passed; final
`GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local go test -count=1 ./...` passed all
packages (CLI suite 26.697s). No third-party trees or real user configuration were
changed. Existing Phase 1 work remains in place.

## Load and use

No installation is required. From the target project, run this session-only Pi
command (also avoids collision with an older globally installed skill):

```sh
pi --no-skills --skill /Users/rob/Development/reffector/skills/reffector/SKILL.md
```

Then enter `/skill:reffector` followed by the question and explicit Go scope.
The skill is self-contained and has no mandatory supporting-document reads.

**Optional user-authorized installation:** run the freshly built
`"$bin_dir/reffector" skill install`. It writes
`~/.agents/skills/reffector/SKILL.md` and installer bookkeeping; Pi discovers this
location. Existing unmanaged/modified copies are refused. Reload Pi resources
with `/reload` after an update. No real user installation was performed here.

Limitations remain explicit: dependency preflight is not type checking; failed
loading invalidates analysis. Workspace preflight should use Reffector's private
workspace loading rather than risk raw `go list` checksum writes. Results cover
the current build, recognized operations and bounded static reachability, not
execution, arbitrary dataflow/mutation, resource destinations, policy correctness
or defects. Open contracts remain uncertain; filtering/empty output is not
absence proof. Follow source for ordering, locks, authority, ownership and errors.

## Final files and checks

Phase 2 changed only:

- `skills/reffector/SKILL.md` — canonical task-neutral skill.
- `README.md` — Pi loading/install guidance, replacing stale `find` instructions.
- `man/man1/reffector.1` — matching skill description and Pi invocation.
- `skills/reffector/embed.go` — comment says agent skill rather than Codex skill.
- `PHASE2_SKILL_VERIFICATION.md` — this note.

Baseline hash comparison confirms every pre-existing file outside the four
intended documentation edits above is byte-identical. The status below includes
all pre-existing uncommitted work, not just Phase 2:

```text
$ git diff --check
(no output; exit 0)

$ git status --short
 M README.md
 M cmd/reffector/integration_test.go
 M cmd/reffector/main.go
 M internal/analyzer/analyzer.go
 M internal/effects/effects.go
 M internal/output/output.go
 M man/man1/reffector.1
 M skills/reffector/SKILL.md
 M skills/reffector/embed.go
?? .DS_Store
?? HANDOFF.md
?? PHASE1_EFFECTS_REPORT.md
?? PHASE2_SKILL_VERIFICATION.md
?? cmd/reffector/coverage_test.go
?? cmd/reffector/effectvariants_test.go
?? cmd/reffector/effectview_test.go
?? cmd/reffector/emptyeffect_test.go
?? cmd/reffector/generics_test.go
?? cmd/reffector/inventory_test.go
?? cmd/reffector/investigation_test.go
?? cmd/reffector/provenance_test.go
?? cmd/reffector/uncertainty_test.go
?? experiments/
?? internal/analyzer/details.go
?? internal/analyzer/effectview.go
?? internal/analyzer/selection_test.go
?? internal/analyzer/uncertainty.go
?? internal/output/effects.go
?? internal/output/effects_test.go
?? reffector-self.json
?? testdata/phase1/
```
