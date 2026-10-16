# reffector

`reffector` lists side effects statically reachable from Go functions. It analyzes
code; it does not refactor it or run its functions, initializers, or tests.
An absent result is **not proof of purity**.

## Install and run

Requires Go 1.26 or newer and the Go toolchain on `PATH`. From this checkout:

```sh
go install ./cmd/reffector
reffector .
reffector ./...
reffector -direct ./internal/...
reffector -tests -json example.com/project/billing
```

The executable is installed in `GOBIN`, or normally `$(go env GOPATH)/bin`.
No package arguments means `.`. Flags precede packages; `-h` and `-help` show
usage. Patterns are resolved by `golang.org/x/tools/go/packages`, using the
current build environment. Dependencies are inspected, but only requested
packages are displayed. `-tests` adds internal and external test functions;
generated test executables are not displayed.

Text output is `full/import/path.Function effect`, one unique pair per line,
sorted by package, function, and effect. Methods use `(Type).Method` or
`(*Type).Method`; closures append `$1`, `$2`, etc. Source `init` functions have
numbered names; synthetic `init` represents variable and imported-package
initialization. JSON is an array, including `[]` for no results:

```json
[{"package":"example.com/app","function":"Load","effect":"fs.read","direct":true}]
```

`direct: true` means this function calls a summarized API or has an unresolved
call itself. Otherwise the effect is inherited. If both apply, one record is
emitted with `direct: true`. Diagnostics go to stderr. Exit statuses: 0 success
(including effects found and help), 1 loading/analysis/output failure, 2 bad flags.

## Effect vocabulary

| Identifier | Operation |
|---|---|
| `fs.read` | Read file contents or metadata |
| `fs.write` | Modify contents, metadata, or filesystem state |
| `fs.create` | Create filesystem entries |
| `fs.remove` | Remove filesystem entries |
| `net.connect` | Initiate network connections |
| `net.listen` | Listen for or accept connections |
| `process.exec` | Start external processes |
| `env.read` / `env.write` | Read / modify process environment |
| `db.query` / `db.exec` | Database query / command operations |
| `unknown.call` | Unresolved or opaque calls, or interactions outside this vocabulary |

The centralized, package-and-receiver-qualified table in
[`internal/effects/effects.go`](internal/effects/effects.go) covers common `os`
(including `File` and `Root`), `path/filepath`, `io/ioutil`, `net`, `crypto/tls`,
`net/http`, `os/exec`, and `database/sql` operations. For example, `WriteFile`
reports both creation and writing. `OpenFile` conservatively reports reading,
writing, and creation without inspecting flags. `exec.Command` can search PATH
(`env.read`, `fs.read`); starting the command adds `process.exec`.

## Analysis and limits

SSA supplies typed calls, including `go` and `defer`. A demand-driven call graph
follows static calls, methods, closures, local function values and phi merges,
and instantiated generics across packages. Known interface values resolve to
their concrete methods; open interface calls consider compatible concrete types
in loaded packages **and** report `unknown.call` for outside implementations.
This class-hierarchy approach works for libraries and unused functions without a
`main`, unlike main-rooted reachability, but can overestimate possible receivers.
A monotone worklist propagates effects through recursive cycles.

API summaries stop traversal at known operations. Ordinary helpers inherit
effects: if `Process` calls `Load`, and `Load` calls `os.ReadFile`, both report
`fs.read`; `-direct` reports only `Load`.

This is possible reachability, not an execution trace or a complete effect
system. Branch feasibility, argument values, call context, resource identities,
shared-memory mutation, and implicit operating-system behavior are not modeled.
Function values flowing through memory, parameters, or return values are unknown;
known local values work. Reflection calls, assembly, cgo, and missing bodies are
opaque. External code is not inspected. Summary boundaries describe primary
operation kinds, not all secondary behavior of drivers, hooks, or custom
implementations; HTTP summaries additionally mark unknown callbacks. These
boundaries and unsupported value flows can miss specific effect kinds even when
`unknown.call` is present. Open-interface dispatch can create many false positives.
Only the current platform/build-selected files are analyzed, not all build tags.

Loading uses `-mod=readonly` (rather than vendor mode), so missing module metadata
is an error, not repaired. Workspaces are copied to temporary metadata files to
avoid writes to `go.work.sum`; source and module files are left unchanged. The
trusted Go toolchain may download dependencies/toolchains and update its caches.
This is not a sandbox for untrusted toolchains or package drivers.

## Checks and manual

```sh
go build ./...
go test ./...
go vet ./...
man -l man/man1/reffector.1
# macOS/BSD alternative:
man "$PWD/man/man1/reffector.1"
```

Executable integration tests analyze small fixture packages without running
them, including a panicking initializer that must never execute. They check
propagation, dispatch, unresolved calls, flags, patterns, ordering, and unchanged
analyzed files, including missing checksums in modules and workspaces.

Actual self-analysis excerpt (`reffector -direct ./...`, filtered to `loadConfig`):

```text
reffector/internal/analyzer.loadConfig env.read
reffector/internal/analyzer.loadConfig fs.create
reffector/internal/analyzer.loadConfig fs.read
reffector/internal/analyzer.loadConfig fs.write
reffector/internal/analyzer.loadConfig process.exec
```
