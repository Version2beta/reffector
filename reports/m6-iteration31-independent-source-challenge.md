# M6 iteration 31 — independent source/control-flow challenge

## Scope and inputs

Reviewed the accepted isolated result at `reports/m5-iteration30-real-investigation/result.json` (candidate/site `3e6912d4…`, hypothesis: cleanup is shown on write-error paths but all-later-exit cleanup is not established), its packet at `request.json`, and the exact source reference `src:loading-go-60-88`. Packet SHA-256 is `10a6d596e03185eba584ffa3e1156973c641e38addea3e0e5584048ba187c286`; current `internal/analyzer/loading.go` has that same digest and no diff. The model's cited evidence `ev:operation` is exactly the packet's one effect-site evidence ID; its IDs and schema match the packet. This checks artifact/source binding, not correctness of the model's interpretation.

## Independent challenge

The model's concern about later exits is contradicted by broader caller source, omitted from its 29-line excerpt: `internal/analyzer/analyzer.go:139-143` calls `loadConfig`, returns on error, and immediately registers `defer cleanup()` before `packages.Load`. Consequently every ordinary return from `AnalyzeReport` after successful `loadConfig` has the returned cleanup scheduled. Separately, `internal/analyzer/loading.go:78-93` invokes cleanup before all three post-`MkdirTemp` error returns and returns the cleanup closure on success. This source/control-flow check independently challenges the accepted hypothesis; it is not a runtime test.

Classification by claim:
- **Contradicted (static):** later ordinary exits from `AnalyzeReport` lack cleanup registration. The caller's immediate defer contradicts this concern.
- **Supported-static:** cleanup is invoked on the shown workspace-file error paths and handed to the caller on success.
- **Plausible-unverified / unresolved:** whether `os.RemoveAll` succeeds in any particular runtime situation. Its error is discarded; no filesystem failure injection or runtime confirmation was done. This does not establish a defect.

The accepted result itself already lists caller context as an alternative and says the excerpt omits later control flow. It is appropriately cautious, but its broad all-exit concern loses force once caller source is included. Do not promote the candidate edge into an established-call claim; this validation relies on direct source/caller code, not graph reachability.

## Reproduction

From repository root:

```sh
shasum -a 256 internal/analyzer/loading.go
# 10a6d596e03185eba584ffa3e1156973c641e38addea3e0e5584048ba187c286
git diff -- internal/analyzer/loading.go
rg -n 'loadConfig\(|defer cleanup\(\)|cleanup\(\)' internal/analyzer/analyzer.go internal/analyzer/loading.go
PYTHONPATH=tools/effect_investigation python3 -m unittest test_model_investigation test_investigate
```

The focused tests exercise rejection of an invented evidence ID (`edge:invented`), candidate/site ID mismatch, malformed schemas, and candidate-VTA/function-value non-promotion in the packet projection fixture. They are negative boundary tests, not a semantic verifier proving arbitrary model interpretations false. No files in project source or tests were changed this iteration.

## Gate status

M6 remains **pending**. This iteration provides a source-linked independent challenge of one real accepted result and records differentiated claim statuses. Before M6 passes, add/verify an explicit independent verifier test that rejects a result promoting a candidate-only relationship to an established call (and ensure fabricated path/API claims cannot pass as source citations); the current generic schema/citation boundary and M4 projection assertions do not fully demonstrate semantic rejection of such a result. No model call this iteration; cumulative real attempts remain four, one accepted (iteration 30), with no new usage.
