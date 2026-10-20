# Case A — Architectural simplification

## Outcome

**Propose sharing custom DNS resolver construction within `reverseproxy`; retain independent HTTP response state machines.** This is a source-supported proposal, not an implemented or verified improvement. DNS connections support consequential HTTP upstream selection and dialing; this is not a defect finding or a claim that fewer sites are inherently better.

## Run and provenance

Repository: `/Users/rob/Development/reffector/experiments/caddy`, clean pinned HEAD `0e392c89baa50f558bc24440482c5d57fff4499f`; scope `./modules/caddyhttp/...`. Initial and subsequent status checks were empty. No source edits, application execution, tests, network loads, or remote mutations occurred. All writes are evaluation artifacts in `A/`.

Frozen executable: `/Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector`; SHA256 `ad9f0ac67b2d512880197e24e4b6be7077553497750380a4af7bc6c8c138ff97`. `go version -m` reports revision `5553a1d13011bf638148661f6a23bf561c3eef5a`, `vcs.modified=true`, module version `v0.0.0-20261016233039-5553a1d13011+dirty`, built with Go 1.27.2, darwin/arm64, CGO enabled. This is the supplied dirty Phase 1/2 build, not a clean-revision artifact. Installed Go is also 1.27.2. Full build information and actual help are retained in `provenance.txt` and `help.txt`.

Fully read skill: `/Users/rob/.agents/skills/reffector/SKILL.md`, phase2-1; verified SHA256 `ea99f629183bfafaf5764bc9eee86902f5e574a39a2a55ebdf41284146173297`.

Selected Pi identity: provider `openai-codex`, model `gpt-6.1-sol`, reasoning `high`, session `01a15a5c-bb81-7692-9ce3-17506538a1a1`; session file is under `A/sessions/` (full value recorded in provenance). These environment values identify Pi's selection, not an independently attested backend. Loaded context: default Pi instructions, supplied global AGENTS guidance, Caddy `AGENTS.md`, explicit skill, and `.github/CONTRIBUTING.md`. Tools: read/bash/write; no extensions, MCP, other skills, other cases, or prior reports. Direct execution was explicitly requested. Tokens/cost unavailable.

## Prioritized candidates and evidence

### 1. Custom resolver construction — recommend bounded consolidation

[R] `connect.json` and `dial-sites.json` identify three independent calls to `net.(*Dialer).DialContext`:

| Source | Canonical site ID |
|---|---|
| `reverseproxy/upstreams.go:110`, `(*SRVUpstreams).Provision$1` | `site:b78bf8f18398362a2f11076cbaf7b6fced2393387cabae9d8f11876abd2d5fc4` |
| `reverseproxy/upstreams.go:268`, `(*AUpstreams).Provision$1` | `site:17934aed1822adc4463c0992fadf6890f2f8d6488c4b634336355a814dcbedcd` |
| `reverseproxy/httptransport.go:272`, `(*HTTPTransport).NewTransport$1` | `site:942e698c9bd26db1dc70801907c3999f6e3ab09072f2d9a006818dd09e4d61b0` |

All paths above are under `modules/caddyhttp/`. [F] Each independently parses configured resolver addresses, constructs a DNS dialer with timeout/fallback delay, sets `PreferGo`, randomly selects a configured endpoint, and forwards context plus that endpoint's network/address. Address parsing is **already shared**, at `UpstreamResolver.ParseAddresses`, `upstreams.go:393`; consolidate construction rather than duplicating or replacing parsing.

[F] Source relationships: SRV/A `Provision` install `net.Resolver.Dial` callbacks; `GetUpstreams` uses their resolver's `LookupSRV`/`LookupIP` (`:134`, `:286`). HTTP `NewTransport` installs the resolver on its destination dialer, whose callback becomes `http.Transport.DialContext` (`httptransport.go:410`). [R] Reverse graphs show callback origins without caller edges for these three sites. Assignment is not invocation evidence; actual DNS callback execution remains a boundary.

**Before:** three implementations separately maintain resolver construction and DNS connection policy. **After:** one unexported construction helper in `upstreams.go` owns parsing, DNS dialer settings, endpoint choice, and context forwarding. Callers retain their nonnil guards, module defaults, resolver assignment, and lookup/caching behavior. Do not share resolver instances globally.

Proposed signature: `newUpstreamResolver(current func() *UpstreamResolver, timeout, fallbackDelay time.Duration) (*net.Resolver, error)`. The small accessor preserves existing callbacks' late reads of `su.Resolver`, `au.Resolver`, or `h.Resolver`; silently capturing a fixed receiver could change behavior if that field is replaced. Parse once during construction; consult the accessor during dialing as today. This added callback is a tradeoff for behavioral fidelity, not a new policy layer. If provisioned configuration immutability is established, a receiver method would be simpler.

[F] Preserve important differences: HTTP defaults timeout to three seconds (`httptransport.go:224`); SRV/A leave zero unchanged. Their nil/default-resolver handling remains outside the helper. Ordinary HTTP destination dialing (`:292`) additionally handles proxy routing, `DialError`, connection cleanup, and PROXY protocol: **exclude it**. FastCGI's `fastcgi.go:214` is likewise not custom DNS construction.

Falsifiers: legitimate divergence in DNS policy, required mutable-field behavior not preserved by extraction, or helper complexity outweighing three short implementations. Empty configured address behavior must not be silently changed; new validation would be separate work.

### 2–4. Lower-value or rejected consolidation

[R/F] Encode `Write`/`Close` independently emit deferred explicit status and mark headers written (`encode.go:411/:497`; IDs in `encode-headers.json`). A small private commitment helper is possible, but body sniffing versus header-only initialization remains independent. Lower payoff; do not merge all five header sites.

[R/F] Recorder, headers, and encode wrappers have distinct buffering/header-mutation/compression obligations. Recorder treats 101 as final; headers treats all 1xx as informational. Reject a common response state machine based merely on shared `WriteHeader` APIs.

[F] Intercept and proxy both select response handlers, but local buffered-body fallback differs from upstream body ownership, retries, trailers, and upgrades (`intercept.go:114–207`; `reverseproxy.go:1317–1499`). Reject broad unification. Details and evolving priorities are in `candidates.md`.

## Preservation evidence and proposed validation

[F] Existing inspected tests cover IP-version selection (`upstreams_test.go`), HTTP transport/TLS/PROXY protocol setup (`httptransport_test.go`), and dynamic host state (`dynamic_upstreams_test.go`). Resolver-specific search found a parsing benchmark (`upstreams_bench_test.go:19`), not a focused construction test. Encode conformance tests cover HEAD, 304, range, WebSocket, minimum length, and cache semantics. None were executed; these observations do not establish passing coverage.

Proposed tests: identical construction/settings and parse failures across all three callers; unchanged nil/default behavior and timeout defaults; loopback DNS fixtures for UDP/TCP, cancellation, endpoint membership and network/address forwarding; replacement of the resolver configuration after construction; concurrent lookups under race detection. Avoid probabilistic endpoint-frequency assertions. Verify HTTP destination error wrapping/cleanup and SRV/A caching remain untouched. Characterize empty-address behavior separately before changing any failure rule.

## Analyzer commands and limitations

All commands ran from the repository cwd. Literal load commands (stdout/stderr retained in correspondingly named artifacts):

```sh
env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -max-depth 0 -max-sites 65 -max-edges 0 -stats -json ./modules/caddyhttp/...
env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -max-depth 0 -max-sites 65 -max-edges 1 -stats -json ./modules/caddyhttp/...
env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -api 'net/http.(ResponseWriter).WriteHeader' -package /encode -max-depth 2 -max-sites 20 -max-edges 50 -stats -json ./modules/caddyhttp/...
env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -effect net.connect -max-depth 2 -max-sites 20 -max-edges 60 -stats -json ./modules/caddyhttp/...
env GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly /Users/rob/Development/reffector/experiments/phase3-engineering-20261020-nYFQEJ/bin/reffector -api 'net.(*Dialer).DialContext' -max-depth 3 -max-sites 10 -max-edges 40 -stats -json ./modules/caddyhttp/...
```

First invocation rejected zero edge limit (exit 2), before loading; corrected invocation and all subsequent loads exited 0. All four successful loads selected 22 packages/1,213 functions; 136 identified operation sites comprise 84 selected and 52 dependency sites. Survey emitted 65/136 with site/depth truncation. Encode emitted 5/5, no reported truncation, but its package filter restricts callers and all five operations have `interface_dispatch` uncertainty. Connect emitted 10/10 (seven selected, three dependency), with depth truncation. Exact DialContext query emitted 5/5 selected sites, no reported truncation; missing callback edges still prevent completeness claims. Tests were not loaded.

Provenance ledger: [R] raw analyzer JSON/stderr; [F] cited source follow-up plus `responsewriter.go`, `headers/headers.go`, `encode.go`, and test files above; [I] no independent finding promoted. Tool transcript retains exact inspection commands. Other effect groups, dependency implementations, and platform variants remain unexplored. No implementation diff exists; pinned history remains untouched. Evaluation complete within budget; final timing/status recorded in `completion.json`.
