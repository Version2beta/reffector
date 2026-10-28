package dependency

type Handler interface{ Handle() }

func Dispatch(handler Handler) { handler.Handle() }

func Callback(callback func()) { callback() }

func First()  { Second() }
func Second() { First() }
