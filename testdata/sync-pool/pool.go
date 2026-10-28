package syncpoolfixture

import (
	"os"
	"sync"
)

type FakePool struct{}

func (*FakePool) Get() any { return nil }
func (*FakePool) Put(any)  {}

func PoolOperations(p *sync.Pool) {
	_ = p.Get()
	p.Put([]byte("buffer"))
	defer p.Put([]byte("deferred"))
}

func PoolWithNewCallback() {
	p := &sync.Pool{New: func() any {
		_ = os.WriteFile("callback", nil, 0o600)
		return []byte{}
	}}
	_ = p.Get()
}

func Helper(p *sync.Pool) {
	p.Put("helper")
}

func Caller(p *sync.Pool) {
	Helper(p)
}

func SameNamedMethods(p *FakePool) {
	_ = p.Get()
	p.Put(nil)
}
