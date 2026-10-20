# Candidate ledger

## 1. Shared custom DNS resolver construction — strongest, explored
[R] connect.json identifies independent net.(*Dialer).DialContext sites in SRVUpstreams.Provision$1 (upstreams.go:110, site:b78bf8f18398362a2f11076cbaf7b6fced2393387cabae9d8f11876abd2d5fc4), AUpstreams.Provision$1 (:268, site:17934aed1822adc4463c0992fadf6890f2f8d6488c4b634336355a814dcbedcd), HTTPTransport.NewTransport$1 (httptransport.go:272, site:942e698c9bd26db1dc70801907c3999f6e3ab09072f2d9a006818dd09e4d61b0).
[F] All three repeat ParseAddresses, Dialer timeout/fallback setup, PreferGo, random configured endpoint selection, network/address forwarding and context propagation. Parsing is already shared in UpstreamResolver.ParseAddresses (:393). Final proposal: unexported newUpstreamResolver factory taking an explicit current-resolver accessor and durations, used only inside existing nonnil branches. The accessor retains existing late reads of each module Resolver field; a receiver-only factory would need an established immutability contract. Keep module defaults/nil semantics and ordinary HTTP destination dialing independent. No global cache/singleton. DNS is a prerequisite of HTTP routing, not response output itself. No defect claim.
Unresolved: empty configured resolver handling and post-provision mutation contract must not be silently altered; behavior-preserving extraction should retain these existing semantics, with any changed validation separately authorized.

## 2. Encode deferred header commitment — explored, lower priority
[R] encode-headers.json: independent WriteHeader API sites in Write (:411) and Close (:497), plus three immediate paths (:287/:294/:304). [F] Write/Close share conditional explicit status emission and wroteHeader bookkeeping, but encoding initialization differs (body sniffing vs content length); 1xx/CONNECT/incremental/SSE paths have different timing. Tiny private helper could unify only the deferred commitment invariant; likely lower payoff than resolver construction. Reject broad all-header-state-machine merger. Existing response conformance tests inspected, not run.

## 3. Cross-module response wrappers — rejected
[R] survey WriteHeader sites in recorder/headers/encode. [F] recorder buffers and treats 101 as final; headers applies conditional operations and treats all 1xx as informational. Similar API sites represent different responsibilities. Keep separate.

## 4. Intercept vs reverseproxy response handling — rejected
[F] Both select response handlers, but intercept handles buffered local output with fallback body; proxy owns upstream body lifetime, retry classification, trailers and upgrades. Shared matching loop would not remove these independent correctness obligations.

Preflight: required HEAD clean; binary/skill hashes match. Initial survey invocation rejected max-edges=0 (exit 2); corrected to 1, successful full required scope load (22 packages). Raw outputs retained. Survey limited to 65/136 sites, depth 0; not a completeness claim. Connect query emits all 10 matches (7 selected, 3 dependency); depth truncation exists. Encode package-filter query emits 5 matches without reported truncation but excludes external callers and cannot resolve concrete ResponseWriter implementations.

Final gate: proposal only; no implementation or test success claimed. Full report saved. Final git status and diff were empty. No independent [I] finding promoted.
