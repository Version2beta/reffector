// Package concurrency is analyzed, never executed by the CLI fixtures.
package concurrency

import (
	"bytes"
	"os"
	"sync"
)

func Goroutine() { go Work() }
func Deferred()  { defer os.Remove("not-executed") }
func Work()      {}

func Send(ch chan<- int)        { ch <- 1 }
func Receive(ch <-chan int) int { return <-ch }
func Select(send chan<- int, receive <-chan int) {
	select {
	case send <- 1:
	case <-receive:
	default:
	}
}

func Inherited(ch chan<- int)                 { Send(ch) }
func GenericSend[T any](ch chan<- T, value T) { ch <- value }
func GenericUse(ch chan<- int)                { GenericSend(ch, 1) }

func MutexOperations(mu *sync.Mutex) { mu.Lock(); mu.Unlock() }
func MutexCaller(mu *sync.Mutex)     { MutexOperations(mu) }

type FakeLocker struct{}

func (FakeLocker) Lock()    {}
func FakeLock(f FakeLocker) { f.Lock() }

func LocalStore() {
	value := 1
	value = 2
	_ = value
}

func BufferWrite() {
	var buffer bytes.Buffer
	_, _ = buffer.Write([]byte("local"))
}

type Writer interface{ Write([]byte) (int, error) }
type Unrelated struct{}

func (Unrelated) Write(p []byte) (int, error) { return len(p), nil }
func UnknownWriter(w Writer)                  { _, _ = w.Write(nil) }
