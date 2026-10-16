// Package effects defines version one's API summaries. Keys use resolved Go
// package/type identities, never just method names. A summary is a boundary:
// its implementation is not traversed. Keep callback caveats explicit here.
package effects

import (
	"go/types"
	"sort"
)

type Set uint16

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
	Unknown
)

var names = map[Set]string{
	FSRead: "fs.read", FSWrite: "fs.write", FSCreate: "fs.create", FSRemove: "fs.remove",
	NetConnect: "net.connect", NetListen: "net.listen", ProcessExec: "process.exec",
	EnvRead: "env.read", EnvWrite: "env.write", DBQuery: "db.query", DBExec: "db.exec", Unknown: "unknown.call",
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

func Lookup(fn *types.Func) (Set, bool) {
	s, ok := summaries[Key(fn)]
	return s, ok
}

// These are operation-kind summaries, not resource or permission models.
// OpenFile's flags are not evaluated: all permitted operation kinds are kept.
// Constructors are summarized separately from operations that start work.
var summaries = buildSummaries()

func buildSummaries() map[string]Set {
	m := make(map[string]Set)
	add := func(pkg string, effect Set, funcs ...string) {
		for _, fn := range funcs {
			m[pkg+"."+fn] = effect
		}
	}
	add("os", FSRead, "ReadFile", "ReadDir", "Stat", "Lstat", "Readlink", "Getwd", "Executable", "Open", "(*File).Read", "(*File).ReadAt", "(*File).ReadDir", "(*File).Readdir", "(*File).Readdirnames", "(*File).Stat")
	add("os", FSWrite, "Truncate", "Chmod", "Chown", "Lchown", "Chtimes", "Chdir", "(*File).Write", "(*File).WriteAt", "(*File).WriteString", "(*File).Truncate", "(*File).Chmod", "(*File).Chown", "(*File).Chdir", "(*File).Sync", "(*File).Close")
	add("os", FSCreate|FSWrite, "WriteFile", "Create", "CreateTemp")
	add("os", FSCreate, "Mkdir", "MkdirAll", "MkdirTemp", "Link", "Symlink")
	add("os", FSRead|FSWrite|FSCreate, "OpenFile")
	add("os", FSRemove, "Remove", "RemoveAll")
	add("os", FSRemove|FSCreate, "Rename")
	add("os", FSRead, "OpenRoot", "OpenInRoot", "(*Root).Open", "(*Root).OpenRoot", "(*Root).ReadFile", "(*Root).Stat", "(*Root).Lstat", "(*Root).Readlink", "(*rootFS).Open", "(*rootFS).ReadFile", "(*rootFS).ReadDir", "(*rootFS).Stat", "(*rootFS).Lstat", "(*rootFS).ReadLink")
	add("os", FSWrite, "(*Root).Chmod", "(*Root).Chown", "(*Root).Lchown", "(*Root).Chtimes")
	add("os", FSCreate|FSWrite, "(*Root).Create", "(*Root).WriteFile")
	add("os", FSRead|FSWrite|FSCreate, "(*Root).OpenFile")
	add("os", FSCreate, "(*Root).Mkdir", "(*Root).MkdirAll", "(*Root).Link", "(*Root).Symlink")
	add("os", FSRemove, "(*Root).Remove", "(*Root).RemoveAll")
	add("os", FSRemove|FSCreate, "(*Root).Rename")
	add("os", EnvRead, "Getenv", "LookupEnv", "Environ", "ExpandEnv", "UserHomeDir", "UserCacheDir", "UserConfigDir", "TempDir")
	add("os", EnvWrite, "Setenv", "Unsetenv", "Clearenv")
	add("os", ProcessExec, "StartProcess")
	add("os", Unknown, "Exit", "(*Process).Kill", "(*Process).Signal", "(*Process).Wait", "Pipe")
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
	add("reflect", Unknown, "(Value).Call", "(Value).CallSlice", "(Value).Method", "(Value).MethodByName")
	add("errors", 0, "New")
	return m
}
