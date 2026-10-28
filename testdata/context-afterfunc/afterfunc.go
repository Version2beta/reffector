// Package contextafterfunc is analyzed by CLI tests and never executed.
package contextafterfunc

import "context"

func noOp() {}

func Schedule(ctx context.Context) {
	context.AfterFunc(ctx, noOp)
}

func NilContext() {
	context.AfterFunc(nil, noOp)
}

func StopNow(ctx context.Context) {
	stop := context.AfterFunc(ctx, noOp)
	stop()
}

func StopDeferred(ctx context.Context) {
	stop := context.AfterFunc(ctx, noOp)
	defer stop()
}

func ReturnStop(ctx context.Context) func() bool {
	return context.AfterFunc(ctx, noOp)
}

func CallReturnedStop(ctx context.Context) bool {
	return ReturnStop(ctx)()
}

func MixedStop(ctx context.Context, useContextStop bool) bool {
	stop := func() bool { return false }
	if useContextStop {
		stop = context.AfterFunc(ctx, noOp)
	}
	return stop()
}

func PhiStops(first, second context.Context, chooseFirst bool) bool {
	var stop func() bool
	if chooseFirst {
		stop = context.AfterFunc(first, noOp)
	} else {
		stop = context.AfterFunc(second, noOp)
	}
	return stop()
}

func InvokeArbitraryStop(stop func() bool) bool {
	return stop()
}

func LocalArbitraryStop() bool {
	stop := func() bool { return true }
	return stop()
}

type FakeContext struct{}

func (FakeContext) AfterFunc(func()) func() bool {
	return func() bool { return true }
}

func FakeAfterFunc(fake FakeContext) bool {
	return fake.AfterFunc(noOp)()
}

func CallbackEffect(ctx context.Context, ch chan int) {
	context.AfterFunc(ctx, func() { close(ch) })
}
