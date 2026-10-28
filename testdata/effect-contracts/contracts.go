// Package effectcontracts is analyzed by CLI tests and never executed.
package effectcontracts

import (
	"context"
	"errors"
	"os"
	"time"
)

func CancelNow(parent context.Context) {
	_, cancel := context.WithCancel(parent)
	cancel()
}

func CancelDeferred(parent context.Context) {
	_, cancel := context.WithCancel(parent)
	defer cancel()
}

func CancelFromDeadline(parent context.Context, deadline time.Time) {
	_, cancel := context.WithDeadline(parent, deadline)
	cancel()
}

func CancelFromDeadlineCause(parent context.Context, deadline time.Time, cause error) {
	_, cancel := context.WithDeadlineCause(parent, deadline, cause)
	cancel()
}

func CancelFromTimeout(parent context.Context, timeout time.Duration) {
	_, cancel := context.WithTimeout(parent, timeout)
	cancel()
}

func CancelFromTimeoutCause(parent context.Context, timeout time.Duration, cause error) {
	_, cancel := context.WithTimeoutCause(parent, timeout, cause)
	cancel()
}

func CancelCauseNow(parent context.Context) {
	_, cancel := context.WithCancelCause(parent)
	cancel(errors.New("cause"))
}

func PlainCallback(callback func()) { callback() }

type LocalCancel func()

func LocalCancellation(cancel LocalCancel) { cancel() }

func FakeContextCancel() {
	cancel := context.CancelFunc(func() {})
	cancel()
}

func FakeContextCancelCause() {
	cancel := context.CancelCauseFunc(func(error) {})
	cancel(errors.New("cause"))
}

func MixedContextCancel(parent context.Context, useReal bool) {
	var cancel context.CancelFunc
	if useReal {
		_, cancel = context.WithCancel(parent)
	} else {
		cancel = context.CancelFunc(func() {})
	}
	cancel()
}

type FakeProcess struct{}

func (FakeProcess) Kill() error                     { return nil }
func (FakeProcess) Signal(int) error                { return nil }
func (FakeProcess) Release() error                  { return nil }
func (FakeProcess) Wait() (*os.ProcessState, error) { return nil, nil }

func FakeProcessOperations(process FakeProcess) {
	_ = process.Kill()
	_ = process.Signal(1)
	_ = process.Release()
	_, _ = process.Wait()
}

func OSProcessOperations(process *os.Process) {
	_ = process.Kill()
	_ = process.Signal(os.Interrupt)
	_ = process.Release()
	_, _ = process.Wait()
}
