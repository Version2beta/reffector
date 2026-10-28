// Package synconce is analyzed by CLI tests and never executed.
package synconce

import (
	"os"
	"sync"
)

func OnceDo(once *sync.Once) { once.Do(func() {}) }

func OnceCaller(once *sync.Once) { OnceDo(once) }

func OnceWithOpaqueCallback(once *sync.Once) {
	once.Do(func() { _ = os.WriteFile("callback", []byte("x"), 0600) })
}

type FakeOnce struct{}

func (FakeOnce) Do(func()) {}

func FakeDo(once FakeOnce) { once.Do(func() {}) }

func OnceFuncResult(callback func()) {
	run := sync.OnceFunc(callback)
	run()
}

func OnceValueResult(callback func() int) int {
	get := sync.OnceValue(callback)
	return get()
}

func OnceValuesResult(callback func() (int, error)) (int, error) {
	get := sync.OnceValues(callback)
	return get()
}

func OnceFuncSameConstructorPhi(callback func(), first bool) {
	var run func()
	if first {
		run = sync.OnceFunc(callback)
	} else {
		run = sync.OnceFunc(callback)
	}
	run()
}

func OnceFuncWithCallbackEffect() {
	run := sync.OnceFunc(func() { _ = os.WriteFile("callback", []byte("x"), 0600) })
	run()
}

func OnceFuncConstructOnly(callback func()) {
	_ = sync.OnceFunc(callback)
}

func identityOnceFunc(callback func()) func() { return callback }

func FakeOnceFuncResult(callback func()) {
	run := identityOnceFunc(callback)
	run()
}

func FakeNamedOnceFuncResult(fake FakeOnce, callback func()) {
	run := fake.OnceFunc(callback)
	run()
}

func MixedOnceFuncResult(callback func(), useOnce bool) {
	var run func()
	if useOnce {
		run = sync.OnceFunc(callback)
	} else {
		run = identityOnceFunc(callback)
	}
	run()
}

func makeOnceFunc(callback func()) func() { return sync.OnceFunc(callback) }

func HelperReturnedOnceFuncResult(callback func()) {
	makeOnceFunc(callback)()
}

func (FakeOnce) OnceFunc(callback func()) func() { return callback }
