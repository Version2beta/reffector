// Package atomicfixture is analyzed, never executed by the CLI fixtures.
package atomicfixture

import "sync/atomic"

func AtomicOperations(flag *atomic.Bool, counter *atomic.Int32, wide *atomic.Int64, ptr *atomic.Pointer[int], value *atomic.Value, legacy *int32) {
	flag.Store(true)
	_ = flag.Load()
	_ = flag.Swap(false)
	_ = flag.CompareAndSwap(false, true)
	counter.Add(1)
	counter.Store(2)
	_ = counter.Load()
	_ = counter.Swap(3)
	_ = counter.CompareAndSwap(3, 4)
	_ = counter.And(7)
	_ = counter.Or(1)
	wide.Add(1)
	_ = wide.Load()
	_ = atomic.LoadInt32(legacy)
	_ = ptr.Load()
	ptr.Store(nil)
	_ = ptr.CompareAndSwap(nil, nil)
	_ = value.Load()
}

func Caller(flag *atomic.Bool, counter *atomic.Int32, wide *atomic.Int64, ptr *atomic.Pointer[int], value *atomic.Value, legacy *int32) {
	AtomicOperations(flag, counter, wide, ptr, value, legacy)
}

type FakeAtomic struct{}

func (FakeAtomic) Store(bool)         {}
func SameNamedMethod(fake FakeAtomic) { fake.Store(true) }

func LocalMutation() {
	value := 0
	value++
	_ = value
}
