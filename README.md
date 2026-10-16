# reffector

`reffector` statically reports recognized side effects reachable from Go
functions. It analyzes code only: it does not execute functions, initializers,
or tests, and does not modify analyzed source.

```sh
go install ./cmd/reffector
reffector ./...
reffector -direct ./internal/...
reffector -tests -json example.com/project/billing
man "$PWD/man/man1/reffector.1"
```

It requires Go 1.26+ and the Go toolchain on `PATH`. Package patterns use
`golang.org/x/tools/go/packages`; no arguments means `.`. Dependencies are
inspected to propagate effects, but output normally contains only selected
packages. `-tests` includes internal and external test-file functions.

## Output

Default output is sorted `package.Function effect` pairs. Methods include their
receiver, for example `example.com/app.(*Store).Save db.exec`. Only recognized
effects appear in text output.

`-json` emits one function summary per record:

```json
{
  "package": "example.com/app",
  "function": "Load",
  "effects": [{"identifier":"fs.read","direct":true}],
  "uncertain": false
}
```

`effects` is an array (possibly empty). `direct` says that the function itself
calls a summarized operation; otherwise the effect was inherited. `uncertain`
means an unresolved or opaque operation is reachable. It is deliberately
separate from effects: uncertainty is not an operation and is never printed in
text output. A function with no recognized effects and `uncertain: true` is not
shown in text but remains visible in JSON. Neither an empty effect list nor
`uncertain: false` proves a function is pure.

## Recognized effects

| Identifier | Meaning |
|---|---|
| `fs.read` | Read filesystem contents or metadata |
| `fs.write` | Modify filesystem contents or metadata |
| `fs.create` | Create filesystem entries |
| `fs.remove` | Remove filesystem entries |
| `net.connect` | Initiate a network connection |
| `net.listen` | Listen for or accept connections |
| `process.exec` | Start an external process |
| `process.exit` | Terminate the process with `os.Exit` |
| `env.read`, `env.write` | Read or modify process environment values |
| `db.query`, `db.exec` | Database query or command operations |

The centralized table in `internal/effects/effects.go` recognizes common `os`
(including `File` and `Root`), `path/filepath`, `io/ioutil`, `net`, `crypto/tls`,
`net/http`, `os/exec`, and `database/sql` APIs. It recognizes selected in-memory operations such as basic-argument
`fmt.Sprintf`, nil-qualifier `go/types.TypeString`, and `bytes.Buffer` methods
as effect-free summary boundaries, preventing their implementation plumbing
from being interpreted as I/O. Formatting or type-string callbacks that could
execute application code are marked uncertain instead. `os.File.Write` remains `fs.write`, but
that is a resource-kind approximation: a file descriptor may be a pipe, device,
or standard output. `File.Close` has no filesystem-write classification.

`os.OpenFile` uses a constant flag argument when SSA provides one: read-only
reports `fs.read`; write/create flags report their applicable effects. Unknown
flags conservatively report read, write, and create.

## Analysis model and limits

Direct effects come from a recognized call. Transitive effects are propagated
through ordinary calls, methods, closures, known local function values, phi
merges, generics, `go`, and `defer`; recursion uses a stable worklist.

Interface dispatch is intentionally evidence-based. A receiver made from a
known concrete SSA value is followed to that method. An interface parameter,
memory-loaded interface, or other open receiver is **not** expanded to every
loaded implementation (for example every `io.Writer`); it is marked uncertain
instead. This avoids inventing filesystem writes from unrelated `*os.File`
implementations while retaining the fact that analysis is incomplete.

Known standard-library summaries stop traversal. An unrecognized standard
library call, reflection invocation, cgo, assembly, missing body, or unsupported
function-value flow is uncertain rather than guessed. Branch feasibility,
resource destinations, permissions, plugins, all build tags/platforms, shared
memory mutation, and secondary driver/callback effects are not modeled. Results
are possible static reachability, not proof of execution or purity.

Module loading uses `-mod=readonly`. Workspace metadata is copied to a temporary
workspace so analyzed `go.mod`, `go.sum`, and `go.work.sum` are not repaired or
modified. The Go toolchain may still update its own caches.

## Verification

```sh
go build ./...
go test ./...
go vet ./...
```

The executable integration suite analyzes fixtures without running them. It
covers propagation, recursive calls, package patterns, tests, closures,
generics, known/open interfaces, memory and file writers, formatting/type
strings, OpenFile flags, JSON uncertainty, and source-tree preservation.
