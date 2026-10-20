# Candidate ledger

## Initial survey (17:30 UTC)
Provenance verified: repository pinned and clean; executable and phase2-1 skill hashes match. Full required analyzer scope loads offline (survey exit 0).

1. **Selected, investigating** — Local.Save temporary-file publication (`local.go:107–200`): [R] rename, file sync, directory sync, deferred temporary removal. Strong preservation/cleanup/retry payoff because HasAtomicReplace is true and errors can occur after publication. Inspect existing generic/local/retry tests before claiming a gap.
2. **Unresolved secondary** — repository config-version replacement/rollback (`upgrade_repo.go`): [F] atomic replace capability gates upgrade. Might be useful caller-level validation; do not duplicate existing rollback tests.
3. **Deprioritized** — Local.Delete/Remove and directory listing: [R] destructive/read effects, but no evidence of comparably valuable replacement-state test gap. Frequency alone does not justify selection.

Survey limits: fs. filter, 24/33 matching sites emitted, depth 0, sites/depth truncated, edges not truncated. Six selected packages; 38 selected and 13 dependency operation sites identified. Effects are not findings.

## Focused follow-up (17:31 UTC)
1. **Recommended test design, no defect claim** — real local replacement with old ConfigFile and distinct new bytes. [F] Save errors before rename preserve old name; failures after rename need not restore old bytes. Cleanup removes temporary name, not final name. [F] Retry.Save rewinds on each attempt and does not Remove when HasAtomicReplace. Proposed minimal three behavioral families: pre-publication failure preservation/cleanup; successful replacement plus post-publication error semantics; real-local retry across those states and permanent errors. Reader injection and existing tempFile seam suffice for immediate cases; deterministic post-rename fsync failure needs a future narrow private seam or authorized OS fault harness. No implementation performed.
2. **Deferred, not disproved** — UpgradeRepo rollback: existing TestUpgradeRepoV2Failure asserts both upload and reupload errors and backup path, but does not inject a local post-publication failure. Repository rollback/backup correctness is a separate obligation; do not expand selected boundary to implement this.
3. **Rejected as new proposal** — another generic short-upload error-only test, another mock-only atomic no-Remove test, and another ENOSPC-at-create classification-only test: existing tests already establish these intended checks in source (not executed in this evaluation). Add real filesystem state assertions instead.
4. **Deferred compatibility matrix** — ENOTSUP/ENOTTY file fsync fallbacks, directory EINVAL/ENOENT tolerances, chmod policy, Windows directory-sync no-op. Their behavior is intentionally differentiated; avoid treating common effects as consolidation grounds. Relevant falsifiers/limits in report.

Focused save-effects query emits all seven matching filesystem sites with no reported site/edge/depth truncation. Rename-only query has no caller edge, not proof of no clients. Broad unresolved Save query was dependency-dominated (10/50, site/depth truncated); refined .Save/repository query emits 8/8 with depth truncation and identifies interface-dispatch boundary at repository.go:526. No static edge is invented to Local.Save.
