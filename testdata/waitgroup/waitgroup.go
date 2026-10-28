// Package waitgroup is analyzed by CLI tests and never executed.
package waitgroup

import "sync"

func AddDoneWait(group *sync.WaitGroup) {
	group.Add(1)
	group.Done()
	group.Wait()
}

func StartWithGo(group *sync.WaitGroup) {
	group.Go(func() {})
}

func Caller(group *sync.WaitGroup) { AddDoneWait(group) }

type FakeWaitGroup struct{}

func (FakeWaitGroup) Add(int)   {}
func (FakeWaitGroup) Done()     {}
func (FakeWaitGroup) Wait()     {}
func (FakeWaitGroup) Go(func()) {}

func FakeOperations(group FakeWaitGroup) {
	group.Add(1)
	group.Done()
	group.Wait()
	group.Go(func() {})
}

type AddOnly interface{ Add(int) }

func OpenAdd(group AddOnly) { group.Add(1) }
