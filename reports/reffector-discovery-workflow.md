# Discovery workflow: operations first, source investigation next

## 1. Earlier workflow: evidence versus reconstruction

**Decision:** use the existing CLI interactively, not a canonical packet for every
candidate. Effects identify places to investigate, not defects. This report is
Phase 2 preparation and a known-case retrieval smoke, **not blind discovery**.

Public evidence was retrieved by unauthenticated GET on **2026-10-29,
01:27:20–01:27:29 UTC**. The evidence root for this run is
`/tmp/reffector-discovery-pivot.upub0G` (called `$A` below; `/private/tmp` is the
same macOS location). `public/retrieval.json` records each URL, retrieval time,
HTTP status, selected response headers, size and artifact. Each request had a
25-second timeout and 2 MB response cap. No remote mutation or dependency
network access occurred. The following are **metadata facts at retrieval**:

| PR | State | API base SHA | API head SHA |
| --- | --- | --- | --- |
| [Caddy #8269](https://github.com/caddyserver/caddy/pull/8269) | closed, `merged=true`, merged 2026-10-24T22:19:19Z | `8e237f195506741d3109ba7792e81d57a5f65036` | `b45acbf6593e52d3c76e6a78e036d954fd48f3f3` |
| [rclone #10172](https://github.com/rclone/rclone/pull/10172) | open, `merged=false` | `81e7372f6fbedfa80592e9a564593d63a8dc32f3` | `eaadb88cd150523db72b26c4a160c1b1cf50db2f` |
| [restic #22136](https://github.com/restic/restic/pull/22136) | open, `merged=false` | `dd09b6bd01af220b1d5d95c0d4d7d472e05e05c4` | `f9d2d3b2603fe88d49e2d5f3850b3ca8e3de78de` |

For each PR, `public/{caddy-8269,rclone-10172,restic-22136}.{json,commits.json,files.json,diff}`
contains the full captured body/metadata, commit list, changed-file records and
raw diff. API provenance is `https://api.github.com/repos/OWNER/REPO/pulls/NUMBER`,
plus `/commits` and `/files?per_page=100`; diff provenance is the public PR URL
plus `.diff`. Captured lists contain respectively 3/1/2 commits and 2/2/3 files,
matching the metadata totals; no pagination was needed. Caddy's merge establishes
upstream merge status, **not semantic validation**. Neither other PR is accepted
upstream by this evidence.

Commit histories returned by those endpoints, in order:

- Caddy: `923c3c3b20424c1ab0a0511c40f162d182da6ead` (setup-failure cleanup),
  `ddfedc0bd037223f7a7e6026679a234cabd81738` (post-hijack tests; remove once-close
  wrappers), `b45acbf6593e52d3c76e6a78e036d954fd48f3f3` (merge master).
- rclone: `eaadb88cd150523db72b26c4a160c1b1cf50db2f` (parallel-reader failure
  and cancellation handling).
- restic: `a76b3bb6cb0a6ecc0a0f208d7822151759020708` (atomic-backend recovery
  guard), `f9d2d3b2603fe88d49e2d5f3850b3ca8e3de78de` (changelog).

### What these cases actually document

- **Caddy:** the body describes upstream response-body ownership and client
  connection ownership after hijacking, then early setup failures before tunnel
  handoff. The *final diff*, unlike the original body, uses a deferred body/socket
  cleanup guard and ownership flag, **not `onceCloseConn` wrappers**. Tests cover
  protocol mismatch and post-hijack handshake-flush / buffered-backend-write
  failures, using tracking, injected errors and peer EOF checks. Those tests were
  read, not run here. Local first-fix parent
  `f355b8687d5582ea9fcc99f71598109d1e5b6ced` has the early returns in
  `modules/caddyhttp/reverseproxy/streaming.go`; the API's newer base/head objects
  were unavailable locally. That parent is not misrepresented as the API base.
- **rclone:** the body and diff document a producer writing an error channel while
  the consumer waits for more buffered bytes under the reader lock. The patch
  publishes producer completion, handles short EOF, cancels waits before taking
  the close lock, and also fixes outer `Read` error shadowing. Added tests cover
  producer open/short/read failures for read/seek, and cancel/close scenarios.
  This is **`fs/chunkedreader/parallel.go`, not `averageLoop`**. Source follow-up
  below qualifies the body's simplified description of waiting.
- **restic:** the body and `internal/repository/upgrade_repo.go` show an upgrade
  failure followed by unconditional removal of the original config before a
  recovery save. The forward path already checks `HasAtomicReplace`; the patch
  applies that check to recovery. The added local-backend test injects failing
  config saves and checks original bytes and reopenability. It was read, not run
  here. Actual backend atomicity/failure contracts still matter; a remove or save
  operation alone cannot establish config loss.

The bounded tracked-document search (`local/tracked-case-search.txt`), last 12
report/doc history commits, and last eight relevant file-history entries per
external project did **not** locate original agent investigation transcripts for
these PRs. Excluded journals/raw provider logs were not read. **Exact initial
prompts and initial Reffector outputs are unknown**, as are original operation
selection order, source/caller queries, rejected hypotheses, timing and tokens.
Caddy's follow-up commit discloses AI assistance; that does not establish which
analyzer output, if any, initiated the work. PR prose is not a transcript.

A plausible *reconstruction*, not a historical claim, is: select operations;
read ownership/error/cancellation policy in their enclosing functions; trace
callers and helper contracts; reject incorrect explanations; form a narrow
hypothesis; independently test it. Common mechanism: compare a consequential
operation with the state/ownership/progress obligation on success **and failure**.
No forced shared bug taxonomy is needed. Analyzer locations/reachability can
help select and navigate; ordering, ownership and recovery semantics come from
ordinary source/contracts and tests.

The separate historical `reports/rclone-average-loop-dependency-retry.md` records
a tested group-start regression at `c1b5756e...` / `c91c4cbb...`, a candidate RC
edge and an incomplete canonical build. It is not evidence about PR #10172.
Existing M5/M8 reports likewise do not reconstruct these PR investigations.

## 2. Current capabilities, checked against this binary

Built from clean source `3165846d1f2fec3c40c1d9590dcb53a714203dd9` on
`feat/effect-directed-discovery`. Binary SHA-256:
`779a567802ffd370b58a072545024eab3719632544aa4a99aa0b98149810a383`.
`build/binary-identity.stdout` reports `vcs.modified=false`, Go `go1.27.2`,
darwin/arm64. The installed `/Users/rob/.agents/skills/reffector/SKILL.md` was
loaded and preserved as `local/loaded-SKILL.md`; `build/help.stderr` is actual help.

Available now: text and effect JSON; category/API/site/operation filters;
reverse node/edge references with established versus candidate resolutions;
coverage and report/per-site truncation; `-all` source inventory; optional test
loading; unresolved audit JSONL. `-api` is exact, `-operation` an API substring,
`-effect` a category substring. `-site` uses the emitted canonical ID, not a
same-spelled function name. Depth zero suppresses caller expansion, not loading.
`-direct` restricts origins to selected roots, not all reachable dependencies.

`-audit-unresolved -audit-include-known` streams uncertain origins, including
known-effect uncertain sites. It has **no site limit or reverse graph**; use a
narrow scope, external guard and inspect its coverage on stderr. It does not
replace source or callers. Candidate edges remain candidates. Contract-compatible
implementations are optional leads, not proven receivers. Neither empty output
nor `uncertain=false` proves purity or correct behavior.

## 3. Useful, optional and unnecessary components

- **Useful now:** CLI, shell, a compact site table, source search/read, Git-pinned
  disposable snapshots, a command/evidence log, human selection of a few leads,
  and independently reviewed verification.
- **Optional when a particular question needs them:** audit JSONL, test-source
  loading, bounded dependency VTA, contract leads, argument provenance or SSA
  control-flow projections. Source inspection is often the simplest next query.
- **Unnecessary for this workflow:** packets for all candidates, candidate-ranking
  redesign, a new CLI, dependencies, agent orchestration, context/memory services,
  or another unattended investigation loop. No real tooling gap justified adding
  a repository script.

The newer packet/model/projection machinery remains unchanged as an experimental
alternative. Its expanded-endpoint/model-wrapper incompatibility and canonical
full-build timeout remain checkpoint limitations, not fixes or prerequisites here.

## 4. Smallest runnable process

Build once; use a verified disposable source snapshot with a justified scope.
These commands show the actual successful scope/cache used here (no installs):

```sh
REFFECTOR_SRC=/Users/rob/Development/reffector-discovery-pivot
A=/tmp/reffector-discovery-pivot.upub0G
export GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local
cd "$REFFECTOR_SRC"
go build -mod=readonly -o "$A/build/reffector" ./cmd/reffector
R="$A/build/reffector"
"$R" -help
cd "$A/rclone-pre"
go list -mod=readonly -deps ./fs/chunkedreader
export GOFLAGS=-mod=readonly
```

Do not overwrite this run's evidence files when replaying: choose a fresh `$A`,
copy/archive the pinned source there, and rebuild there. The snippets below are
CLI choices, **not permission to omit external bounds**. This run wrapped every
analyzer invocation with preserved `bounded_run.py`:

```sh
# From the pinned snapshot; NAME must be fresh.
python3 "$A/bounded_run.py" "$A/smoke/NAME.result.json" \
  "$A/smoke/NAME.stdout" "$A/smoke/NAME.stderr" -- \
  env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly \
  "$R" -direct -max-depth 0 -max-sites 80 -max-edges 80 -json ./fs/chunkedreader
```

That external guard enforces 270 seconds and sampled **3.5 GiB direct-child
physical footprint**, kills the process group on violation, and fails closed if
monitoring is unavailable. It is not an aggregate subprocess-memory bound or a
kernel peak measurement. Very short runs may have only startup memory samples.

1. **Inventory:** the preceding invocation emits 22 selected operation sites in
   this package. Read coverage/truncation before interpreting the list. A compact
   display can be obtained without constructing packets:

   ```sh
   python3 - "$A/smoke/inventory-default.stdout" <<'PY'
   import json, sys
   for s in json.load(open(sys.argv[1]))['sites']:
       p = s['location']
       print(s['id'], ','.join(s['effects']), s['api'],
             f"{p['file']}:{p['line']}", s['function'], s['uncertain'])
   PY
   ```

2. **Human selection:** select two or three consequential leads and record why;
   do not auto-interpret every operation as a bug. In this *known-case* smoke,
   the goroutine, producer sends, and close receive are relevant navigation
   anchors; historical case knowledge guided selection.
3. **Ask for callers on demand**, preserving output separately under the same
   guard. Actual checked query arguments (each ends in `./fs/chunkedreader`):

   ```sh
   "$R" -site site:fdb372da9d59fe1ec615793c0959cf86a62dc6168f7a372ee8aa24e4cd90f871 -max-depth 4 -max-sites 1 -max-edges 80 -json ./fs/chunkedreader
   "$R" -api ssa.Send -max-depth 3 -max-sites 4 -max-edges 80 -json ./fs/chunkedreader
   "$R" -operation WaitWrite -max-depth 3 -max-sites 4 -max-edges 80 -json ./fs/chunkedreader
   "$R" -effect channel -direct -max-depth 1 -max-sites 8 -max-edges 30 ./fs/chunkedreader
   "$R" -audit-unresolved -audit-include-known -stats ./fs/chunkedreader
   ```

   The `WaitWrite` query deliberately demonstrates a **zero-match limitation**:
   that local helper is not itself an emitted operation API. Read its source;
   do not infer absence of waiting. Audit JSONL adds uncertainty leads, not every
   local helper call. To inspect emitted relationships from a site query:

   ```sh
   python3 - "$A/smoke/goroutine-tests.stdout" <<'PY'
   import json, sys
   for e in json.load(open(sys.argv[1]))['edges']:
       s = e['source']
       print(s['caller'], '->', s['callee'], s.get('resolution'),
             'candidate=', e['candidate'], s['location'])
   PY
   ```

   Test-source caller context is an explicit second scope setting: preflight
   `go list -mod=readonly -test -deps ./fs/chunkedreader`, then repeat the site
   query with `-tests`. This loads tests, **does not execute them**, and changes
   coverage. No function/package filter was used to hide callers. Incoming
   production callers outside this selected package require an explicitly wider,
   preflighted scope; these results do not establish application-wide reachability.
4. **Read source and contracts:** `sed -n '70,140p' fs/chunkedreader/parallel.go`,
   `sed -n '244,298p' fs/chunkedreader/parallel.go`, then search for `WaitWrite`
   and read `lib/pool/reader_writer.go:273-325`. Follow actual branch conditions,
   lock lifetimes, producer completion and cancellation ownership. Retrieve more
   source/call relationships as questions arise; this is not a fixed packet or
   tool-disabled session. Record what was actually consulted and contrary facts.
5. **End narrowly:** a finding needs source-grounded conditions, reachability
   qualification, a falsifiable regression and verification; otherwise return a
   justified negative or unverified hypothesis plus the missing evidence. A
   graph path is not a runtime trace. Do not run analyzed applications, repair
   dependencies or edit external sources without separate authorization.

## 5. Minimal implementation changes

Only this report is added. No Go/CLI/skill/module/test/packet/model/Ralph source,
checkpoint record, tag or history changes. Existing CLI and ordinary source tools
supply the runnable workflow. Raw inputs, snapshots, scripts and outputs remain
outside Git under `$A`; no large EffectReport or module cache is added to Git.
The temporary logging/validation scripts are evidence helpers, not a new product.

## 6. Known-case smoke: rclone #10172, pre-fix

**Pin and scope.** API base `81e7372f6fbedfa80592e9a564593d63a8dc32f3` is also
local fix `eaadb88c...`'s direct parent, verified by `git show`. `git archive`
created `$A/rclone-pre`; all **2,788 tracked blobs** matched `git ls-tree -rz`
object identities before and after analysis. Initial verification with quoted
non-ASCII `ls-tree` names failed and was corrected to NUL-delimited names before
analysis. No snapshot source was patched. `local/snapshot.json` records archive
command/hash; `local/repo-facts.json` records read-only external refs/status/remotes.
rclone and restic were clean; Caddy had only untracked `.ae/`. No external checkout
was written. Scope `./fs/chunkedreader` was chosen because the PR concerns that
reader's producer/consumer protocol; it selected exactly **one package**, not a
whole application or the historical accounting/averageLoop case.

**Offline loading.** The first dependency preflight and bounded analyzer attempt
using the prior averageLoop isolated module cache failed: versions required by
this newer pin were absent (e.g. `cpuid/v2 v2.4.0`, `go-colorable v0.1.16`). These
are preserved failures, **not zero-effect results**. After explicit supervisor
approval, the exact same pin/scope used the already populated default cache
`/Users/rob/go/pkg/mod`; dependency and test-dependency listing both passed.
`go mod verify` returned `all modules verified`. No dependency was downloaded,
installed or repaired. Offline flags were `GOPROXY=off GOSUMDB=off
GOTOOLCHAIN=local`, with `-mod=readonly` / `GOFLAGS=-mod=readonly`; the failed
attempt additionally set the recorded isolated `GOMODCACHE`. There was no scope
narrowing to suppress loading errors.

**Actual results** (`smoke/*.result.json` includes exact command/exit/bounds/time):

| Query/artifact stem | Result | Coverage / qualifications |
| --- | --- | --- |
| `inventory` (isolated cache) | exit 1, load blocked | no usable analysis |
| `inventory-default` | exit 0; 22 emitted direct sites | 24 selected functions; 67 identified operations = 22 selected + 45 dependency; 59 unresolved-only sites; site/edge truncation false, depth true by deliberate depth 0 |
| `goroutine` | exit 0; 1 site, 4 nodes, 3 established edges | all report/per-site truncation false within selected scope |
| `send` | exit 0; 2 sites, 5 nodes, 4 established edges | all truncation false; includes established `go` relationship |
| `waitwrite` | exit 0; 0 matching APIs | not evidence of no waits |
| `channel-text` | exit 0; 3 selected sites | depth truncated at 1; text exposes IDs/locations/resolutions |
| `audit` | exit 0; 65 uncertain site JSONL records | includes dependencies and known-effect uncertain sites; no reverse expansion/site cap |
| `goroutine-tests` | exit 0; 1 site, 13 nodes, 11 edges | 38 selected functions; 125 operations = 30 selected + 95 dependency; 487 unresolved-only; 8 established + 3 `candidate_vta`; all truncation false |
| `receives` | exit 0; 2 sites | includes dependency semaphore receive, not a WaitWrite summary; depth truncated at 2 |

Coverage's `emitted`/`omitted` fields describe function summary projection, not
site or graph size. Even untruncated graphs are limited by package/build scope,
recognized vocabulary and static resolution. Test variants remain distinct.

Selected evidence (all at pinned `fs/chunkedreader/parallel.go`):

| API / owner | Line | Canonical site ID |
| --- | ---: | --- |
| `ssa.Go`, `(*parallel).newStream` | 64 | `site:fdb372da9d59fe1ec615793c0959cf86a62dc6168f7a372ee8aa24e4cd90f871` |
| `ssa.Send`, `(*stream).readFrom` open-error exit | 78 | `site:539d11db7ee8ab6c9f2de4139ad98c0afcb19f7fbf7b0dfc5ebf0d217a9e0141` |
| `ssa.Send`, `(*stream).readFrom` after ReadFrom | 86 | `site:af2b43eeb25fb746e3643f1a778ebeefb07666842d67aa7499d6d2540356ac90` |
| `ssa.Receive`, `(*stream).close` | 136 | `site:0a537a39789288f8601bb424aefd0bc4adc8f51d315b80b7d983d6e1d9d60003` |
| `sync.(*Mutex).Lock`, `(*parallel).Read` | 247 | `site:000634b9136654d9c78fdeb9dd37adf229ea59d9655229fb0fb748642ff93812` |
| `sync.(*Mutex).Lock`, `(*parallel).Close` | 287 | `site:dd6184b90e7fbc05510fbab6e669b9f223e79891174d03c1323dfceb4f99a65b` |

Established reverse edges include `Read -> _open` at line 256,
`Open -> _open` at 388, `_open -> newStream` at 202; the sends additionally
retrieve `newStream -> readFrom` (`kind=go`) at 64. With test loading, actual
candidate relationships include `TestParallelLarge$2 -> (*parallel).Read` at
`parallel_test.go:77`, `testRead$1$1 -> Read` at `chunkedreader_test.go:82`, and
`testErrorAfterClose -> Read` at line 114. They are `candidate=true`,
`resolution=candidate_vta`, **not established invocations or executed tests**.
Their source locations were inspected; JSON retains endpoint/relationship IDs.

**Source follow-up, not analyzer conclusions:** `stream.read` retries buffer EOF
while expected bytes remain, then calls `WaitWrite`; producer error publication
is on `s.err`, consumed in `stream.close`, not that retry loop. `parallel.Read`
holds `cr.mu`; `Close` acquires it before stream cancellation. Crucial contrary
fact: `pool.RW.WaitWrite` returns on context, notification **or a one-second
timer** (`lib/pool/reader_writer.go:310-324`). Thus “one WaitWrite call sleeps
forever” is rejected by source. Repeated retries without a producer-completion
check remain a plausible progress failure; timer wakeup alone does not supply
missing bytes. This needs a controlled failure/cancellation test, not just a
send/lock effect. Error-shadowing behavior additionally requires lexical source
reasoning. `local/source-witnesses.json` records whole-file SHA-256 and exact
consulted ranges, with numbered witness files. Those and the PR regression diff
are evaluator evidence, **not a blind prompt**.

No external tests or application were run; no runtime hang, config loss or socket
leak is claimed reproduced. A further agent would need relevant reader/backend
contracts, admissible failing-producer scenarios, cancellation ownership and a
bounded regression test checking returned bytes/errors and termination. Ordinary
source inspection, not an added operation category, supplies those requirements.

## 7. Validation, measurements and limits

Offline validation in this worktree:

- `go build -mod=readonly -o "$A/build/reffector" ./cmd/reffector`: pass.
- Actual binary `-help`: exit 0; saved, not assumed from installed tooling.
- `go test -mod=readonly ./...`: pass (cmd, analyzer, output, skillinstall;
  effects and bundled skill have no test files); 68.0 seconds command wall time.
  Existing site/filter/reverse/candidate/audit tests run unchanged.
- `go vet -mod=readonly ./...`: pass.
- External `smoke/validate_smoke.py`: pass; asserts expected IDs/coverage,
  graph reference closure, candidate/established counts, source locations/hashes,
  truncation and successful bounded exits. Snapshot re-verification: pass.
- New-direction whitespace/index checks are recorded separately in the artifact
  root. No assertions were weakened; no repository tests were added for a
  documentation-only change. The ancestral imported-oracle EOF exception stays
  preserved, not rewritten.

Nine analyzer invocations: eight successful, one preserved dependency-blocked
attempt. Successful analyzer command wall times total **19.34 seconds** including
per-run guard overhead; these are retrieval costs, **not time-to-find-a-defect**.
There were 12 public GETs. Exact analysis/query commands and exit records, build
and test logs, source identity/hashes, consulted witnesses, and raw public captures
are in `$A`; `artifact-manifest.json` indexes the evidence. Broad environment or
credentials were not captured. No model/provider/adapters, benchmark, simulated
blind agent, Ralph loop or full candidate-packet build was invoked.

Outcome measures here: **0 independently verified new defects**; one historically
selected progress lead made navigable; one specific “single eternal WaitWrite”
explanation rejected by source. False-positive rate and incremental discovery
benefit are **unmeasured**, not zero. Source/graphs actually consulted are listed
above and in witness/command records. Model-call count is zero; token usage and
complete human investigation/tool-call time are unavailable and not estimated
from operation counts or bytes. PR status, regression test text, operation count,
packet production and static paths are not defect yield or causal benefit.

`/tmp` artifacts are local, nondurable evidence; preserve them separately before
cleanup if later review/replay is needed. Offline replay depends on already
available toolchains/modules. The direct-child sampled memory guard does not
bound aggregate Go helper memory. No whole-application caller completeness,
path feasibility, arbitrary state/dataflow coverage or production behavior is
claimed. Existing model/packet limits remain exactly as checkpointed.

## 8. Controlled source-only comparison plan — not run

Pre-register one narrow, finite human-reviewed investigation before any future
model call. Prefer an unseen case; using these known PRs permits only a labeled
retrospective exercise. Freeze source/build identity and keep evaluator-only
case knowledge, fixes, new regression tests, PR metadata and this report hidden
from investigators. Both arms receive identical pre-fix source access, existing
pre-fix tests, generic task, scope, source-search/read tools, wall/call/output/token
budgets and verification rights. Treatment additionally may request the existing
bounded CLI inventory and follow-up site/API/caller/audit queries; control may
use ordinary source tools. Do not give treatment extra hidden source excerpts or
freeze it into a tool-disabled packet. Log all requested/actually consulted
source, graph relationships, coverage and omissions. Randomize order/assignment;
prevent cross-arm contamination; keep evaluator knowledge out of either prompt.

Candidate generic prompt, stored separately from case facts when registered:

> Review the pinned Go package for actionable correctness issues. Select only a
> few leads. Inspect relevant source and callers, seek contrary evidence, and
> return concise findings or a justified negative. For each hypothesis identify
> conditions, supporting/contrary evidence, uncertainties and a falsifiable
> verification step. Static reachability is not execution. Stay within the
> approved read/tool budget; do not modify source or dependencies.

Independent review should score unique independently verified defects, useful
leads, rejected/unsupported hypotheses and false positives; source/graph actually
consulted; elapsed effort, calls and actual token accounting; and incremental
benefit over source-only **with provenance**. A verifier, not either arm's claim,
decides defect confirmation. Predefine failure/timeout handling and report null
results. No broad benchmark or model invocation is authorized by this plan.

## 9. Next concrete experiment

After independent review of this unstaged report, preserve the temporary evidence
and choose one approved narrow investigation with an explicit time budget. First
confirm a bounded pre-fix/fix regression independently in disposable source,
without showing the fix/test to investigators. Then register and run at most one
matched source-only versus interactive-Reffector comparison under equal budgets
and the generic prompt above. Stop if dependencies, caller evidence or regression
ground truth are unavailable; do not substitute canonical packet throughput for
useful discovery. No commit, tag, push, branch change or external source edit was
performed in this run. Proposed eventual commit: `docs: prepare interactive
operation-directed discovery workflow`.
