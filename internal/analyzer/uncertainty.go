package analyzer

import (
	"go/token"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/Version2beta/reffector/internal/effects"
)

func appendUnique(values []string, value string) []string {
	for _, v := range values {
		if v == value {
			return values
		}
	}
	values = append(values, value)
	sort.Strings(values)
	return values
}

// The uncertainty boolean is propagated as before. Only direct origins carry
// reasons and locations; inherited explanations are paths to these grouped sites.
func (g *graph) unknown(n *node, pos token.Pos, api, reason string) {
	n.uncertain, n.directUncertain = true, true
	g.addSite(n, pos, api, effects.Unknown, "unresolved", reason)
}

func summaryRecognition(fn *ssa.Function) string {
	obj, _ := fn.Object().(*types.Func)
	if _, ok := effects.LookupInterface(obj); ok {
		return "interface_summary"
	}
	return "summary"
}

func summaryReason(api string, set effects.Set) string {
	if set&effects.Unknown == 0 {
		return ""
	}
	switch api {
	case "os/signal.Notify", "os/signal.NotifyContext", "os/signal.Ignore", "os/signal.Reset", "time.NewTicker", "time.Tick", "time.(*Ticker).Reset", "time.Sleep":
		return "argument_condition"
	case "os/signal.Stop", "context.AfterFunc.stop", "time.(*Timer).Stop", "time.(*Timer).Reset":
		return "state_condition"
	case "context.AfterFunc", "time.AfterFunc", "sync.(*Map).Range", "sync.(*Pool).Get", "crypto/x509.(*CertPool).AddCertWithConstraint":
		return "callback"
	case "crypto/x509.(*CertPool).AppendCertsFromPEM":
		return "argument_condition"
	case "sync.(*Map).LoadAndDelete", "sync.(*Map).LoadOrStore", "sync.(*Map).CompareAndSwap", "sync.(*Map).CompareAndDelete", "sync.(*Cond).Signal", "sync.(*Cond).Broadcast":
		return "state_condition"
	case "sync.(*Cond).Wait":
		return "interface_dispatch"
	case "net/http.(ResponseWriter).Write", "net/http.(ResponseWriter).WriteHeader", "net.(Conn).Close", "net.(PacketConn).Close", "net.(Listener).Close", "io.(Closer).Close":
		return "interface_dispatch"
	}
	if strings.HasPrefix(api, "reflect.") {
		return "reflection_opaque"
	}
	// Summary Unknown bits denote unmodeled callbacks, drivers, transports or
	// hooks. Do not invent call targets or pretend the summary is exhaustive.
	return "callback"
}
