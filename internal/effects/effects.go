// Package effects defines version one's API summaries. Keys use resolved Go
// package/type identities, never just method names. A summary is a boundary:
// its implementation is not traversed. Keep callback caveats explicit here.
package effects

import (
	"go/constant"
	"go/types"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"
)

type Set uint32

const (
	FSRead Set = 1 << iota
	FSWrite
	FSCreate
	FSRemove
	NetConnect
	NetListen
	ProcessExec
	EnvRead
	EnvWrite
	DBQuery
	DBExec
	ProcessExit
	HTTPResponse
	ResourceAcquire
	ResourceRelease
	NetClose
	FSRename
	// Unknown is an internal uncertainty marker, never an output effect.
	Unknown
)

var names = map[Set]string{
	FSRead: "fs.read", FSWrite: "fs.write", FSCreate: "fs.create", FSRemove: "fs.remove",
	NetConnect: "net.connect", NetListen: "net.listen", ProcessExec: "process.exec",
	EnvRead: "env.read", EnvWrite: "env.write", DBQuery: "db.query", DBExec: "db.exec", ProcessExit: "process.exit",
	HTTPResponse: "http.response", ResourceAcquire: "resource.acquire", ResourceRelease: "resource.release", NetClose: "net.close", FSRename: "fs.rename",
}

func (s Set) Names() []string {
	var result []string
	for bit, name := range names {
		if s&bit != 0 {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

// Key returns a canonical identity, e.g. os.ReadFile or os.(*File).Read.
func Key(fn *types.Func) string {
	if fn == nil || fn.Pkg() == nil {
		return ""
	}
	key := fn.Pkg().Path() + "."
	if recv := fn.Signature().Recv(); recv != nil {
		key += "(" + types.TypeString(recv.Type(), func(*types.Package) string { return "" }) + ")."
	}
	return key + fn.Name()
}

// LookupInterface summarizes the named interface contract, not a concrete
// implementation. It never supplies a call target. Unknown io.Writer stays
// unclassified: its resource kind and destination are not established.
func LookupInterface(fn *types.Func) (Set, bool) {
	switch Key(fn) {
	case "net/http.(ResponseWriter).Write", "net/http.(ResponseWriter).WriteHeader":
		return HTTPResponse, true
	case "net.(Conn).Close", "net.(PacketConn).Close", "net.(Listener).Close":
		return NetClose | ResourceRelease, true
	case "io.(Closer).Close":
		// Embedded standard contracts (ReadCloser, ReadWriteCloser, etc.)
		// retain the declaring Closer method identity. Resource kind is unknown.
		return ResourceRelease, true
	}
	return 0, false
}

func Lookup(fn *types.Func) (Set, bool) {
	s, ok := summaries[Key(fn)]
	return s, ok
}

// LookupCall recognizes summaries whose effect depends on a constant argument.
func LookupCall(fn *types.Func, args []ssa.Value) (Set, bool) {
	if Key(fn) == "go/types.TypeString" {
		if len(args) >= 2 {
			if c, ok := args[1].(*ssa.Const); ok && c.IsNil() {
				return 0, true
			}
		}
		return Unknown, true
	}
	if Key(fn) == "fmt.Sprintf" || Key(fn) == "fmt.Sprint" || Key(fn) == "fmt.Sprintln" || Key(fn) == "fmt.Errorf" {
		for _, arg := range args[1:] {
			if !formatValueIsBasic(arg) {
				return Unknown, true // Stringer/Error/Formatter may execute application code.
			}
		}
		return 0, true
	}
	if Key(fn) == "os.OpenFile" {
		if len(args) < 2 {
			return FSRead | FSWrite | FSCreate | ResourceAcquire, true
		}
		if c, ok := args[1].(*ssa.Const); ok && c.Value != nil {
			if flags, ok := constant.Int64Val(c.Value); ok {
				set := ResourceAcquire
				if flags&int64(os.O_WRONLY|os.O_RDWR) != 0 {
					set |= FSWrite
				} else {
					set |= FSRead
				}
				if flags&int64(os.O_CREATE) != 0 {
					set |= FSCreate
				}
				if flags&int64(os.O_TRUNC) != 0 {
					set |= FSWrite
				}
				return set, true
			}
		}
		return FSRead | FSWrite | FSCreate | ResourceAcquire, true
	}
	return Lookup(fn)
}

func formatValueIsBasic(v ssa.Value) bool {
	if _, ok := v.(*ssa.Const); ok {
		return true
	}
	if v, ok := v.(*ssa.MakeInterface); ok {
		_, ok := v.X.Type().Underlying().(*types.Basic)
		return ok
	}
	if slice, ok := v.(*ssa.Slice); ok {
		return formatSliceIsBasic(slice)
	}
	return false
}

// Go lowers a variadic call such as Sprintf("%d", n) to an allocated [N]any,
// stores, and a slice. Inspect only that local SSA shape; anything else is
// uncertain rather than attempting general memory dataflow.
func formatSliceIsBasic(slice *ssa.Slice) bool {
	alloc, ok := slice.X.(*ssa.Alloc)
	if !ok || alloc.Referrers() == nil {
		return false
	}
	count := 0
	for _, use := range *alloc.Referrers() {
		if use == slice {
			continue
		}
		index, ok := use.(*ssa.IndexAddr)
		if !ok || index.Referrers() == nil {
			return false
		}
		for _, indexedUse := range *index.Referrers() {
			store, ok := indexedUse.(*ssa.Store)
			if !ok || store.Addr != index || !formatValueIsBasic(store.Val) {
				return false
			}
			count++
		}
	}
	return count > 0
}

// These are operation-kind summaries, not resource or permission models.
// OpenFile flags are evaluated when SSA provides a constant; unknown flags keep
// all possible operation kinds. Constructors are separate from operations that start work.
var summaries = buildSummaries()

func buildSummaries() map[string]Set {
	m := make(map[string]Set)
	add := func(pkg string, effect Set, funcs ...string) {
		for _, fn := range funcs {
			m[pkg+"."+fn] = effect
		}
	}
	add("os", FSRead, "ReadFile", "ReadDir", "Stat", "Lstat", "Readlink", "Getwd", "Executable", "Open", "(*File).Read", "(*File).ReadAt", "(*File).ReadDir", "(*File).Readdir", "(*File).Readdirnames", "(*File).Stat")
	add("os", FSWrite, "Truncate", "Chmod", "Chown", "Lchown", "Chtimes", "Chdir", "(*File).Write", "(*File).WriteAt", "(*File).WriteString", "(*File).Truncate", "(*File).Chmod", "(*File).Chown", "(*File).Chdir", "(*File).Sync")
	add("os", FSCreate|FSWrite, "WriteFile", "Create", "CreateTemp")
	add("os", FSCreate, "Mkdir", "MkdirAll", "MkdirTemp", "Link", "Symlink")
	// OpenFile is handled by LookupCall.
	add("os", FSRemove, "Remove", "RemoveAll")
	// Preserve the existing entry-change categories while naming the operation.
	add("os", FSRemove|FSCreate|FSRename, "Rename")
	add("os", FSRead, "OpenRoot", "OpenInRoot", "(*Root).Open", "(*Root).OpenRoot", "(*Root).ReadFile", "(*Root).Stat", "(*Root).Lstat", "(*Root).Readlink", "(*rootFS).Open", "(*rootFS).ReadFile", "(*rootFS).ReadDir", "(*rootFS).Stat", "(*rootFS).Lstat", "(*rootFS).ReadLink")
	add("os", FSWrite, "(*Root).Chmod", "(*Root).Chown", "(*Root).Lchown", "(*Root).Chtimes")
	add("os", FSCreate|FSWrite, "(*Root).Create", "(*Root).WriteFile")
	add("os", FSRead|FSWrite|FSCreate, "(*Root).OpenFile")
	add("os", FSCreate, "(*Root).Mkdir", "(*Root).MkdirAll", "(*Root).Link", "(*Root).Symlink")
	add("os", FSRemove, "(*Root).Remove", "(*Root).RemoveAll")
	add("os", FSRemove|FSCreate|FSRename, "(*Root).Rename")
	add("os", EnvRead, "Getenv", "LookupEnv", "Environ", "ExpandEnv", "UserHomeDir", "UserCacheDir", "UserConfigDir", "TempDir")
	add("os", EnvWrite, "Setenv", "Unsetenv", "Clearenv")
	add("os", ProcessExec, "StartProcess")
	add("os", ProcessExit, "Exit")
	add("log", ProcessExit, "Fatal", "Fatalln", "Fatalf", "(*Logger).Fatal", "(*Logger).Fatalln", "(*Logger).Fatalf")
	add("os", ResourceRelease, "(*File).Close", "(*Root).Close")
	add("os", 0, "(*Process).Kill", "(*Process).Signal", "(*Process).Wait", "Pipe")
	add("path/filepath", FSRead, "Abs", "EvalSymlinks", "Glob")
	add("io/ioutil", FSRead, "ReadFile", "ReadDir")
	add("io/ioutil", FSCreate|FSWrite, "WriteFile", "TempFile")
	add("io/ioutil", FSCreate, "TempDir")
	add("os/exec", 0, "(*Cmd).String")
	add("os/exec", FSRead|EnvRead, "Command", "LookPath")
	add("os/exec", FSRead|EnvRead|Unknown, "CommandContext")
	add("os/exec", ProcessExec, "(*Cmd).Start", "(*Cmd).Run", "(*Cmd).Output", "(*Cmd).CombinedOutput")
	add("os/exec", Unknown, "(*Cmd).Wait")
	add("net", NetConnect, "Dial", "DialTimeout", "(*Dialer).Dial", "(*Dialer).DialContext", "DialTCP", "DialUDP", "DialIP", "DialUnix")
	add("net", NetListen, "Listen", "ListenPacket", "ListenTCP", "ListenUDP", "ListenIP", "ListenUnix", "ListenUnixgram", "(*ListenConfig).Listen", "(*ListenConfig).ListenPacket", "(*TCPListener).Accept", "(*TCPListener).AcceptTCP", "(*UnixListener).Accept", "(*UnixListener).AcceptUnix")
	add("crypto/tls", NetConnect, "Dial", "DialWithDialer", "(*Dialer).Dial", "(*Dialer).DialContext")
	add("crypto/tls", NetListen, "Listen", "(*listener).Accept")
	// HTTP can invoke application hooks/transports/handlers inside this boundary.
	add("net/http", NetConnect|Unknown, "Get", "Head", "Post", "PostForm", "(*Client).Do", "(*Client).Get", "(*Client).Head", "(*Client).Post", "(*Client).PostForm", "(*Transport).RoundTrip")
	add("net/http", NetListen|Unknown, "ListenAndServe", "ListenAndServeTLS", "Serve", "ServeTLS", "(*Server).ListenAndServe", "(*Server).ListenAndServeTLS", "(*Server).Serve", "(*Server).ServeTLS")
	add("net/http", FSRead|Unknown, "ServeFile", "ServeFileFS")
	add("net/http", Unknown, "ServeContent")
	add("database/sql", Unknown, "Open")
	add("database/sql", 0, "OpenDB", "(*DB).Driver", "Named", "(*Row).Err")
	for _, recv := range []string{"(*DB)", "(*Tx)", "(*Conn)", "(*Stmt)"} {
		for _, method := range []string{"Query", "QueryContext", "QueryRow", "QueryRowContext"} {
			add("database/sql", DBQuery, recv+"."+method)
		}
		for _, method := range []string{"Exec", "ExecContext"} {
			add("database/sql", DBExec, recv+"."+method)
		}
	}
	add("database/sql", DBExec, "(*DB).Begin", "(*DB).BeginTx", "(*Conn).BeginTx", "(*Tx).Commit", "(*Tx).Rollback", "(*DB).Prepare", "(*DB).PrepareContext", "(*Tx).Prepare", "(*Tx).PrepareContext", "(*Conn).PrepareContext")
	add("database/sql", Unknown, "(*DB).Ping", "(*DB).PingContext", "(*DB).Conn")
	add("reflect", 0, "(Value).Method", "(Value).MethodByName")
	add("reflect", Unknown, "(Value).Call", "(Value).CallSlice")
	// In-memory standard-library operations are summary boundaries, preventing
	// their internal io.Writer plumbing from being mistaken for file writes.
	add("fmt", 0, "Appendf", "Append", "Appendln")
	add("go/types", 0, "ObjectString", "SelectionString", "Id", "NewContext")
	add("bytes", 0, "(*Buffer).Write", "(*Buffer).WriteString", "(*Buffer).WriteByte", "(*Buffer).WriteRune", "(*Buffer).Read")
	add("errors", 0, "New")
	// Resource ownership is separate from filesystem mutation. These operations
	// return owned handles when successful; this is not leak/lifetime analysis.
	for _, key := range []string{
		"os.Open", "os.Create", "os.CreateTemp", "os.OpenRoot", "os.OpenInRoot", "os.Pipe",
		"os.(*Root).Open", "os.(*Root).OpenRoot", "os.(*Root).Create", "os.(*Root).OpenFile",
		"io/ioutil.TempFile", "database/sql.Open", "database/sql.OpenDB", "database/sql.(*DB).Conn",
	} {
		m[key] |= ResourceAcquire
	}
	for key := range m {
		if (strings.HasPrefix(key, "net.") || strings.HasPrefix(key, "crypto/tls.")) && m[key]&(NetConnect|NetListen) != 0 {
			m[key] |= ResourceAcquire
		}
	}
	add("database/sql", ResourceRelease, "(*DB).Close", "(*Conn).Close", "(*Stmt).Close", "(*Rows).Close")
	for _, recv := range []string{"(*conn)", "(*TCPConn)", "(*UDPConn)", "(*IPConn)", "(*UnixConn)", "(*TCPListener)", "(*UnixListener)"} {
		add("net", NetClose|ResourceRelease, recv+".Close")
	}
	// Half-close changes connection state but does not release the whole handle.
	for _, recv := range []string{"(*TCPConn)", "(*UnixConn)"} {
		add("net", NetClose, recv+".CloseRead", recv+".CloseWrite")
	}
	add("crypto/tls", NetClose|ResourceRelease|Unknown, "(*Conn).Close")
	add("crypto/tls", NetClose|Unknown, "(*Conn).CloseWrite")
	add("net/http", NetClose|ResourceRelease|Unknown, "(*Server).Close", "(*Server).Shutdown")
	add("net/http", NetClose|Unknown, "(*Transport).CloseIdleConnections", "(*Client).CloseIdleConnections")
	return m
}
