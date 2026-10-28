// Package waitgroupfixture is analyzed by CLI tests and never executed.
package waitgroupfixture

import (
	"sync"

	"example.com/dependency"
)

type Waiter struct{}

func (Waiter) Handle() {
	var group sync.WaitGroup
	group.Add(1)
	group.Done()
	group.Wait()
}

type Empty struct{}

func (Empty) Handle() {}

var shared dependency.Handler

func setShared() { shared = Waiter{} }

func UnknownGlobal() { forward(shared) }

func forward(handler dependency.Handler) { dependency.Dispatch(handler) }

func Root() { forward(Waiter{}) }

func ClosedBoundary() { forward(Empty{}) }

func MixedBoundary() {
	forward(Waiter{})
	forward(Waiter{})
	forward(Empty{})
}

func OpenBoundary(handler dependency.Handler) { forward(handler) }
