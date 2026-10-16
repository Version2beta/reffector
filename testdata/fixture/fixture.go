package fixture

import (
	"bytes"
	"database/sql"
	"fmt"
	"go/types"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"reflect"

	"example.com/effects/dep"
	"example.com/effects/opaque"
)

// If application initialization were executed, every analysis would fail.
func init() { panic("reffector must not execute analyzed code") }

func External() { opaque.Native() }
func Remove()   { _ = os.Remove("not-executed") }

func Pure(n int) int { x := n + 1; return x * 2 }
func Read()          { _, _ = os.ReadFile("not-executed") }
func Write()         { _ = os.WriteFile("not-executed", nil, 0600) }
func Multiple()      { Read(); Write(); Read(); Write() }
func Middle()        { Multiple() }
func Top()           { Middle() }
func Cross()         { dep.Load() }

type Value struct{}

func (Value) Load() { Read() }

type Pointer struct{}

func (*Pointer) Save() { Write() }

type Other struct{}

func (Other) Load() { _ = os.Setenv("NOT_EXECUTED", "1") }

type private struct{}

func (private) unused() { Read() }

// Same spelling as known operations, but these are pure application methods.
type Fake struct{}

func (Fake) Read()    {}
func (Fake) Write()   {}
func (Fake) Query()   {}
func (Fake) Execute() {}
func NotSinks()       { f := Fake{}; f.Read(); f.Write(); f.Query(); f.Execute() }

type Loader interface{ Load() }

func KnownInterface()        { var v Loader = Value{}; v.Load() }
func OpenInterface(v Loader) { v.Load() }

type Missing interface{ Absent() }

func MissingInterface(v Missing) { v.Absent() }

func Recursive(n int) {
	if n > 0 {
		Recursive(n - 1)
	}
	Read()
}
func MutualA(n int) {
	if n > 0 {
		MutualB(n - 1)
	}
}
func MutualB(n int) {
	if n > 0 {
		MutualA(n - 1)
	}
	Write()
}
func Unknown(fn func())        { fn() }
func ForwardUnknown(fn func()) { Unknown(fn) }
func KnownValue()              { fn := Read; fn() }
func PhiValue(b bool) {
	fn := Read
	if b {
		fn = Write
	}
	fn()
}
func BoundMethod()   { fn := (&Pointer{}).Save; fn() }
func BoundSink()     { var f *os.File; fn := f.Read; _, _ = fn(nil) }
func Closure()       { fn := func() { Read() }; fn() }
func ReturnedValue() { fn := factory(); fn() }

//go:noinline
func factory() func()      { return Read }
func Reflect(fn any)       { reflect.ValueOf(fn).Call(nil) }
func Spawn()               { go Read(); defer Write() }
func Generic[T any](v T) T { _ = os.Getenv("NOT_EXECUTED"); return v }

type Box[T any] struct{ V T }

func (Box[T]) Load()               { Read() }
func GenericUse()                  { _ = Generic(1); Box[int]{}.Load() }
func HTTP()                        { _, _ = http.Get("http://not-executed.invalid") }
func Connect()                     { _, _ = net.Dial("tcp", "not-executed.invalid:80") }
func Listen()                      { _, _ = net.Listen("tcp", ":0") }
func Exec()                        { _ = exec.Command("not-executed").Run() }
func Environment()                 { _, _ = os.LookupEnv("NOT_EXECUTED"); _ = os.Unsetenv("NOT_EXECUTED") }
func Database(db *sql.DB)          { _, _ = db.Query("not executed"); _, _ = db.Exec("not executed") }
func FileMethods(f *os.File)       { _, _ = f.Read(nil); _, _ = f.Write(nil) }
func OpenFile()                    { _, _ = os.OpenFile("not-executed", os.O_RDONLY, 0) }
func OpenFileUnknown(flags int)    { _, _ = os.OpenFile("not-executed", flags, 0) }
func OpenFileWrite()               { _, _ = os.OpenFile("not-executed", os.O_WRONLY|os.O_CREATE, 0600) }
func Exit()                        { os.Exit(1) }
func Root(r *os.Root)              { _, _ = r.ReadFile("never"); _ = r.WriteFile("never", nil, 0600) }
func Format() string               { return fmt.Sprintf("memory") }
func TypeName(t types.Type) string { return types.TypeString(t, nil) }
func WriteMemory()                 { var b bytes.Buffer; var w io.Writer = &b; _, _ = w.Write(nil) }

type fileWriter struct{ f *os.File }

func (w fileWriter) Write(p []byte) (int, error) { return w.f.Write(p) }
func WriteFile(w fileWriter)                     { var out io.Writer = w; _, _ = out.Write(nil) }
func WriteOpen(w io.Writer)                      { _, _ = w.Write(nil) }
