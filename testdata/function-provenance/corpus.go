package provenance

import (
	"example.com/provenance/dep"
	"os"
)

func read()                         { _, _ = os.ReadFile("input") }
func write()                        { _ = os.WriteFile("output", nil, 0600) }
func DirectAssignment()             { f := read; f() }
func execute(f func())              { f() }
func Parameter()                    { execute(read) }
func factory() func()               { return read }
func Factory()                      { f := factory(); f() }
func makeWriter(path string) func() { return func() { _ = os.WriteFile(path, nil, 0600) } }
func OrdinaryCapture(path string)   { f := makeWriter(path); f() }
func identity(f func()) func()      { return f }
func layer(f func()) func()         { return identity(f) }
func Helpers()                      { f := layer(factory()); f() }
func Synchronous(path string)       { execute(makeWriter(path)) }

var saved func()

func Retained()           { saved = makeWriter("later") }
func RetainedDependency() { dep.Retain(read) }
func Goroutine()          { f := factory(); go f() }
func Deferred()           { f := factory(); defer f() }
func Conditional(b bool) {
	f := read
	if b {
		f = write
	}
	f()
}
func conditionalFactory(b bool) func() {
	if b {
		return read
	}
	return write
}
func ConditionalReturn(b bool) { f := conditionalFactory(b); f() }

type holder struct{ f func() }

func Struct()                   { h := holder{read}; h.f() }
func Interface()                { var a any = read; a.(func())() }
func Dependency()               { f := dep.Identity(makeWriter("cross")); f() }
func DependencyInvoke()         { dep.Execute(read) }
func External(f func())         { f() }
func ExternalIdentity(f func()) { g := identity(f); g() }
func ReturnOnly() func()        { return makeWriter("not invoked") }
func cyclic(b bool) func() {
	if b {
		return read
	}
	return cyclic(b)
}
func Recursive(b bool)                { f := cyclic(b); f() }
func mutualA() func()                 { return mutualB() }
func mutualB() func()                 { return mutualA() }
func Cyclic()                         { f := mutualA(); f() }
func CaptureFunction(f func()) func() { return func() { f() } }
func CapturedFunction()               { f := CaptureFunction(write); f() }
func MixedExternal(b bool, f func()) {
	g := read
	if b {
		g = f
	}
	g()
}
func NilAlternative(b bool) {
	f := read
	if b {
		f = nil
	}
	f()
}
func mutate(f *func()) { *f = write }
func Aliased()         { f := read; mutate(&f); f() }

var global = read

func Global()         { global() }
func Opaque(f func()) // deliberately bodyless
func OpaqueCall()     { Opaque(read) }
func SamePhi(b bool) {
	f := read
	if b {
		f = read
	}
	f()
}
func pair() (int, func()) { return 1, read }
func Tuple()              { _, f := pair(); f() }

func SummaryConditional(b bool) {
	f := os.Chdir
	if b {
		f = os.Remove
	}
	_ = f("same")
}
func SummaryOpen(b bool, f func(string) error) {
	g := os.Chdir
	if b {
		g = f
	}
	_ = g("unknown")
}
func SingleSummary()                     { f := os.Remove; _ = f("single") }
func summaryFactory() func(string) error { return os.Remove }
func ReturnedSummary()                   { f := summaryFactory(); _ = f("returned") }
func SameCapture(b bool, first, second string) {
	f := makeWriter(first)
	if b {
		f = makeWriter(second)
	}
	f()
}
func OrdinaryUnknownCapture(path string, data []byte) {
	f := func() { _ = os.WriteFile(path, data, 0600) }
	f()
}

func genericIdentity[T any](f func()) func() { return f }
func GenericReturn()                         { f := genericIdentity[int](read); f() }

type action func()

func namedIdentity(f action) action { return f }
func NamedReturn()                  { f := namedIdentity(read); f() }
func ClosedWithOpaqueBody(b bool, callback func()) {
	f := CaptureFunction(callback)
	if b {
		f = read
	}
	f()
}
func deferredFactory() func() { defer read(); return write }
func DeferredFactoryResult()  { f := deferredFactory(); f() }

func ConditionalGoroutine(b bool) {
	f := factory()
	if b {
		f = write
	}
	go f()
}
func ConditionalDeferred(b bool) {
	f := factory()
	if b {
		f = write
	}
	defer f()
}
func openReturn(b bool, f func()) func() {
	if b {
		return read
	}
	return f
}
func OpenReturned(b bool, f func()) { g := openReturn(b, f); g() }
func OpaqueFactory() func()
func OpaqueReturned() { f := OpaqueFactory(); f() }
