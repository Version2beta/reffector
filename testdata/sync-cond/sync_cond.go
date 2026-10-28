// Package synccond is analyzed by CLI tests and never executed.
package synccond

import (
	"os"
	"sync"
)

func ConditionOperations(cond *sync.Cond) {
	cond.Wait()
	cond.Signal()
	cond.Broadcast()
}

func Caller(cond *sync.Cond) { ConditionOperations(cond) }

type OpaqueLocker struct{}

func (*OpaqueLocker) Lock() {}
func (*OpaqueLocker) Unlock() {
	_ = os.WriteFile("unused", nil, 0o600)
}

func WaitWithOpaqueLocker() {
	cond := &sync.Cond{L: &OpaqueLocker{}}
	cond.Wait()
}

type FakeCond struct{}

func (*FakeCond) Wait()      {}
func (*FakeCond) Signal()    {}
func (*FakeCond) Broadcast() {}

func SameNamedOperations(cond *FakeCond) {
	cond.Wait()
	cond.Signal()
	cond.Broadcast()
}

type CondLike interface {
	Wait()
	Signal()
	Broadcast()
}

func OpenConditionOperations(cond CondLike) {
	cond.Wait()
	cond.Signal()
	cond.Broadcast()
}
