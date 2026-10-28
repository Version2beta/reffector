package opaquefixture

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

func makeEmpty() dependency.Handler { return Empty{} }

// ReturnBoundary dispatches the result of a helper. Per-root candidate
// refinement cannot propagate this interface result back to the caller.
func ReturnBoundary() { dependency.Dispatch(makeEmpty()) }

// CaptureBoundary invokes an interface method on a value captured by a
// callback. Per-root candidate refinement does not inspect closure captures.
func CaptureBoundary() {
	handler := dependency.Handler(Empty{})
	dependency.Callback(func() { handler.Handle() })
}

// ClosedBoundary is a direct-argument control for interface refinement.
func ClosedBoundary() { dependency.Dispatch(Empty{}) }

// Seed keeps Waiter in the program-wide VTA candidate set.
func Seed() { dependency.Dispatch(Waiter{}) }
