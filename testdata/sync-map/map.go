package syncmapfixture

import "sync"

type FakeMap struct{}

func (*FakeMap) Load(any) (any, bool)      { return nil, false }
func (*FakeMap) Store(any, any)            {}
func (*FakeMap) Range(func(any, any) bool) {}

func MapOperations(m *sync.Map) {
	m.Load("load")
	m.Store("store", 1)
	m.Delete("delete")
	m.LoadAndDelete("load-delete")
	m.LoadOrStore("load-store", 1)
	m.Swap("swap", 1)
	m.CompareAndSwap("compare-swap", 1, 2)
	m.CompareAndDelete("compare-delete", 1)
	m.Range(func(any, any) bool { return true })
	m.Clear()
}

func Helper(m *sync.Map) {
	m.Store("helper", 1)
}

func Caller(m *sync.Map) {
	Helper(m)
}

func SameNamedMethods(m *FakeMap) {
	m.Load("fake")
	m.Store("fake", 1)
	m.Range(func(any, any) bool { return true })
}

func PlainMapOperations(m map[string]int) {
	m["key"] = 1
	_ = m["key"]
	delete(m, "key")
}
