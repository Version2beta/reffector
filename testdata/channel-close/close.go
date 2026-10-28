// Package channelclose is analyzed, never executed by the CLI fixtures.
package channelclose

func Close(ch chan struct{}) { close(ch) }

func Caller(ch chan struct{}) { Close(ch) }

func ShadowedClose(ch chan struct{}) {
	close := func(chan struct{}) {}
	close(ch)
}

func NoClose() {
	ch := make(chan struct{})
	_ = ch
}
