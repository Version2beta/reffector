package analyzer

import (
	"go/types"
	"sort"

	"golang.org/x/tools/go/ssa"
)

// Function identity is not a closure environment. In particular, FreeVar and
// memory loads stay open even when the enclosing closure's identity is known.
type functionProvenance struct {
	targets []*ssa.Function
	params  []*ssa.Parameter
	open    bool
	nil     bool
}

type functionResult struct {
	fn    *ssa.Function
	index int
}

// Each root query has its own bounded memo table. No incomplete recursive
// summary is reused by another query, so discovery order cannot close a cycle.
type functionQuery struct {
	g        *graph
	values   map[ssa.Value]functionProvenance
	returns  map[functionResult]functionProvenance
	active   map[functionResult]bool
	visiting map[ssa.Value]bool
	steps    int
}

const functionValueSteps = 4096
const functionValueDepth = 64

func mergeFunctionProvenance(a, b functionProvenance) functionProvenance {
	out := functionProvenance{open: a.open || b.open, nil: a.nil || b.nil}
	for _, p := range []functionProvenance{a, b} {
		for _, target := range p.targets {
			found := false
			for _, old := range out.targets {
				found = found || old == target
			}
			if !found {
				out.targets = append(out.targets, target)
			}
		}
		for _, param := range p.params {
			found := false
			for _, old := range out.params {
				found = found || old == param
			}
			if !found {
				out.params = append(out.params, param)
			}
		}
	}
	return out
}

func (q *functionQuery) value(v ssa.Value, depth int) functionProvenance {
	if depth >= functionValueDepth || q.steps >= functionValueSteps || v == nil {
		return functionProvenance{open: true}
	}
	q.steps++
	if p, ok := q.values[v]; ok {
		return p
	}
	if q.visiting[v] {
		return functionProvenance{open: true}
	}
	q.visiting[v] = true
	var out functionProvenance
	switch v := v.(type) {
	case *ssa.Function:
		out.targets = []*ssa.Function{v}
	case *ssa.MakeClosure:
		out = q.value(v.Fn, depth+1)
	case *ssa.Parameter:
		out.params = []*ssa.Parameter{v}
	case *ssa.ChangeType:
		out = q.value(v.X, depth+1)
	case *ssa.Phi:
		for _, edge := range v.Edges {
			out = mergeFunctionProvenance(out, q.value(edge, depth+1))
		}
	case *ssa.Call:
		out = q.call(v, 0, depth+1)
	case *ssa.Extract:
		if call, ok := v.Tuple.(*ssa.Call); ok {
			out = q.call(call, v.Index, depth+1)
		} else {
			out.open = true
		}
	case *ssa.Const:
		out.nil, out.open = v.IsNil(), !v.IsNil()
	default:
		out.open = true
	}
	delete(q.visiting, v)
	q.values[v] = out
	return out
}

func (q *functionQuery) call(call *ssa.Call, index, depth int) functionProvenance {
	fn := call.Common().StaticCallee()
	if fn == nil || q.g.isStandardLibrary(fn) || len(fn.Blocks) == 0 || index >= fn.Signature.Results().Len() {
		return functionProvenance{open: true}
	}
	// Summarized APIs are opaque boundaries, even if source happens to be loaded.
	if _, ok := summary(fn); ok {
		return functionProvenance{open: true}
	}
	if _, ok := types.Unalias(fn.Signature.Results().At(index).Type()).Underlying().(*types.Signature); !ok {
		return functionProvenance{open: true}
	}
	p := q.result(functionResult{fn, index}, depth+1)
	out := functionProvenance{targets: p.targets, open: p.open, nil: p.nil}
	for _, param := range p.params {
		matched := false
		for i, formal := range fn.Params {
			if param == formal && i < len(call.Common().Args) {
				out = mergeFunctionProvenance(out, q.value(call.Common().Args[i], depth+1))
				matched = true
				break
			}
		}
		if !matched {
			out.open = true
		}
	}
	return out
}

func (q *functionQuery) result(key functionResult, depth int) functionProvenance {
	if depth >= functionValueDepth || q.steps >= functionValueSteps || q.active[key] {
		return functionProvenance{open: true}
	}
	q.steps++
	if p, ok := q.returns[key]; ok {
		return p
	}
	q.active[key] = true
	var out functionProvenance
	found := false
	for _, block := range key.fn.Blocks {
		for _, instruction := range block.Instrs {
			if q.steps >= functionValueSteps {
				out.open = true
				break
			}
			q.steps++
			if ret, ok := instruction.(*ssa.Return); ok && key.index < len(ret.Results) {
				out = mergeFunctionProvenance(out, q.value(ret.Results[key.index], depth+1))
				found = true
			}
		}
	}
	out.open = out.open || !found
	delete(q.active, key)
	q.returns[key] = out
	return out
}

func (g *graph) functionValue(v ssa.Value) functionProvenance {
	if g.functionValues == nil {
		g.functionValues = make(map[ssa.Value]functionProvenance)
	}
	if p, ok := g.functionValues[v]; ok {
		return p
	}
	q := functionQuery{g: g, values: make(map[ssa.Value]functionProvenance), returns: make(map[functionResult]functionProvenance), active: make(map[functionResult]bool), visiting: make(map[ssa.Value]bool)}
	p := q.value(v, 0)
	p.open = p.open || len(p.params) > 0
	sort.Slice(p.targets, func(i, j int) bool { return g.variant(p.targets[i]) < g.variant(p.targets[j]) })
	g.functionValues[v] = p
	return p
}

// Only a closed, non-nil singleton establishes an invocation target. Finite
// alternatives are candidate_function_value evidence, not VTA guesses.
func (g *graph) functionTargets(call ssa.CallInstruction) ([]*ssa.Function, bool) {
	p := g.functionValue(call.Common().Value)
	if parameter, ok := call.Common().Value.(*ssa.Parameter); ok {
		// Actuals observed at static callsites are useful candidates, but the
		// graph cannot prove that all callers of this formal are known.
		if candidates, found := g.functionParamCandidates[parameter]; found {
			p = mergeFunctionProvenance(p, candidates)
			p.open = true
		}
	}
	sort.Slice(p.targets, func(i, j int) bool { return g.variant(p.targets[i]) < g.variant(p.targets[j]) })
	if !p.open && !p.nil && len(p.targets) == 1 {
		return p.targets, false
	}
	if g.functionCandidates == nil {
		g.functionCandidates = make(map[ssa.CallInstruction][]*ssa.Function)
	}
	g.functionCandidates[call] = p.targets
	return nil, p.open
}

// collectFunctionArguments gathers formal-parameter candidates before build
// resolves any indirect calls, so candidate evidence is independent of traversal order.
func (g *graph) collectFunctionArguments(roots map[*ssa.Function]bool) {
	pending := make([]*ssa.Function, 0, len(roots))
	for fn := range roots {
		pending = append(pending, fn)
	}
	seen := make(map[*ssa.Function]bool)
	for len(pending) > 0 {
		fn := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if fn == nil || seen[fn] || len(fn.Blocks) == 0 || g.isStandardLibrary(fn) {
			continue
		}
		seen[fn] = true
		if _, summarized := summary(fn); summarized {
			continue
		}
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}
				target := call.Common().StaticCallee()
				if target == nil || !roots[target] {
					continue
				}
				g.recordFunctionArguments(target, call.Common())
				pending = append(pending, target)
			}
		}
	}
}

func (g *graph) recordFunctionArguments(target *ssa.Function, call *ssa.CallCommon) {
	if target == nil || call.StaticCallee() != target || g.isStandardLibrary(target) {
		return
	}
	for i, parameter := range target.Params {
		if i >= len(call.Args) {
			break
		}
		if _, ok := types.Unalias(parameter.Type()).Underlying().(*types.Signature); !ok {
			continue
		}
		actual := g.functionValue(call.Args[i])
		if g.functionParamCandidates == nil {
			g.functionParamCandidates = make(map[*ssa.Parameter]functionProvenance)
		}
		g.functionParamCandidates[parameter] = mergeFunctionProvenance(g.functionParamCandidates[parameter], actual)
	}
}

func (g *graph) callCandidates(call ssa.CallInstruction) []*ssa.Function {
	out := append([]*ssa.Function(nil), g.functionCandidates[call]...)
	if !call.Common().IsInvoke() && call.Common().StaticCallee() == nil && !g.functionValue(call.Common().Value).open {
		return out
	}
	for _, fn := range g.candidates[call] {
		if !containsExactFunction(out, fn) {
			out = append(out, fn)
		}
	}
	return out
}

func containsExactFunction(functions []*ssa.Function, target *ssa.Function) bool {
	for _, fn := range functions {
		if fn == target {
			return true
		}
	}
	return false
}
