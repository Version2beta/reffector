// Package effects defines version one's API summaries. Keys use resolved Go
// package/type identities, never just method names. A summary is a boundary:
// its implementation is not traversed. Keep callback caveats explicit here.
package effects

import (
	"go/constant"
	"go/types"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"
)

type Set uint64

const (
	FSRead Set = 1 << iota
	FSWrite
	FSCreate
	FSRemove
	NetConnect
	NetListen
	ProcessExec
	EnvRead
	EnvWrite
	DBQuery
	DBExec
	ProcessExit
	ProcessControl
	ContextCancel
	HTTPResponse
	ResourceAcquire
	ResourceRelease
	NetClose
	FSRename
	GoroutineStart
	ChannelSend
	ChannelReceive
	ChannelSelect
	ChannelClose
	ConcurrencyLock
	ConcurrencyUnlock
	ConcurrencyTaskAccounting
	ConcurrencyWait
	ProcessSignalRegister
	ProcessSignalUnregister
	ProcessSignalIgnore
	ProcessSignalReset
	ConcurrencyAtomic
	ConcurrencyMapRead
	ConcurrencyMapWrite
	ConcurrencyCallbackRegister
	ConcurrencyCallbackStop
	ConcurrencyTimerSchedule
	ConcurrencyTimerStop
	ConcurrencyTimerReset
	ConcurrencySleep
	ConcurrencyPoolGet
	ConcurrencyPoolPut
	ConcurrencyCertPoolAdd
	CertPoolSystem
	// Unknown is an internal uncertainty marker, never an output effect.
	Unknown
	ConcurrencyConditionSignal
	ConcurrencyConditionBroadcast
)

var names = map[Set]string{
	FSRead: "fs.read", FSWrite: "fs.write", FSCreate: "fs.create", FSRemove: "fs.remove",
	NetConnect: "net.connect", NetListen: "net.listen", ProcessExec: "process.exec",
	EnvRead: "env.read", EnvWrite: "env.write", DBQuery: "db.query", DBExec: "db.exec", ProcessExit: "process.exit",
	ProcessControl: "process.control", ContextCancel: "context.cancel",
	HTTPResponse: "http.response", ResourceAcquire: "resource.acquire", ResourceRelease: "resource.release", NetClose: "net.close", FSRename: "fs.rename",
	GoroutineStart: "concurrency.goroutine", ChannelSend: "channel.send", ChannelReceive: "channel.receive", ChannelSelect: "channel.select", ChannelClose: "channel.close",
	ConcurrencyLock: "concurrency.lock", ConcurrencyUnlock: "concurrency.unlock",
	ConcurrencyTaskAccounting: "concurrency.task", ConcurrencyWait: "concurrency.wait",
	ProcessSignalRegister: "process.signal.register", ProcessSignalUnregister: "process.signal.unregister",
	ProcessSignalIgnore: "process.signal.ignore", ProcessSignalReset: "process.signal.reset",
	ConcurrencyAtomic:           "concurrency.atomic",
	ConcurrencyMapRead:          "concurrency.map.read",
	ConcurrencyMapWrite:         "concurrency.map.write",
	ConcurrencyCallbackRegister: "concurrency.callback.register", ConcurrencyCallbackStop: "concurrency.callback.stop",
	ConcurrencyTimerSchedule: "concurrency.timer.schedule", ConcurrencyTimerStop: "concurrency.timer.stop",
	ConcurrencyTimerReset:         "concurrency.timer.reset",
	ConcurrencySleep:              "concurrency.sleep",
	ConcurrencyPoolGet:            "concurrency.pool.get",
	ConcurrencyPoolPut:            "concurrency.pool.put",
	ConcurrencyCertPoolAdd:        "crypto.x509.certpool.add",
	CertPoolSystem:                "crypto.x509.certpool.system",
	ConcurrencyConditionSignal:    "concurrency.condition.signal",
	ConcurrencyConditionBroadcast: "concurrency.condition.broadcast",
}

func (s Set) Names() []string {
	var result []string
	for bit, name := range names {
		if s&bit != 0 {
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result
}

// Key returns a canonical identity, e.g. os.ReadFile or os.(*File).Read.
func Key(fn *types.Func) string {
	if fn == nil || fn.Pkg() == nil {
		return ""
	}
	key := fn.Pkg().Path() + "."
	if recv := fn.Signature().Recv(); recv != nil {
		key += "(" + types.TypeString(recv.Type(), func(*types.Package) string { return "" }) + ")."
	}
	return key + fn.Name()
}

// LookupInterface summarizes the named interface contract, not a concrete
// implementation. It never supplies a call target. Unknown io.Writer stays
// unclassified: its resource kind and destination are not established.
func LookupInterface(fn *types.Func) (Set, bool) {
	switch Key(fn) {
	case "net/http.(ResponseWriter).Write", "net/http.(ResponseWriter).WriteHeader":
		return HTTPResponse, true
	case "net.(Conn).Close", "net.(PacketConn).Close", "net.(Listener).Close":
		return NetClose | ResourceRelease, true
	case "io.(Closer).Close":
		// Embedded standard contracts (ReadCloser, ReadWriteCloser, etc.)
		// retain the declaring Closer method identity. Resource kind is unknown.
		return ResourceRelease, true
	}
	return 0, false
}

func Lookup(fn *types.Func) (Set, bool) {
	if s, ok := summaries[Key(fn)]; ok {
		return s, true
	}
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != "sync/atomic" || fn.Signature().Recv() == nil {
		return 0, false
	}
	receiver, ok := types.Unalias(fn.Signature().Recv().Type()).(*types.Pointer)
	if !ok {
		return 0, false
	}
	named, ok := types.Unalias(receiver.Elem()).(*types.Named)
	if !ok {
		return 0, false
	}
	origin := named.Origin()
	if origin.Obj().Pkg() == nil || origin.Obj().Pkg().Path() != "sync/atomic" || origin.Obj().Name() != "Pointer" {
		return 0, false
	}
	// SSA may expose an instantiated receiver (*Pointer[T]) as (*Pointer[int]),
	// while the summary table keys the declared generic method.
	s, ok := summaries["sync/atomic.(*Pointer[T])."+fn.Name()]
	return s, ok
}

// LookupCall recognizes summaries whose effects depend on call arguments.
// LookupCallValue recognizes function-value calls whose local SSA provenance
// and standard-library contract support a specific operation summary.
func LookupCallValue(value ssa.Value) (string, Set, bool) {
	if value == nil {
		return "", 0, false
	}
	switch value.(type) {
	case *ssa.Call, *ssa.ChangeType, *ssa.Phi:
		if api, ok := syncOnceFunctionResult(value, make(map[ssa.Value]syncOnceProvenance)); ok {
			return api, ConcurrencyWait | Unknown, true
		}
	}
	if contextAfterFuncStopResult(value, make(map[ssa.Value]bool)) {
		return "context.AfterFunc.stop", ConcurrencyCallbackStop | Unknown, true
	}
	cancelType := contextCancelType(value.Type())
	if cancelType == "" {
		return "", 0, false
	}
	if cancelType == "CancelFunc" && signalNotifyContextStopResult(value, make(map[ssa.Value]bool)) {
		return "os/signal.NotifyContext.stop", ContextCancel | ProcessSignalUnregister, true
	}
	if contextCancelResult(value, make(map[ssa.Value]bool)) {
		return "context." + cancelType, ContextCancel, true
	}
	return "", 0, false
}

// contextAfterFuncStopResult recognizes a stop function only when local SSA
// traces every value path directly to context.AfterFunc's result. Helper
// returns and mixed or arbitrary function values remain unresolved.
func contextAfterFuncStopResult(value ssa.Value, seen map[ssa.Value]bool) bool {
	if value == nil || seen[value] {
		return false
	}
	seen[value] = true
	defer delete(seen, value)
	switch value := value.(type) {
	case *ssa.Call:
		callee := value.Common().StaticCallee()
		if callee == nil {
			return false
		}
		fn, ok := callee.Object().(*types.Func)
		return ok && Key(fn) == "context.AfterFunc"
	case *ssa.ChangeType:
		return contextAfterFuncStopResult(value.X, seen)
	case *ssa.Phi:
		if len(value.Edges) == 0 {
			return false
		}
		for _, edge := range value.Edges {
			if !contextAfterFuncStopResult(edge, seen) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// syncOnceProvenance memoizes the result while keeping active SSA cycles
// distinct from completed negative lookups.
type syncOnceProvenance struct {
	api      string
	resolved bool
	complete bool
}

// syncOnceFunctionResult recognizes returned functions only when every local
// SSA value path leads to the same exact sync.Once* constructor. It does not
// follow helper returns, memory, or arbitrary callbacks.
func syncOnceFunctionResult(value ssa.Value, memo map[ssa.Value]syncOnceProvenance) (string, bool) {
	if value == nil {
		return "", false
	}
	if result, ok := memo[value]; ok {
		if !result.complete {
			return "", false
		}
		return result.api, result.resolved
	}
	memo[value] = syncOnceProvenance{}
	api, resolved := syncOnceFunctionOrigin(value, memo)
	memo[value] = syncOnceProvenance{api: api, resolved: resolved, complete: true}
	return api, resolved
}

func syncOnceFunctionOrigin(value ssa.Value, memo map[ssa.Value]syncOnceProvenance) (string, bool) {
	switch value := value.(type) {
	case *ssa.Call:
		callee := value.Common().StaticCallee()
		if callee == nil {
			return "", false
		}
		fn, ok := callee.Object().(*types.Func)
		if !ok {
			return "", false
		}
		switch Key(fn) {
		case "sync.OnceFunc", "sync.OnceValue", "sync.OnceValues":
			return Key(fn), true
		default:
			return "", false
		}
	case *ssa.ChangeType:
		return syncOnceFunctionResult(value.X, memo)
	case *ssa.Phi:
		if len(value.Edges) == 0 {
			return "", false
		}
		api := ""
		for _, edge := range value.Edges {
			edgeAPI, ok := syncOnceFunctionResult(edge, memo)
			if !ok || (api != "" && edgeAPI != api) {
				return "", false
			}
			api = edgeAPI
		}
		return api, api != ""
	default:
		return "", false
	}
}

func contextCancelType(t types.Type) string {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != "context" {
		return ""
	}
	switch named.Obj().Name() {
	case "CancelFunc", "CancelCauseFunc":
		return named.Obj().Name()
	default:
		return ""
	}
}

// signalNotifyContextStopResult recognizes only stop functions directly
// extracted from os/signal.NotifyContext. Helper returns, arbitrary
// CancelFuncs, and mixed-provenance phi values remain unresolved.
func signalNotifyContextStopResult(value ssa.Value, seen map[ssa.Value]bool) bool {
	if value == nil || seen[value] {
		return false
	}
	seen[value] = true
	defer delete(seen, value)
	switch value := value.(type) {
	case *ssa.Extract:
		if value.Index != 1 {
			return false
		}
		call, ok := value.Tuple.(*ssa.Call)
		if !ok || len(call.Common().Args) == 0 || staticallyNil(call.Common().Args[0], make(map[ssa.Value]bool)) {
			return false
		}
		callee := call.Common().StaticCallee()
		if callee == nil {
			return false
		}
		fn, ok := callee.Object().(*types.Func)
		return ok && Key(fn) == "os/signal.NotifyContext" && contextCancelType(value.Type()) == "CancelFunc"
	case *ssa.ChangeType:
		return signalNotifyContextStopResult(value.X, seen)
	case *ssa.Phi:
		if len(value.Edges) == 0 {
			return false
		}
		for _, edge := range value.Edges {
			if !signalNotifyContextStopResult(edge, seen) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// contextCancelResult recognizes only cancellation functions directly
// extracted from APIs whose contracts guarantee that invocation cancels a
// context. The named function type can also wrap arbitrary application code.
func contextCancelResult(value ssa.Value, seen map[ssa.Value]bool) bool {
	if value == nil || seen[value] {
		return false
	}
	seen[value] = true
	defer delete(seen, value)
	switch value := value.(type) {
	case *ssa.Extract:
		if value.Index != 1 {
			return false
		}
		call, ok := value.Tuple.(*ssa.Call)
		if !ok {
			return false
		}
		callee := call.Common().StaticCallee()
		if callee == nil {
			return false
		}
		fn, ok := callee.Object().(*types.Func)
		if !ok {
			return false
		}
		switch Key(fn) {
		case "context.WithCancel", "context.WithDeadline", "context.WithDeadlineCause", "context.WithTimeout", "context.WithTimeoutCause":
			return contextCancelType(value.Type()) == "CancelFunc"
		case "context.WithCancelCause":
			return contextCancelType(value.Type()) == "CancelCauseFunc"
		}
		return false
	case *ssa.ChangeType:
		return contextCancelResult(value.X, seen)
	case *ssa.Phi:
		if len(value.Edges) == 0 {
			return false
		}
		for _, edge := range value.Edges {
			if !contextCancelResult(edge, seen) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func LookupCall(fn *types.Func, args []ssa.Value) (Set, bool) {
	switch Key(fn) {
	case "os/signal.Notify":
		return signalNotifyEffects(args), true
	case "os/signal.Stop":
		return signalStopEffects(args), true
	case "os/signal.Ignore":
		return signalPolicyEffects(args, ProcessSignalIgnore), true
	case "os/signal.Reset":
		return signalPolicyEffects(args, ProcessSignalReset), true
	case "os/signal.NotifyContext":
		return signalNotifyContextEffects(args), true
	case "context.AfterFunc":
		if len(args) == 0 || staticallyNil(args[0], make(map[ssa.Value]bool)) {
			// A nil context panics before a callback association can be created.
			return 0, true
		}
		return ConcurrencyCallbackRegister | Unknown, true
	case "time.NewTicker", "time.Tick":
		return tickerScheduleEffects(Key(fn), args), true
	case "time.(*Ticker).Reset":
		return tickerResetEffects(args), true
	case "time.Sleep":
		return sleepEffects(args), true
	}
	if Key(fn) == "go/types.TypeString" {
		if len(args) >= 2 {
			if c, ok := args[1].(*ssa.Const); ok && c.IsNil() {
				return 0, true
			}
		}
		return Unknown, true
	}
	if Key(fn) == "fmt.Sprintf" || Key(fn) == "fmt.Sprint" || Key(fn) == "fmt.Sprintln" || Key(fn) == "fmt.Errorf" {
		for _, arg := range args[1:] {
			if !formatValueIsBasic(arg) {
				return Unknown, true // Stringer/Error/Formatter may execute application code.
			}
		}
		return 0, true
	}
	if Key(fn) == "os.OpenFile" {
		if len(args) < 2 {
			return FSRead | FSWrite | FSCreate | ResourceAcquire, true
		}
		if c, ok := args[1].(*ssa.Const); ok && c.Value != nil {
			if flags, ok := constant.Int64Val(c.Value); ok {
				set := ResourceAcquire
				if flags&int64(os.O_WRONLY|os.O_RDWR) != 0 {
					set |= FSWrite
				} else {
					set |= FSRead
				}
				if flags&int64(os.O_CREATE) != 0 {
					set |= FSCreate
				}
				if flags&int64(os.O_TRUNC) != 0 {
					set |= FSWrite
				}
				return set, true
			}
		}
		return FSRead | FSWrite | FSCreate | ResourceAcquire, true
	}
	return Lookup(fn)
}

func positiveDuration(value ssa.Value) (positive, known bool) {
	constantValue, ok := value.(*ssa.Const)
	if !ok || constantValue.Value == nil {
		return false, false
	}
	duration, exact := constant.Int64Val(constantValue.Value)
	return duration > 0 && exact, exact
}

func tickerScheduleEffects(api string, args []ssa.Value) Set {
	if len(args) == 0 {
		return ConcurrencyTimerSchedule | Unknown
	}
	positive, known := positiveDuration(args[0])
	if known {
		if positive {
			return ConcurrencyTimerSchedule
		}
		if api == "time.Tick" {
			// Tick returns nil for a non-positive interval.
			return 0
		}
		// NewTicker panics for a non-positive interval; no ticker is created.
		return Unknown
	}
	return ConcurrencyTimerSchedule | Unknown
}

func tickerResetEffects(args []ssa.Value) Set {
	if len(args) == 0 {
		return ConcurrencyTimerReset | Unknown
	}
	// Method-call SSA may carry the receiver as the first argument; the
	// duration remains the final source argument in either representation.
	positive, known := positiveDuration(args[len(args)-1])
	if known {
		if positive {
			return ConcurrencyTimerReset
		}
		// Ticker.Reset panics for a non-positive interval without resetting it.
		return Unknown
	}
	return ConcurrencyTimerReset | Unknown
}

func sleepEffects(args []ssa.Value) Set {
	if len(args) != 1 {
		return ConcurrencySleep | Unknown
	}
	positive, known := positiveDuration(args[0])
	if known {
		if positive {
			return ConcurrencySleep
		}
		// Sleep returns immediately for a non-positive duration.
		return 0
	}
	return ConcurrencySleep | Unknown
}

func signalPolicyEffects(args []ssa.Value, effect Set) Set {
	values, known := signalArgumentValues(args, 0)
	if !known {
		return effect | Unknown
	}
	if len(values) == 0 {
		// Ignore/Reset with no explicit signal list applies to all signals.
		return effect
	}
	for _, value := range values {
		if classifySignalValue(value, make(map[ssa.Value]bool)) != signalUnsupported {
			return effect | Unknown
		}
	}
	// Concrete signal types not recognized by the target implementation are
	// ignored by Ignore/Reset and do not change signal policy.
	return 0
}

func signalStopEffects(args []ssa.Value) Set {
	if len(args) == 0 || staticallyNil(args[0], make(map[ssa.Value]bool)) {
		// Stop(nil) finds no registration and returns without changing signal state.
		return 0
	}
	// Stop only removes an existing registration. A call site alone does not
	// establish that this channel was previously passed to Notify.
	return ProcessSignalUnregister | Unknown
}

func signalNotifyContextEffects(args []ssa.Value) Set {
	if len(args) == 0 || staticallyNil(args[0], make(map[ssa.Value]bool)) {
		// A nil parent panics while constructing the context, before signal
		// registration or the watcher goroutine can be started.
		return 0
	}
	set := GoroutineStart | Unknown // The watcher starts only if the child context is not already done.
	values, known := signalArgumentValues(args, 1)
	if !known || len(values) == 0 {
		// No signal list means registration for all signals.
		return set | ProcessSignalRegister
	}
	for _, value := range values {
		if classifySignalValue(value, make(map[ssa.Value]bool)) != signalUnsupported {
			return set | ProcessSignalRegister
		}
	}
	return set
}

func signalNotifyEffects(args []ssa.Value) Set {
	if len(args) == 0 {
		return ProcessSignalRegister | Unknown
	}
	if staticallyNil(args[0], make(map[ssa.Value]bool)) {
		// Notify panics on a nil channel before registering it.
		return 0
	}
	channelKnown := staticallyNonNilChannel(args[0], make(map[ssa.Value]bool))
	values, known := signalArgumentValues(args, 1)
	if !known {
		return ProcessSignalRegister | Unknown
	}
	if len(values) == 0 {
		// With no signals, Notify registers the channel for all signals.
		if channelKnown {
			return ProcessSignalRegister
		}
		return ProcessSignalRegister | Unknown
	}
	for _, value := range values {
		if classifySignalValue(value, make(map[ssa.Value]bool)) != signalUnsupported {
			// Explicit signal support is not inferred from interface or global
			// values, so registration remains possible rather than established.
			return ProcessSignalRegister | Unknown
		}
	}
	// The platform implementation ignores values whose concrete type it does
	// not recognize (for example, a custom os.Signal implementation).
	return 0
}

type signalArgumentSupport uint8

const (
	signalUnsupported signalArgumentSupport = iota
	signalUnknown
)

// signalArgumentValues recognizes a single-use slice of a local array, with
// constant bounds and one initialization store per element before the slice.
// Competing uses (including aliases passed to helpers) and other memory flows
// remain unknown. This is a literal-shape proof, not a memory analysis.
func signalArgumentValues(args []ssa.Value, firstArgument int) ([]ssa.Value, bool) {
	if len(args) <= firstArgument {
		return nil, true // No explicit signals means all signals.
	}
	if c, ok := args[firstArgument].(*ssa.Const); ok && c.IsNil() {
		return nil, true // A nil ellipsis slice also means no explicit signals.
	}
	slice, ok := args[firstArgument].(*ssa.Slice)
	if !ok || slice.Referrers() == nil || len(*slice.Referrers()) != 1 {
		return nil, false
	}
	call, ok := (*slice.Referrers())[0].(ssa.CallInstruction)
	if !ok || len(call.Common().Args) != len(args) {
		return nil, false
	}
	for i, arg := range call.Common().Args {
		if arg != args[i] || (i != firstArgument && arg == slice) {
			return nil, false
		}
	}
	block := slice.Block()
	if block == nil || call.Block() != block {
		return nil, false
	}
	alloc, ok := slice.X.(*ssa.Alloc)
	if !ok || alloc.Referrers() == nil || alloc.Block() != block {
		return nil, false
	}
	pointer, ok := types.Unalias(alloc.Type()).Underlying().(*types.Pointer)
	if !ok {
		return nil, false
	}
	array, ok := types.Unalias(pointer.Elem()).Underlying().(*types.Array)
	if !ok || array.Len() > int64(^uint(0)>>1) {
		return nil, false
	}
	bound := func(value ssa.Value, fallback int64) (int64, bool) {
		if value == nil {
			return fallback, true
		}
		c, ok := value.(*ssa.Const)
		if !ok || c.Value == nil {
			return 0, false
		}
		return constant.Int64Val(c.Value)
	}
	low, lowKnown := bound(slice.Low, 0)
	high, highKnown := bound(slice.High, array.Len())
	max, maxKnown := bound(slice.Max, array.Len())
	if !lowKnown || !highKnown || !maxKnown || low < 0 || high < low || max < high || max > array.Len() {
		return nil, false
	}
	if low == high {
		// An empty slice means all signals, regardless of backing-array values.
		return nil, true
	}
	positions := make(map[ssa.Instruction]int, len(block.Instrs))
	for i, instruction := range block.Instrs {
		positions[instruction] = i
	}
	values := make([]ssa.Value, int(array.Len()))
	found := make([]bool, len(values))
	for _, ref := range *alloc.Referrers() {
		if ref == slice {
			continue
		}
		index, ok := ref.(*ssa.IndexAddr)
		if !ok || index.X != alloc || index.Referrers() == nil {
			return nil, false
		}
		constantIndex, ok := index.Index.(*ssa.Const)
		if !ok || constantIndex.Value == nil {
			return nil, false
		}
		position, ok := constant.Int64Val(constantIndex.Value)
		if !ok || position < 0 || position >= int64(len(values)) || found[position] {
			return nil, false
		}
		refs := *index.Referrers()
		if len(refs) != 1 {
			return nil, false
		}
		store, ok := refs[0].(*ssa.Store)
		if !ok || store.Addr != index || store.Block() != block || positions[store] >= positions[slice] {
			return nil, false
		}
		values[position] = store.Val
		found[position] = true
	}
	for _, ok := range found {
		if !ok {
			return nil, false
		}
	}
	return values[low:high], true
}

func classifySignalValue(value ssa.Value, seen map[ssa.Value]bool) signalArgumentSupport {
	if value == nil || seen[value] {
		return signalUnknown
	}
	seen[value] = true
	defer delete(seen, value)
	switch value := value.(type) {
	case *ssa.Const:
		if value.IsNil() {
			return signalUnsupported
		}
	case *ssa.MakeInterface:
		concrete := types.Unalias(value.X.Type())
		if _, ok := concrete.(*types.TypeParam); ok {
			return signalUnknown
		}
		if _, ok := concrete.Underlying().(*types.Interface); ok {
			return signalUnknown
		}
		if named, ok := concrete.(*types.Named); ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "syscall" {
			switch named.Obj().Name() {
			case "Signal", "Note":
				// These are recognized by some platform implementations, but
				// the value's support is platform- and value-dependent.
				return signalUnknown
			}
		}
		return signalUnsupported
	case *ssa.ChangeType:
		return classifySignalValue(value.X, seen)
	case *ssa.ChangeInterface:
		return classifySignalValue(value.X, seen)
	case *ssa.Phi:
		if len(value.Edges) == 0 {
			return signalUnknown
		}
		for _, edge := range value.Edges {
			if classifySignalValue(edge, seen) != signalUnsupported {
				return signalUnknown
			}
		}
		return signalUnsupported
	}
	return signalUnknown
}

func staticallyNil(value ssa.Value, seen map[ssa.Value]bool) bool {
	if value == nil || seen[value] {
		return false
	}
	seen[value] = true
	defer delete(seen, value)
	switch value := value.(type) {
	case *ssa.Const:
		return value.IsNil()
	case *ssa.ChangeType:
		return staticallyNil(value.X, seen)
	case *ssa.Phi:
		if len(value.Edges) == 0 {
			return false
		}
		for _, edge := range value.Edges {
			if !staticallyNil(edge, seen) {
				return false
			}
		}
		return true
	}
	return false
}

func staticallyNonNilChannel(value ssa.Value, seen map[ssa.Value]bool) bool {
	if value == nil || seen[value] {
		return false
	}
	seen[value] = true
	defer delete(seen, value)
	switch value := value.(type) {
	case *ssa.MakeChan:
		return true
	case *ssa.ChangeType:
		return staticallyNonNilChannel(value.X, seen)
	case *ssa.Phi:
		if len(value.Edges) == 0 {
			return false
		}
		for _, edge := range value.Edges {
			if !staticallyNonNilChannel(edge, seen) {
				return false
			}
		}
		return true
	}
	return false
}

func formatValueIsBasic(v ssa.Value) bool {
	if _, ok := v.(*ssa.Const); ok {
		return true
	}
	if v, ok := v.(*ssa.MakeInterface); ok {
		_, ok := v.X.Type().Underlying().(*types.Basic)
		return ok
	}
	if slice, ok := v.(*ssa.Slice); ok {
		return formatSliceIsBasic(slice)
	}
	return false
}

// Go lowers a variadic call such as Sprintf("%d", n) to an allocated [N]any,
// stores, and a slice. Inspect only that local SSA shape; anything else is
// uncertain rather than attempting general memory dataflow.
func formatSliceIsBasic(slice *ssa.Slice) bool {
	alloc, ok := slice.X.(*ssa.Alloc)
	if !ok || alloc.Referrers() == nil {
		return false
	}
	count := 0
	for _, use := range *alloc.Referrers() {
		if use == slice {
			continue
		}
		index, ok := use.(*ssa.IndexAddr)
		if !ok || index.Referrers() == nil {
			return false
		}
		for _, indexedUse := range *index.Referrers() {
			store, ok := indexedUse.(*ssa.Store)
			if !ok || store.Addr != index || !formatValueIsBasic(store.Val) {
				return false
			}
			count++
		}
	}
	return count > 0
}

// These are operation-kind summaries, not resource or permission models.
// OpenFile flags are evaluated when SSA provides a constant; unknown flags keep
// all possible operation kinds. Constructors are separate from operations that start work.
var summaries = buildSummaries()

func buildSummaries() map[string]Set {
	m := make(map[string]Set)
	add := func(pkg string, effect Set, funcs ...string) {
		for _, fn := range funcs {
			m[pkg+"."+fn] = effect
		}
	}
	add("os", FSRead, "ReadFile", "ReadDir", "Stat", "Lstat", "Readlink", "Getwd", "Executable", "Open", "(*File).Read", "(*File).ReadAt", "(*File).ReadDir", "(*File).Readdir", "(*File).Readdirnames", "(*File).Stat")
	add("os", FSWrite, "Truncate", "Chmod", "Chown", "Lchown", "Chtimes", "Chdir", "(*File).Write", "(*File).WriteAt", "(*File).WriteString", "(*File).Truncate", "(*File).Chmod", "(*File).Chown", "(*File).Chdir", "(*File).Sync")
	add("os", FSCreate|FSWrite, "WriteFile", "Create", "CreateTemp")
	add("os", FSCreate, "Mkdir", "MkdirAll", "MkdirTemp", "Link", "Symlink")
	// OpenFile is handled by LookupCall.
	add("os", FSRemove, "Remove", "RemoveAll")
	// Preserve the existing entry-change categories while naming the operation.
	add("os", FSRemove|FSCreate|FSRename, "Rename")
	add("os", FSRead, "OpenRoot", "OpenInRoot", "(*Root).Open", "(*Root).OpenRoot", "(*Root).ReadFile", "(*Root).Stat", "(*Root).Lstat", "(*Root).Readlink", "(*rootFS).Open", "(*rootFS).ReadFile", "(*rootFS).ReadDir", "(*rootFS).Stat", "(*rootFS).Lstat", "(*rootFS).ReadLink")
	add("os", FSWrite, "(*Root).Chmod", "(*Root).Chown", "(*Root).Lchown", "(*Root).Chtimes")
	add("os", FSCreate|FSWrite, "(*Root).Create", "(*Root).WriteFile")
	add("os", FSRead|FSWrite|FSCreate, "(*Root).OpenFile")
	add("os", FSCreate, "(*Root).Mkdir", "(*Root).MkdirAll", "(*Root).Link", "(*Root).Symlink")
	add("os", FSRemove, "(*Root).Remove", "(*Root).RemoveAll")
	add("os", FSRemove|FSCreate|FSRename, "(*Root).Rename")
	add("os", EnvRead, "Getenv", "LookupEnv", "Environ", "ExpandEnv", "UserHomeDir", "UserCacheDir", "UserConfigDir", "TempDir")
	add("os", EnvWrite, "Setenv", "Unsetenv", "Clearenv")
	add("os", ProcessExec, "StartProcess")
	add("os", ProcessExit, "Exit")
	add("os", ProcessControl, "(*Process).Kill", "(*Process).Signal")
	add("os/signal", ProcessSignalRegister, "Notify")
	add("os/signal", ProcessSignalUnregister|Unknown, "Stop")
	add("log", ProcessExit, "Fatal", "Fatalln", "Fatalf", "(*Logger).Fatal", "(*Logger).Fatalln", "(*Logger).Fatalf")
	add("os", ResourceRelease, "(*File).Close", "(*Root).Close", "(*Process).Release", "(*Process).Wait")
	add("os", 0, "Pipe")
	add("path/filepath", FSRead, "Abs", "EvalSymlinks", "Glob")
	add("io/ioutil", FSRead, "ReadFile", "ReadDir")
	add("io/ioutil", FSCreate|FSWrite, "WriteFile", "TempFile")
	add("io/ioutil", FSCreate, "TempDir")
	add("os/exec", 0, "(*Cmd).String")
	add("os/exec", FSRead|EnvRead, "Command", "LookPath")
	add("os/exec", FSRead|EnvRead|Unknown, "CommandContext")
	add("os/exec", ProcessExec, "(*Cmd).Start", "(*Cmd).Run", "(*Cmd).Output", "(*Cmd).CombinedOutput")
	add("os/exec", Unknown, "(*Cmd).Wait")
	add("net", NetConnect, "Dial", "DialTimeout", "(*Dialer).Dial", "(*Dialer).DialContext", "DialTCP", "DialUDP", "DialIP", "DialUnix")
	add("net", NetListen, "Listen", "ListenPacket", "ListenTCP", "ListenUDP", "ListenIP", "ListenUnix", "ListenUnixgram", "(*ListenConfig).Listen", "(*ListenConfig).ListenPacket", "(*TCPListener).Accept", "(*TCPListener).AcceptTCP", "(*UnixListener).Accept", "(*UnixListener).AcceptUnix")
	add("crypto/tls", NetConnect, "Dial", "DialWithDialer", "(*Dialer).Dial", "(*Dialer).DialContext")
	add("crypto/tls", NetListen, "Listen", "(*listener).Accept")
	// HTTP can invoke application hooks/transports/handlers inside this boundary.
	add("net/http", NetConnect|Unknown, "Get", "Head", "Post", "PostForm", "(*Client).Do", "(*Client).Get", "(*Client).Head", "(*Client).Post", "(*Client).PostForm", "(*Transport).RoundTrip")
	add("net/http", NetListen|Unknown, "ListenAndServe", "ListenAndServeTLS", "Serve", "ServeTLS", "(*Server).ListenAndServe", "(*Server).ListenAndServeTLS", "(*Server).Serve", "(*Server).ServeTLS")
	add("net/http", FSRead|Unknown, "ServeFile", "ServeFileFS")
	add("net/http", Unknown, "ServeContent")
	add("database/sql", Unknown, "Open")
	add("database/sql", 0, "OpenDB", "(*DB).Driver", "Named", "(*Row).Err")
	for _, recv := range []string{"(*DB)", "(*Tx)", "(*Conn)", "(*Stmt)"} {
		for _, method := range []string{"Query", "QueryContext", "QueryRow", "QueryRowContext"} {
			add("database/sql", DBQuery, recv+"."+method)
		}
		for _, method := range []string{"Exec", "ExecContext"} {
			add("database/sql", DBExec, recv+"."+method)
		}
	}
	add("database/sql", DBExec, "(*DB).Begin", "(*DB).BeginTx", "(*Conn).BeginTx", "(*Tx).Commit", "(*Tx).Rollback", "(*DB).Prepare", "(*DB).PrepareContext", "(*Tx).Prepare", "(*Tx).PrepareContext", "(*Conn).PrepareContext")
	add("database/sql", Unknown, "(*DB).Ping", "(*DB).PingContext", "(*DB).Conn")
	add("reflect", 0, "(Value).Method", "(Value).MethodByName")
	add("reflect", Unknown, "(Value).Call", "(Value).CallSlice")
	// In-memory standard-library operations are summary boundaries, preventing
	// their internal io.Writer plumbing from being mistaken for file writes.
	add("fmt", 0, "Appendf", "Append", "Appendln")
	add("go/types", 0, "ObjectString", "SelectionString", "Id", "NewContext")
	add("bytes", 0, "(*Buffer).Write", "(*Buffer).WriteString", "(*Buffer).WriteByte", "(*Buffer).WriteRune", "(*Buffer).Read")
	add("errors", 0, "New")
	add("sync", ConcurrencyLock, "(*Mutex).Lock", "(*RWMutex).Lock", "(*RWMutex).RLock")
	add("sync", ConcurrencyUnlock, "(*Mutex).Unlock", "(*RWMutex).Unlock", "(*RWMutex).RUnlock")
	add("sync", ConcurrencyTaskAccounting, "(*WaitGroup).Add", "(*WaitGroup).Done")
	add("sync", ConcurrencyWait, "(*WaitGroup).Wait")
	// Cond.Wait waits while releasing and reacquiring its Locker. The concrete
	// Locker methods use unresolved interface dispatch, retained alongside known effects.
	add("sync", ConcurrencyWait|ConcurrencyLock|ConcurrencyUnlock|Unknown, "(*Cond).Wait")
	// Signal and Broadcast identify one/all-waiter notification attempts; the
	// current waiter set is not known at the call site.
	add("sync", ConcurrencyConditionSignal|Unknown, "(*Cond).Signal")
	add("sync", ConcurrencyConditionBroadcast|Unknown, "(*Cond).Broadcast")
	// Pool.Get may invoke the configured New callback; the pool state and
	// callback value are intentionally not inferred here.
	add("sync", ConcurrencyPoolGet|Unknown, "(*Pool).Get")
	add("sync", ConcurrencyPoolPut, "(*Pool).Put")
	// NewCertPool creates an empty in-memory value. Adding certificates is an
	// operation on that value, not proof it is used as trust configuration.
	add("crypto/x509", 0, "NewCertPool")
	// SystemCertPool requests a copy of the platform system roots. Its result
	// does not prove which roots were loaded or that the pool is later used.
	add("crypto/x509", CertPoolSystem, "SystemCertPool")
	add("crypto/x509", ConcurrencyCertPoolAdd, "(*CertPool).AddCert")
	// PEM contents determine whether any certificate is appended. A constraint
	// callback may run later during certificate verification.
	add("crypto/x509", ConcurrencyCertPoolAdd|Unknown, "(*CertPool).AppendCertsFromPEM", "(*CertPool).AddCertWithConstraint")
	// These exact sync.Map APIs identify access/update intent, not the formal
	// memory-model read/write classification, sharedness, or observed ordering.
	add("sync", ConcurrencyMapRead, "(*Map).Load")
	add("sync", ConcurrencyMapWrite, "(*Map).Store", "(*Map).Delete", "(*Map).Clear")
	add("sync", ConcurrencyMapRead|ConcurrencyMapWrite, "(*Map).Swap")
	// Conditional mutations retain state uncertainty; Range invokes an opaque
	// callback and does not provide a consistent snapshot.
	add("sync", ConcurrencyMapRead|ConcurrencyMapWrite|Unknown, "(*Map).LoadAndDelete", "(*Map).LoadOrStore", "(*Map).CompareAndSwap", "(*Map).CompareAndDelete")
	add("sync", ConcurrencyMapRead|Unknown, "(*Map).Range")
	// Once.Do waits for its one callback to finish before any Do call returns;
	// the callback body remains opaque at this summary boundary. OnceFunc,
	// OnceValue and OnceValues only create wrappers; invocation is recognized
	// separately when local SSA proves the wrapper's constructor provenance.
	add("sync", ConcurrencyWait|Unknown, "(*Once).Do")
	add("sync", 0, "OnceFunc", "OnceValue", "OnceValues")
	// WaitGroup.Go starts a task, updates the group counter, and invokes a
	// callback whose body is outside this summary boundary. Keep that opacity.
	add("sync", ConcurrencyTaskAccounting|GoroutineStart|Unknown, "(*WaitGroup).Go")
	// Timer constructors schedule channel notifications; AfterFunc schedules
	// an opaque callback asynchronously. Timer.Stop/Reset retain state
	// uncertainty, while Ticker.Stop directly disables future ticks.
	add("time", ConcurrencyTimerSchedule, "After", "NewTimer")
	add("time", ConcurrencyCallbackRegister|Unknown, "AfterFunc")
	add("time", ConcurrencyTimerStop|Unknown, "(*Timer).Stop")
	add("time", ConcurrencyTimerReset|Unknown, "(*Timer).Reset")
	add("time", ConcurrencyTimerStop, "(*Ticker).Stop")
	// Exact sync/atomic APIs are atomic memory operations. They do not establish
	// that the accessed value is shared or that a larger protocol is correct.
	for _, typ := range []string{"Int32", "Int64", "Uint32", "Uint64", "Uintptr"} {
		for _, op := range []string{"Add", "And", "Or", "Load", "Store", "Swap", "CompareAndSwap"} {
			add("sync/atomic", ConcurrencyAtomic, op+typ)
		}
	}
	for _, op := range []string{"Load", "Store", "Swap", "CompareAndSwap"} {
		add("sync/atomic", ConcurrencyAtomic, op+"Pointer")
	}
	for _, typ := range []string{"Bool", "Pointer[T]", "Value"} {
		for _, op := range []string{"Load", "Store", "Swap", "CompareAndSwap"} {
			add("sync/atomic", ConcurrencyAtomic, "(*"+typ+")."+op)
		}
	}
	for _, typ := range []string{"Int32", "Int64", "Uint32", "Uint64", "Uintptr"} {
		for _, op := range []string{"Load", "Store", "Swap", "CompareAndSwap", "Add", "And", "Or"} {
			add("sync/atomic", ConcurrencyAtomic, "(*"+typ+")."+op)
		}
	}
	// Resource ownership is separate from filesystem mutation. These operations
	// return owned handles when successful; this is not leak/lifetime analysis.
	for _, key := range []string{
		"os.Open", "os.Create", "os.CreateTemp", "os.OpenRoot", "os.OpenInRoot", "os.Pipe",
		"os.(*Root).Open", "os.(*Root).OpenRoot", "os.(*Root).Create", "os.(*Root).OpenFile",
		"io/ioutil.TempFile", "database/sql.Open", "database/sql.OpenDB", "database/sql.(*DB).Conn",
	} {
		m[key] |= ResourceAcquire
	}
	for key := range m {
		if (strings.HasPrefix(key, "net.") || strings.HasPrefix(key, "crypto/tls.")) && m[key]&(NetConnect|NetListen) != 0 {
			m[key] |= ResourceAcquire
		}
	}
	add("database/sql", ResourceRelease, "(*DB).Close", "(*Conn).Close", "(*Stmt).Close", "(*Rows).Close")
	for _, recv := range []string{"(*conn)", "(*TCPConn)", "(*UDPConn)", "(*IPConn)", "(*UnixConn)", "(*TCPListener)", "(*UnixListener)"} {
		add("net", NetClose|ResourceRelease, recv+".Close")
	}
	// Half-close changes connection state but does not release the whole handle.
	for _, recv := range []string{"(*TCPConn)", "(*UnixConn)"} {
		add("net", NetClose, recv+".CloseRead", recv+".CloseWrite")
	}
	add("crypto/tls", NetClose|ResourceRelease|Unknown, "(*Conn).Close")
	add("crypto/tls", NetClose|Unknown, "(*Conn).CloseWrite")
	add("net/http", NetClose|ResourceRelease|Unknown, "(*Server).Close", "(*Server).Shutdown")
	add("net/http", NetClose|Unknown, "(*Transport).CloseIdleConnections", "(*Client).CloseIdleConnections")
	return m
}
