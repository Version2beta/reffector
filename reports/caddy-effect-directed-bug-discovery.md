# Caddy effect-directed defect discovery

**Status:** One configuration-dependent defect candidate reproduced at the database layer and suitable for maintainer review. No production source was changed, and no upstream report was filed.

## Executive finding

In Caddy's embedded ACME server, the CA-ID cleaner uses the regexp ``[^\w.-_]``. The `.-_` sequence is a character range, not a list of three literal characters: it admits `/` (and `\`) while excluding `-`. Because the cleaned string is used both as the global database-pool key and as a filesystem path component, distinct configured CA IDs can alias the same ACME database, and traversal components can escape Caddy's ACME/app-data directories.

Both behaviors were reproduced using the current Caddy source in a disposable copy. In particular, IDs `a-b` and `ab` return the same database pointer; in a local test, the ACME database adapter's `CreateAccount` call for the second CA with the same account key fails with `key-id to account-id index already exists`. A traversal ID created a bbolt database at a sibling of the temporary Caddy app-data directory, never outside the test's temporary XDG data home.

The trigger is operator-supplied Caddy configuration. This investigation did **not** establish an unauthenticated or remote attacker path, arbitrary-file overwrite, or privilege escalation. The strongest demonstrated consequence is an account-key conflict at the ACME database layer and unintended database sharing for CA IDs that normalize to the same key. An HTTP-level registration failure is plausible but was not directly exercised; path escape is a separate consequence of the same cleaner.

## Target, tool, and survey scope

- Caddy snapshot: upstream `master` revision `32bba1545989e07368c0977a37652f33d80c8cd4`. The disposable source snapshot has no Git metadata; revision selection and source provenance are recorded under `/tmp/reffector-caddy-discovery/source-identities/` and `novel-discovery/`. The original `/Users/rob/Development/caddy` checkout was not used for tests or modified.
- Reffector: built from `feat/effect-directed-discovery` at `e0e62a296170881822b08bafd50c1f0b81104ce7`; binary SHA-256 `822b818277e4cf37a134de5b0b0c058c5e70ad612d888a741f096c7b3063c967`.
- Offline dependency preflight passed. Caddy's `go.mod` pins `github.com/smallstep/certificates v0.30.2`.
- The blind direct inventory selected 49 packages, 401 Go files, and 173 test files. Reffector reported 2,636 source functions, 2,623 operation sites (837 selected and 1,786 dependency sites), and 11,802 unresolved sites. The full reverse graph was truncated; those counts and edges are leads, not proof of runtime execution.
- Targeted source review covered core/configuration, command, reverse proxy, fileserver/encoding, TLS/PKI, authentication, request-body, rewrite, timeouts, tracing, intercept, logging, networking, events, metrics, and ACME storage. Many other modules and unresolved sites were not inspected deeply.
- The two blind survey stages reported about 8 minutes and 5 minutes 43 seconds of active time, respectively (about 14 minutes total). Their frozen outputs are `/tmp/reffector-caddy-discovery/novel-discovery/caddy-discovery-phase1-frozen.md` (SHA-256 `91017716646b05c7f14600c6fc460898a180eab89fa55ea09427bc9f3b9d59f7`) and `caddy-discovery-phase2-frozen.md` (SHA-256 `2755b94482ada717fceef8620155c18eda25925bb3e7ccfff10a666a40fd129f`). Phase evidence and manifests are beside them. Discovery inventory was frozen before public history/novelty searches.

## Candidate: lossy ACME CA-ID cleaning aliases databases and permits path escape

### Contract and source path

`modules/caddypki/pki.go:38-41` defines the configured CA map as IDs that uniquely identify CAs. `PKI.Provision` passes each map key to `CA.Provision`; `modules/caddypki/ca.go:104-105` rejects only an empty ID. The ACME handler selects a CA by that ID and names its ACME provisioner with the same ID (`modules/caddypki/acmeserver/acmeserver.go:175`).

`Handler.getDatabaseKey` lowercases/trims the ID, then removes regexp matches (`acmeserver.go:255-260`). The package-level cleaner is ``regexp.MustCompile(`[^\w.-_]`)`` (`acmeserver.go:375`). `Handler.openDatabase` passes this cleaned value to `databasePool.LoadOrNew` and joins it below `filepath.Join(caddy.AppDataDir(), "acme_server", key)` before `os.MkdirAll` and bbolt creation (`acmeserver.go:279-289`). The same lossy value therefore controls both resource identity and path placement.

The Reffector inventory identified `os.MkdirAll` in `(Handler).openDatabase$1` as `fs.create`, site `site:4949f26ce0586e810576f24dbaa9fb67a14599bc38c7e00e860e631c745a9c34`, without a site-level uncertainty marker. Source inspection established that this operation is in the constructor passed to `UsagePool.LoadOrNew`; `usagepool.go` calls that constructor when the key is absent. Reffector exposed the effect site; the regex, contract violation, and consequences were established by source review and local tests, not inferred from the graph alone.

### Reproduced behavior

All probes ran in `/tmp/reffector-caddy-discovery/novel-discovery/candidates/acme-key-cleaner/tree`, a disposable copy whose 742 original files match the prepared Caddy snapshot manifest. Only two temporary test files were added to that copy. The original Caddy checkout and prepared source snapshot remained untouched.

1. **Distinct CAs share one database.** `TestDiscoveryDistinctCAIDsAliasDatabasePoolEntry` calls the real `openDatabase` with `CA: "a-b"` and `CA: "ab"`; both keys become `ab`, both handlers receive the same database pointer, and the usage-pool reference count is two.
2. **The alias has a concrete ACME database consequence.** `TestDiscoveryAliasedCAsShareACMEAccountKeyIndex` uses the pinned Smallstep ACME NoSQL implementation and a local generated account key. The database adapter's `CreateAccount` under `a-b` succeeds; the second database wrapper sees that account marked for provisioner `a-b`, and `CreateAccount` under `ab` fails with `key-id to account-id index already exists`. A non-colliding control ID (`c-d`, key `cd`) accepts the same key in its separate database. In the pinned dependency, ACME account lookup indexes by key ID, while request middleware rejects accounts whose stored provisioner name differs from the requested one (`acme/api/middleware.go` in Smallstep v0.30.2). This demonstrates a database-layer key collision and supports, but does not prove at HTTP protocol level, an account-registration/availability impact; no cross-CA authentication bypass was demonstrated.
3. **Traversal escapes the intended storage roots.** `TestDiscoveryTraversalCAIDCreatesDatabaseOutsideACMEServerDir` invokes the real `openDatabase` with `CA: "../../escaped"` and `XDG_DATA_HOME=t.TempDir()`. It creates `<temporary-XDG>/escaped/db`, outside both `<temporary-XDG>/caddy/acme_server` and Caddy's `<temporary-XDG>/caddy` app-data directory, while remaining inside the test's temporary tree. No path outside that tree was touched.

The exact regexp/path probe is `phase2-evidence/probe-keycleaner.go` and `.out`. The final focused test command was:

```text
GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOWORK=off GOFLAGS=-mod=readonly \
  go test ./modules/caddypki/acmeserver -run '^TestDiscovery' -count=1 -v
```

It passed all three temporary tests on Go 1.27.2, darwin/arm64 (1.682 s); output and hashes are in `candidates/acme-key-cleaner/test-acme-db-confirmation2.*`. A separate offline run of the unmodified ACME-server package tests passed in phase 2 (`go test ./modules/caddypki/acmeserver`, 5.776 s). One earlier version of the temporary account probe did not compile because it passed a pointer-to-interface where the dependency expects `nosql.DB`; the harness was corrected to use the same type assertion as Caddy before the passing run.

### Novelty and limitations

Bounded, unauthenticated GitHub API searches in `candidates/acme-key-cleaner/upstream-search/` covered ACME-server/database, account-key, CA-ID, path, and sanitizer terms, plus the ACME-server file history and recent release notes. They surfaced no issue or PR specifically describing CA-ID collisions or traversal. The file history shows that PR [#3874](https://github.com/caddyserver/caddy/pull/3874) introduced this cleaner in 2020 as part of a change described as sanitizing the ACME database path; its patch contains the current regexp. The related issue/PR concerned database reload/locking, not the reproduced behaviors. The recent v2.11.6 notes likewise discuss a database-lock message but not key normalization. This is a bounded search, not proof that no report exists.

Impact requires configuration containing IDs that normalize to the same key or a traversal-like ID. The tests exercised the real database-opening code and ACME DB adapter, but did not load a complete Caddy configuration or send an ACME HTTP request. The generated database file was confined to a temporary test directory and removed by test cleanup. The filesystem effect demonstrated is creation of directories and a bbolt `db` file; arbitrary-file overwrite was not tested or claimed.

**Disposition:** Qualified candidate for maintainer review. A safe repair should preserve distinct CA identity in the pool key and use a path-safe representation with explicit containment checks; merely changing the character class while continuing to delete characters may leave ID collisions. No repair was made.

## Other survey results

- Phase 1 retained a low-confidence observation in `healthchecks.go:453-459`: a templated active-health-check header name that expands to `Host` is read back from the unexpanded map key. The directive documents field/value syntax but does not explicitly promise dynamic field names; literal `Host` works. No runtime test was performed, so this remains an unverified lead and is not handed off.
- Active-health response-size/drain behavior, authentication response isolation, request-body limits, timeout handling, HTTP shutdown, TLS/PKI policy, logging permissions, and several other effect groups were reviewed without an independent defect established. Detailed coverage limits are in the frozen phase reports.
- No full reverse graph, full Caddy integration suite, live service, or broad configuration matrix was run. No external dependency downloads occurred.

## Preserved evidence

Primary evidence is under `/tmp/reffector-caddy-discovery/novel-discovery/`: frozen phase reports and source-integrity checks; `phase1-evidence/` and `phase2-evidence/`; candidate test source/results under `candidates/acme-key-cleaner/`; pinned Smallstep source excerpts and hashes under `dependency-evidence/`; and public upstream search responses/summaries under `upstream-search/`.
