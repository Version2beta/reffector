// Analysis-only signal argument regressions; never execute these functions.
package signalarguments

import (
	"os"
	"os/signal"
	"syscall"
)

type customSignal struct{}

func (customSignal) Signal()        {}
func (customSignal) String() string { return "custom" }
func mutate(s []os.Signal)          { s[0] = syscall.SIGTERM }
func inspect(s []os.Signal)         { _ = s[0] }

var escaped []os.Signal

func NotifyMutated() {
	ch := make(chan os.Signal, 1)
	s := []os.Signal{customSignal{}}
	mutate(s)
	signal.Notify(ch, s...)
}

func NotifyPointerEmptyCustom() {
	ch := make(chan os.Signal, 1)
	a := &[1]os.Signal{customSignal{}}
	signal.Notify(ch, a[:0]...)
}

func NotifyDirectUnsupported() { ch := make(chan os.Signal, 1); signal.Notify(ch, customSignal{}) }

func NotifyDirectNil() { ch := make(chan os.Signal, 1); signal.Notify(ch, nil) }

func NotifyDirectSupported() { ch := make(chan os.Signal, 1); signal.Notify(ch, syscall.SIGTERM) }

func NotifyDirectMixed() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, customSignal{}, syscall.SIGTERM)
}

func NotifyDirectUnknown(value os.Signal) { ch := make(chan os.Signal, 1); signal.Notify(ch, value) }

func NotifyNoArgs() { ch := make(chan os.Signal, 1); signal.Notify(ch) }

func NotifyNilSlice() { ch := make(chan os.Signal, 1); var s []os.Signal; signal.Notify(ch, s...) }

func NotifyEmptyLiteral() { ch := make(chan os.Signal, 1); signal.Notify(ch, []os.Signal{}...) }

func NotifyPointerPrefix() {
	ch := make(chan os.Signal, 1)
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Notify(ch, a[:1]...)
}

func NotifyPointerSuffix() {
	ch := make(chan os.Signal, 1)
	a := &[2]os.Signal{syscall.SIGTERM, customSignal{}}
	signal.Notify(ch, a[1:]...)
}

func NotifyPointerFull() {
	ch := make(chan os.Signal, 1)
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Notify(ch, a[:]...)
}

func NotifyPointerEmptyRange() {
	ch := make(chan os.Signal, 1)
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Notify(ch, a[1:1]...)
}

func NotifyPointerMaxEmpty() {
	ch := make(chan os.Signal, 1)
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Notify(ch, a[1:1:2]...)
}

func NotifyPointerMaxUnsupported() {
	ch := make(chan os.Signal, 1)
	a := &[2]os.Signal{syscall.SIGTERM, customSignal{}}
	signal.Notify(ch, a[1:2:2]...)
}

func NotifyPointerMaxSupported() {
	ch := make(chan os.Signal, 1)
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Notify(ch, a[1:2:2]...)
}

func NotifyDynamicLow(n int) {
	ch := make(chan os.Signal, 1)
	a := &[2]os.Signal{customSignal{}, customSignal{}}
	signal.Notify(ch, a[n:2]...)
}

func NotifyDynamicHigh(n int) {
	ch := make(chan os.Signal, 1)
	a := &[2]os.Signal{customSignal{}, customSignal{}}
	signal.Notify(ch, a[:n]...)
}

func NotifyDynamicMax(n int) {
	ch := make(chan os.Signal, 1)
	a := &[2]os.Signal{customSignal{}, customSignal{}}
	signal.Notify(ch, a[0:1:n]...)
}

func NotifyAlias() {
	ch := make(chan os.Signal, 1)
	s := []os.Signal{customSignal{}}
	alias := s
	signal.Notify(ch, alias...)
}

func NotifyAliasMutation() {
	ch := make(chan os.Signal, 1)
	s := []os.Signal{customSignal{}}
	alias := s
	alias[0] = syscall.SIGTERM
	signal.Notify(ch, s...)
}

func NotifyReslicedAlias() {
	ch := make(chan os.Signal, 1)
	s := []os.Signal{customSignal{}}
	alias := s[:]
	mutate(alias)
	signal.Notify(ch, s...)
}

func NotifyEscape() {
	ch := make(chan os.Signal, 1)
	s := []os.Signal{customSignal{}}
	escaped = s
	signal.Notify(ch, s...)
}

func NotifyUnknownMutator(change func([]os.Signal)) {
	ch := make(chan os.Signal, 1)
	s := []os.Signal{customSignal{}}
	change(s)
	signal.Notify(ch, s...)
}

func NotifyCompetingRead() {
	ch := make(chan os.Signal, 1)
	s := []os.Signal{customSignal{}}
	inspect(s)
	signal.Notify(ch, s...)
}

func NotifyAfterCallMutation() {
	ch := make(chan os.Signal, 1)
	s := []os.Signal{customSignal{}}
	signal.Notify(ch, s...)
	mutate(s)
}

func NotifyArrayCopy() {
	ch := make(chan os.Signal, 1)
	a := [1]os.Signal{customSignal{}}
	signal.Notify(ch, a[:]...)
}

func NotifyBoundary(s []os.Signal) { ch := make(chan os.Signal, 1); signal.Notify(ch, s...) }

func NotifyConditionalMutation(change bool) {
	ch := make(chan os.Signal, 1)
	s := []os.Signal{customSignal{}}
	if change {
		s[0] = syscall.SIGTERM
	}
	signal.Notify(ch, s...)
}

func IgnoreMutated() { s := []os.Signal{customSignal{}}; mutate(s); signal.Ignore(s...) }

func IgnorePointerEmptyCustom() { a := &[1]os.Signal{customSignal{}}; signal.Ignore(a[:0]...) }

func IgnoreDirectUnsupported() { signal.Ignore(customSignal{}) }

func IgnoreDirectNil() { signal.Ignore(nil) }

func IgnoreDirectSupported() { signal.Ignore(syscall.SIGTERM) }

func IgnoreDirectMixed() { signal.Ignore(customSignal{}, syscall.SIGTERM) }

func IgnoreDirectUnknown(value os.Signal) { signal.Ignore(value) }

func IgnoreNoArgs() { signal.Ignore() }

func IgnoreNilSlice() { var s []os.Signal; signal.Ignore(s...) }

func IgnoreEmptyLiteral() { signal.Ignore([]os.Signal{}...) }

func IgnorePointerPrefix() {
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Ignore(a[:1]...)
}

func IgnorePointerSuffix() {
	a := &[2]os.Signal{syscall.SIGTERM, customSignal{}}
	signal.Ignore(a[1:]...)
}

func IgnorePointerFull() { a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}; signal.Ignore(a[:]...) }

func IgnorePointerEmptyRange() {
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Ignore(a[1:1]...)
}

func IgnorePointerMaxEmpty() {
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Ignore(a[1:1:2]...)
}

func IgnorePointerMaxUnsupported() {
	a := &[2]os.Signal{syscall.SIGTERM, customSignal{}}
	signal.Ignore(a[1:2:2]...)
}

func IgnorePointerMaxSupported() {
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Ignore(a[1:2:2]...)
}

func IgnoreDynamicLow(n int) {
	a := &[2]os.Signal{customSignal{}, customSignal{}}
	signal.Ignore(a[n:2]...)
}

func IgnoreDynamicHigh(n int) {
	a := &[2]os.Signal{customSignal{}, customSignal{}}
	signal.Ignore(a[:n]...)
}

func IgnoreDynamicMax(n int) {
	a := &[2]os.Signal{customSignal{}, customSignal{}}
	signal.Ignore(a[0:1:n]...)
}

func IgnoreAlias() { s := []os.Signal{customSignal{}}; alias := s; signal.Ignore(alias...) }

func IgnoreAliasMutation() {
	s := []os.Signal{customSignal{}}
	alias := s
	alias[0] = syscall.SIGTERM
	signal.Ignore(s...)
}

func IgnoreReslicedAlias() {
	s := []os.Signal{customSignal{}}
	alias := s[:]
	mutate(alias)
	signal.Ignore(s...)
}

func IgnoreEscape() { s := []os.Signal{customSignal{}}; escaped = s; signal.Ignore(s...) }

func IgnoreUnknownMutator(change func([]os.Signal)) {
	s := []os.Signal{customSignal{}}
	change(s)
	signal.Ignore(s...)
}

func IgnoreCompetingRead() { s := []os.Signal{customSignal{}}; inspect(s); signal.Ignore(s...) }

func IgnoreAfterCallMutation() { s := []os.Signal{customSignal{}}; signal.Ignore(s...); mutate(s) }

func IgnoreArrayCopy() { a := [1]os.Signal{customSignal{}}; signal.Ignore(a[:]...) }

func IgnoreBoundary(s []os.Signal) { signal.Ignore(s...) }

func IgnoreConditionalMutation(change bool) {
	s := []os.Signal{customSignal{}}
	if change {
		s[0] = syscall.SIGTERM
	}
	signal.Ignore(s...)
}

func ResetMutated() { s := []os.Signal{customSignal{}}; mutate(s); signal.Reset(s...) }

func ResetPointerEmptyCustom() { a := &[1]os.Signal{customSignal{}}; signal.Reset(a[:0]...) }

func ResetDirectUnsupported() { signal.Reset(customSignal{}) }

func ResetDirectNil() { signal.Reset(nil) }

func ResetDirectSupported() { signal.Reset(syscall.SIGTERM) }

func ResetDirectMixed() { signal.Reset(customSignal{}, syscall.SIGTERM) }

func ResetDirectUnknown(value os.Signal) { signal.Reset(value) }

func ResetNoArgs() { signal.Reset() }

func ResetNilSlice() { var s []os.Signal; signal.Reset(s...) }

func ResetEmptyLiteral() { signal.Reset([]os.Signal{}...) }

func ResetPointerPrefix() {
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Reset(a[:1]...)
}

func ResetPointerSuffix() {
	a := &[2]os.Signal{syscall.SIGTERM, customSignal{}}
	signal.Reset(a[1:]...)
}

func ResetPointerFull() { a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}; signal.Reset(a[:]...) }

func ResetPointerEmptyRange() {
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Reset(a[1:1]...)
}

func ResetPointerMaxEmpty() {
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Reset(a[1:1:2]...)
}

func ResetPointerMaxUnsupported() {
	a := &[2]os.Signal{syscall.SIGTERM, customSignal{}}
	signal.Reset(a[1:2:2]...)
}

func ResetPointerMaxSupported() {
	a := &[2]os.Signal{customSignal{}, syscall.SIGTERM}
	signal.Reset(a[1:2:2]...)
}

func ResetDynamicLow(n int) {
	a := &[2]os.Signal{customSignal{}, customSignal{}}
	signal.Reset(a[n:2]...)
}

func ResetDynamicHigh(n int) {
	a := &[2]os.Signal{customSignal{}, customSignal{}}
	signal.Reset(a[:n]...)
}

func ResetDynamicMax(n int) {
	a := &[2]os.Signal{customSignal{}, customSignal{}}
	signal.Reset(a[0:1:n]...)
}

func ResetAlias() { s := []os.Signal{customSignal{}}; alias := s; signal.Reset(alias...) }

func ResetAliasMutation() {
	s := []os.Signal{customSignal{}}
	alias := s
	alias[0] = syscall.SIGTERM
	signal.Reset(s...)
}

func ResetReslicedAlias() {
	s := []os.Signal{customSignal{}}
	alias := s[:]
	mutate(alias)
	signal.Reset(s...)
}

func ResetEscape() { s := []os.Signal{customSignal{}}; escaped = s; signal.Reset(s...) }

func ResetUnknownMutator(change func([]os.Signal)) {
	s := []os.Signal{customSignal{}}
	change(s)
	signal.Reset(s...)
}

func ResetCompetingRead() { s := []os.Signal{customSignal{}}; inspect(s); signal.Reset(s...) }

func ResetAfterCallMutation() { s := []os.Signal{customSignal{}}; signal.Reset(s...); mutate(s) }

func ResetArrayCopy() { a := [1]os.Signal{customSignal{}}; signal.Reset(a[:]...) }

func ResetBoundary(s []os.Signal) { signal.Reset(s...) }

func ResetConditionalMutation(change bool) {
	s := []os.Signal{customSignal{}}
	if change {
		s[0] = syscall.SIGTERM
	}
	signal.Reset(s...)
}

func NotifyNilChannel()                      { signal.Notify(nil, syscall.SIGTERM) }
func NotifyUnknownChannel(ch chan os.Signal) { signal.Notify(ch) }

// Caller arguments do not bind the separately analyzed callee's parameters.
func NotifyBoundaryCaller() { NotifyBoundary([]os.Signal{customSignal{}}) }
func IgnoreBoundaryCaller() { IgnoreBoundary([]os.Signal{customSignal{}}) }
func ResetBoundaryCaller()  { ResetBoundary([]os.Signal{customSignal{}}) }
