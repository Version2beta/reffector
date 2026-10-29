# Caddy candidate finding: ACME CA-ID normalization aliases databases and permits path escape

**Disposition:** Qualified for maintainer review; behavior reproduced in an isolated copy. Configuration-dependent. Severity is not assigned because the required configuration authority and deployment impact vary. No upstream report or code change was made.

## Affected target

Caddy upstream `master` snapshot `32bba1545989e07368c0977a37652f33d80c8cd4`.

Affected code: `modules/caddypki/acmeserver/acmeserver.go:255-289, 375`.

## Behavior and impact

`Handler.getDatabaseKey` lowercases and trims the configured CA ID, then removes characters matched by ``[^\w.-_]``. The range `.-_` admits `/` (and `\`) while excluding `-`. The resulting string is used both as the key to the shared `databasePool` and as a path component under `caddy.AppDataDir()/acme_server`.

Two distinct PKI CA IDs, `a-b` and `ab`, therefore normalize to the same pool key and receive the same database object. The PKI config documents CA IDs as unique identifiers; `CA.Provision` rejects only the empty ID. The ACME handler also names its provisioner from the original CA ID, so the shared database does not represent one uniquely identified CA.

A local regression probe using the pinned Smallstep ACME NoSQL backend reproduced a concrete database-layer consequence: after the adapter's `CreateAccount` call succeeds under `a-b`, a second `CreateAccount` call with the same key under `ab` fails with `key-id to account-id index already exists`. The second database sees the first CA's provisioner-marked account; Smallstep's ACME middleware rejects an account whose stored provisioner does not match the requested provisioner. A non-colliding control CA (`c-d`) accepts the same key in its separate database. This demonstrates database-level account-key collision and supports, but does not prove at HTTP protocol level, an account-registration/availability impact; no cross-CA authentication bypass was demonstrated.

A second local probe used CA ID `../../escaped` with `XDG_DATA_HOME=t.TempDir()`. The real `openDatabase` created `<temporary-XDG>/escaped/db`, outside both `caddy.AppDataDir()` and its `acme_server` child, while remaining inside the temporary test tree. No path outside that tree was touched. The observed filesystem effect is directory and bbolt database creation; arbitrary-file overwrite was not tested or claimed.

Both triggers require operator-supplied configuration. This work did not establish an unauthenticated or remote attacker path, nor test complete Caddy config loading or a live ACME HTTP request.

## Evidence

- Reffector direct-site inventory identified `os.MkdirAll` in `(Handler).openDatabase$1` as `fs.create`, `site:4949f26ce0586e810576f24dbaa9fb67a14599bc38c7e00e860e631c745a9c34`. Source review traced the constructor through `UsagePool.LoadOrNew`; source and tests, not the graph alone, establish the defect.
- Isolated tests passed for database-pool aliasing, account-key registration collision versus a non-colliding control, and path escape within the temporary directory. Test source/output and hashes: `/tmp/reffector-caddy-discovery/novel-discovery/candidates/acme-key-cleaner/tree/modules/caddypki/acmeserver/discovery_probe_test.go`, `discovery_acme_db_probe_test.go`, and `test-acme-db-confirmation2.*`.
- The offline ACME-server package tests passed on the target snapshot. The focused probe command was `GOPROXY=off GOSUMDB=off GOTOOLCHAIN=local GOWORK=off GOFLAGS=-mod=readonly go test ./modules/caddypki/acmeserver -run '^TestDiscovery' -count=1 -v` in the disposable candidate copy.
- Smallstep ACME behavior was checked against the Caddy-pinned `github.com/smallstep/certificates v0.30.2`; relevant dependency sources and SHA-256 manifest are in `/tmp/reffector-caddy-discovery/novel-discovery/candidates/acme-key-cleaner/dependency-evidence/`.
- Bounded public GitHub issue/PR searches found no report specifically describing CA-ID collisions or traversal. PR #3874 introduced the current cleaner while describing it as database-path sanitization; its stated issue concerned database reload/locking. Search evidence is under `/tmp/reffector-caddy-discovery/novel-discovery/candidates/acme-key-cleaner/upstream-search/`. This bounded search cannot prove no prior report exists.

## Suggested repair direction

Keep the pool identity injective over distinct CA IDs, and derive a path-safe database name without deleting distinctions. Validate the final joined path remains beneath the intended ACME database root. Merely moving the hyphen in the regexp could stop slash traversal but would still leave lossy-normalization collisions.
