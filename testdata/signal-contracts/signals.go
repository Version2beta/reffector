// Package signalcontracts is analyzed by CLI tests and never executed.
package signalcontracts

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

type customSignal struct{}

func (customSignal) Signal()        {}
func (customSignal) String() string { return "custom" }

func Register(ch chan os.Signal) { signal.Notify(ch, os.Interrupt) }

func RegisterFresh() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
}

func OverrideInterrupt() {
	os.Interrupt = customSignal{}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
}

func RegisterSyscall() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM)
}

func RegisterUnknown(sig os.Signal) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sig)
}

func UnsupportedSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, customSignal{})
}

func MixedSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, customSignal{}, os.Interrupt)
}

func RegisterAll() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch)
}

func NilChannel() { signal.Notify(nil, os.Interrupt) }

func Caller(ch chan os.Signal) { Register(ch) }

func Unregister(ch chan os.Signal) { signal.Stop(ch) }

func UnregisterNil() { signal.Stop(nil) }

func UnregisterFresh() {
	ch := make(chan os.Signal, 1)
	signal.Stop(ch)
}

func UnregisterAfterNotify() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch)
	signal.Stop(ch)
}

type FakeSignalStop struct{}

func (FakeSignalStop) Stop(chan os.Signal) {}

func FakeStop(fake FakeSignalStop, ch chan os.Signal) { fake.Stop(ch) }

func IgnoreSignal() { signal.Ignore(os.Interrupt) }

func ResetSignals() { signal.Reset(os.Interrupt) }

func IgnoreAll() { signal.Ignore() }

func ResetAll() { signal.Reset() }

func IgnoreNilSlice() { signal.Ignore(([]os.Signal)(nil)...) }

func ResetNilSlice() { signal.Reset(([]os.Signal)(nil)...) }

func IgnoreUnsupported() { signal.Ignore(customSignal{}) }

func ResetUnsupported() { signal.Reset(customSignal{}) }

func IgnoreNilSignal() { signal.Ignore(nil) }

func ResetNilSignal() { signal.Reset(nil) }

func IgnoreDynamic(sig os.Signal) { signal.Ignore(sig) }

func ResetDynamic(sig os.Signal) { signal.Reset(sig) }

func IgnoreSignalSlice(signals []os.Signal) { signal.Ignore(signals...) }

func ResetSignalSlice(signals []os.Signal) { signal.Reset(signals...) }

func IgnoreMixedSignals() { signal.Ignore(customSignal{}, os.Interrupt) }

func ResetMixedSignals() { signal.Reset(customSignal{}, os.Interrupt) }

type FakeSignalPolicy struct{}

func (FakeSignalPolicy) Ignore(...os.Signal) {}
func (FakeSignalPolicy) Reset(...os.Signal)  {}

func FakeIgnore(fake FakeSignalPolicy) { fake.Ignore(os.Interrupt) }
func FakeReset(fake FakeSignalPolicy)  { fake.Reset(os.Interrupt) }

func RegisterContext(parent context.Context) {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt)
	_ = ctx
	defer stop()
}

func RegisterContextFresh(parent context.Context) {
	ctx, stop := signal.NotifyContext(parent, os.Interrupt)
	_ = ctx
	stop()
}

func RegisterContextAll(parent context.Context) {
	_, _ = signal.NotifyContext(parent)
}

func UnsupportedContext(parent context.Context) {
	_, stop := signal.NotifyContext(parent, customSignal{})
	stop()
}

func NilContext() {
	_, stop := signal.NotifyContext(nil, os.Interrupt)
	stop()
}

func ReturnContextStop(parent context.Context) context.CancelFunc {
	_, stop := signal.NotifyContext(parent, os.Interrupt)
	return stop
}

func CallReturnedContextStop(parent context.Context) {
	stop := ReturnContextStop(parent)
	stop()
}

func MixedContextStop(parent context.Context, useSignal bool) {
	var stop context.CancelFunc
	if useSignal {
		_, stop = signal.NotifyContext(parent, os.Interrupt)
	} else {
		stop = context.CancelFunc(func() {})
	}
	stop()
}

func InvokeArbitraryStop(stop context.CancelFunc) { stop() }

func LocalArbitraryStop() {
	stop := context.CancelFunc(func() {})
	stop()
}

func ReturnArbitraryStop() context.CancelFunc {
	return context.CancelFunc(func() {})
}

func CallReturnedArbitraryStop() { ReturnArbitraryStop()() }

type FakeSignal struct{}

func (FakeSignal) Notify(chan os.Signal) {}

func FakeNotify(fake FakeSignal, ch chan os.Signal) { fake.Notify(ch) }
