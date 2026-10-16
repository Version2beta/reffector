// Package analyzer computes possible effects without executing analyzed code.
package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"reffector/internal/effects"
)

type Options struct {
	Dir    string // Empty means the current directory.
	Tests  bool
	Direct bool
}

type Record struct {
	Package  string `json:"package"`
	Function string `json:"function"`
	Effect   string `json:"effect"`
	Direct   bool   `json:"direct"`
}

// Analyze uses the current build environment (GOOS, GOARCH, build constraints).
// Loading may invoke the Go toolchain, but never application code or tests.
func Analyze(patterns []string, opts Options) ([]Record, error) {
	if len(patterns) == 0 {
		patterns = []string{"."}
	}
	cfg, cleanup, err := loadConfig(opts)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	var diagnostics []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			diagnostics = append(diagnostics, e.Error())
		}
	})
	if len(diagnostics) != 0 {
		sort.Strings(diagnostics)
		return nil, fmt.Errorf("package loading failed:\n%s", strings.Join(diagnostics, "\n"))
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no packages matched")
	}
	prog, _ := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	g := &graph{prog: prog, nodes: make(map[*ssa.Function]*node), invokeCache: make(map[invokeKey][]*ssa.Function)}
	// Collect all declared concrete types, not just types instantiated by main.
	// This is a library-friendly class hierarchy analysis, not points-to analysis.
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.TypesInfo == nil {
			return
		}
		for _, obj := range p.TypesInfo.Defs {
			if tn, ok := obj.(*types.TypeName); ok && !tn.IsAlias() {
				if t, ok := tn.Type().(*types.Named); ok && t.TypeParams().Len() == 0 && !types.IsInterface(t) {
					g.concrete = append(g.concrete, t, types.NewPointer(t))
				}
			}
		}
	})
	g.concrete = append(g.concrete, prog.RuntimeTypes()...)

	type identity struct{ pkg, name string }
	selected := make(map[*ssa.Function]identity)
	var selectFunction func(*ssa.Function, identity)
	selectFunction = func(fn *ssa.Function, id identity) {
		if fn == nil {
			return
		}
		selected[fn] = id
		for _, child := range fn.AnonFuncs {
			selectFunction(child, identity{id.pkg, id.name + strings.TrimPrefix(child.Name(), fn.Name())})
		}
	}
	for _, p := range pkgs {
		// go/packages also returns a generated test executable. Never display it.
		if opts.Tests && p.Name == "main" && strings.HasSuffix(p.PkgPath, ".test") && p.Imports["testing/internal/testdeps"] != nil {
			continue
		}
		for _, file := range p.Syntax {
			for _, decl := range file.Decls {
				if decl, ok := decl.(*ast.FuncDecl); ok {
					obj, _ := p.TypesInfo.Defs[decl.Name].(*types.Func)
					fn := prog.FuncValue(obj)
					if fn != nil {
						name := localName(obj)
						if obj.Name() == "init" {
							name = fn.Name() // SSA numbers multiple init declarations.
						}
						selectFunction(fn, identity{p.PkgPath, name})
					}
				}
			}
		}
		if sp := prog.Package(p.Types); sp != nil {
			selectFunction(sp.Func("init"), identity{p.PkgPath, "init"})
		}
	}
	for fn := range selected {
		g.ensure(fn)
	}
	g.build()
	g.propagate()

	// Test variants and generic instantiations may share a source identity.
	// Merge before emitting so each function/effect pair appears exactly once.
	type sets struct{ direct, reachable effects.Set }
	merged := make(map[identity]sets)
	for fn, id := range selected {
		n := g.nodes[fn]
		s := merged[id]
		s.direct |= n.direct
		s.reachable |= n.reachable
		merged[id] = s
	}
	records := make([]Record, 0)
	for id, s := range merged {
		set := s.reachable
		if opts.Direct {
			set = s.direct
		}
		direct := make(map[string]bool)
		for _, name := range s.direct.Names() {
			direct[name] = true
		}
		for _, name := range set.Names() {
			records = append(records, Record{id.pkg, id.name, name, direct[name]})
		}
	}
	sort.Slice(records, func(i, j int) bool {
		a, b := records[i], records[j]
		if a.Package != b.Package {
			return a.Package < b.Package
		}
		if a.Function != b.Function {
			return a.Function < b.Function
		}
		return a.Effect < b.Effect
	})
	return records, nil
}

func localName(obj *types.Func) string {
	key := effects.Key(obj)
	return strings.TrimPrefix(key, obj.Pkg().Path()+".")
}

type node struct {
	fn                *ssa.Function
	direct, reachable effects.Set
	callers           map[*node]bool
}

type invokeKey struct {
	iface  *types.Interface
	method *types.Func
}

type graph struct {
	prog        *ssa.Program
	nodes       map[*ssa.Function]*node
	pending     []*node
	concrete    []types.Type
	invokeCache map[invokeKey][]*ssa.Function
}

func (g *graph) ensure(fn *ssa.Function) *node {
	if n := g.nodes[fn]; n != nil {
		return n
	}
	n := &node{fn: fn, callers: make(map[*node]bool)}
	g.nodes[fn] = n
	g.pending = append(g.pending, n)
	return n
}

func summary(fn *ssa.Function) (effects.Set, bool) {
	obj, _ := fn.Object().(*types.Func)
	return effects.Lookup(obj)
}

func (g *graph) build() {
	for len(g.pending) > 0 {
		n := g.pending[len(g.pending)-1]
		g.pending = g.pending[:len(g.pending)-1]
		if s, ok := summary(n.fn); ok {
			n.direct |= s
			continue
		}
		if len(n.fn.Blocks) == 0 {
			n.direct |= effects.Unknown // assembly, cgo, or missing implementation
		}
		for _, b := range n.fn.Blocks {
			for _, instr := range b.Instrs {
				site, ok := instr.(ssa.CallInstruction)
				if !ok {
					continue
				}
				call := site.Common()
				if builtin, ok := call.Value.(*ssa.Builtin); ok {
					if builtin.Name() == "print" || builtin.Name() == "println" {
						n.direct |= effects.Unknown // Output outside the v1 vocabulary.
					}
					continue
				}
				var targets []*ssa.Function
				var unknown bool
				if fn := call.StaticCallee(); fn != nil {
					targets = []*ssa.Function{fn}
				} else if call.IsInvoke() {
					targets, unknown = g.invoke(call)
				} else {
					targets, unknown = functionTargets(call.Value, make(map[ssa.Value]bool))
				}
				if unknown {
					n.direct |= effects.Unknown
				}
				for _, target := range targets {
					if s, ok := summary(target); ok {
						n.direct |= s
					} else if len(target.Blocks) == 0 {
						n.direct |= effects.Unknown
					} else {
						g.ensure(target).callers[n] = true
					}
				}
			}
		}
	}
}

// functionTargets handles SSA-known local values, including closures and phi
// nodes. Memory/parameter/result flows are deliberately unknown, not guessed.
func functionTargets(v ssa.Value, seen map[ssa.Value]bool) ([]*ssa.Function, bool) {
	if seen[v] {
		return nil, false
	}
	seen[v] = true
	switch v := v.(type) {
	case *ssa.Function:
		return []*ssa.Function{v}, false
	case *ssa.MakeClosure:
		return functionTargets(v.Fn, seen)
	case *ssa.Phi:
		var result []*ssa.Function
		unknown := false
		for _, edge := range v.Edges {
			fns, u := functionTargets(edge, seen)
			result, unknown = append(result, fns...), unknown || u
		}
		return result, unknown
	case *ssa.ChangeType:
		return functionTargets(v.X, seen)
	case *ssa.Const:
		return nil, !v.IsNil() // Calling nil only panics; it has no callee.
	default:
		return nil, true
	}
}

// concreteValues resolves finite interface assignments. An open receiver still
// gets CHA targets, plus unknown.call for implementations outside the load set.
func concreteValues(v ssa.Value, seen map[ssa.Value]bool) ([]types.Type, bool) {
	if seen[v] {
		return nil, false
	}
	seen[v] = true
	switch v := v.(type) {
	case *ssa.MakeInterface:
		return []types.Type{v.X.Type()}, false
	case *ssa.ChangeInterface:
		return concreteValues(v.X, seen)
	case *ssa.Phi:
		var result []types.Type
		unknown := false
		for _, edge := range v.Edges {
			ts, u := concreteValues(edge, seen)
			result, unknown = append(result, ts...), unknown || u
		}
		return result, unknown
	case *ssa.Const:
		return nil, !v.IsNil()
	default:
		return nil, true
	}
}

func (g *graph) invoke(call *ssa.CallCommon) ([]*ssa.Function, bool) {
	concrete, open := concreteValues(call.Value, make(map[ssa.Value]bool))
	iface := call.Value.Type().Underlying().(*types.Interface)
	key := invokeKey{iface, call.Method}
	if open {
		if cached, ok := g.invokeCache[key]; ok {
			return cached, true
		}
		concrete = append(append([]types.Type(nil), concrete...), g.concrete...)
	}
	var targets []*ssa.Function
	seen := make(map[*ssa.Function]bool)
	for _, t := range concrete {
		if types.IsInterface(t) || !types.Implements(t, iface) {
			continue
		}
		sel := g.prog.MethodSets.MethodSet(t).Lookup(call.Method.Pkg(), call.Method.Name())
		if sel != nil {
			if fn := g.prog.MethodValue(sel); fn != nil && !seen[fn] {
				seen[fn] = true
				targets = append(targets, fn)
			}
		}
	}
	if open {
		g.invokeCache[key] = targets
	}
	return targets, open || (len(concrete) > 0 && len(targets) == 0)
}

// Monotone reverse worklist: each node can gain at most twelve effect bits.
// Cycles therefore terminate without recursion or traversal-order dependence.
func (g *graph) propagate() {
	queue := make([]*node, 0, len(g.nodes))
	queued := make(map[*node]bool)
	for _, n := range g.nodes {
		n.reachable = n.direct
		queue = append(queue, n)
		queued[n] = true
	}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		queued[n] = false
		for caller := range n.callers {
			if next := caller.reachable | n.reachable; next != caller.reachable {
				caller.reachable = next
				if !queued[caller] {
					queued[caller] = true
					queue = append(queue, caller)
				}
			}
		}
	}
}
