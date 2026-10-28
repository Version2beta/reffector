package dep

func Identity(f func()) func() { return f }
func Factory() func()          { return func() {} }
func Execute(f func())         { f() }
func Retain(f func())          { saved = f }

var saved func()
