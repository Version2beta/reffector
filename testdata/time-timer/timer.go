// Package timertimer is analyzed by CLI tests and never executed.
package timertimer

import "time"

func Schedule(callback func()) {
	time.AfterFunc(time.Second, callback)
}

func ScheduleInline(ch chan int) {
	time.AfterFunc(time.Second, func() { close(ch) })
}

func StopNow(timer *time.Timer) bool {
	return timer.Stop()
}

func StopNewTimer() {
	timer := time.NewTimer(time.Second)
	timer.Stop()
}

func StopDeferred(timer *time.Timer) {
	defer timer.Stop()
}

func ResetNow(timer *time.Timer) bool {
	return timer.Reset(time.Second)
}

func ResetAfterFunc(callback func()) bool {
	timer := time.AfterFunc(time.Second, callback)
	return timer.Reset(time.Second)
}

func AfterChannel() <-chan time.Time { return time.After(time.Second) }

func NewTimerOnly() *time.Timer { return time.NewTimer(time.Second) }

func NewTickerAndStop() {
	ticker := time.NewTicker(time.Second)
	ticker.Stop()
}

func TickChannel() <-chan time.Time { return time.Tick(time.Second) }

func NewTickerDynamic(interval time.Duration) *time.Ticker { return time.NewTicker(interval) }

func NewTickerInvalid() { time.NewTicker(0) }

func TickInvalid() <-chan time.Time { return time.Tick(0) }

type FakeTimer struct{}

func (FakeTimer) Stop() bool               { return true }
func (FakeTimer) Reset(time.Duration) bool { return true }

func FakeTimerOperations(timer FakeTimer) {
	timer.Stop()
	timer.Reset(time.Second)
}

type FakeTicker struct{}

func (FakeTicker) Stop() {}

func FakeTickerStop(ticker FakeTicker) { ticker.Stop() }

func TimerCaller() { _ = AfterChannel() }

func TickerResetKnown(ticker *time.Ticker) { ticker.Reset(time.Second) }

func TickerResetDynamic(ticker *time.Ticker, period time.Duration) { ticker.Reset(period) }

func TickerResetInvalid(ticker *time.Ticker) { ticker.Reset(0) }

type FakeTickerWithReset struct{}

func (FakeTickerWithReset) Reset(time.Duration) {}

func FakeTickerReset(ticker FakeTickerWithReset) { ticker.Reset(time.Second) }

func SleepKnown() { time.Sleep(time.Second) }

func SleepDynamic(duration time.Duration) { time.Sleep(duration) }

func SleepZero() { time.Sleep(0) }

func SleepNegative() { time.Sleep(-time.Second) }

func SleepCaller() { SleepKnown() }

func Sleep(time.Duration) {}

func FakeSleep(duration time.Duration) { Sleep(duration) }
