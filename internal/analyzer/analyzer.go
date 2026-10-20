// Package analyzer computes possible effects without executing analyzed code.
package analyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/Version2beta/reffector/internal/effects"
)

type Options struct {
	Dir                                    string // Empty means the current directory.
	Tests                                  bool
	Direct                                 bool
	Exact                                  bool // Do not expand local directories to recursive patterns.
	All                                    bool // Include source functions without effects or uncertainty.
	Details                                bool
	EffectView                             bool
	Unresolved                             bool   // Only unresolved leads without recognized effects.
	API, Site                              string // Exact identities.
	Reason                                 string // Uncertainty-reason substring in effect view.
	Package, Function, Effect, Operation   string // Substring filters; Function accepts =name for exact matching.
	MaxDepth, MaxPaths, MaxSites, MaxEdges int
}

// Stats counts functions, not effect pairs. Direct and inherited can overlap.
// Counts describe the selected inventory before output filtering.
type Stats struct {
	Packages        int `json:"packages_selected"`
	SourceFunctions int `json:"source_functions"`
	Emitted         int `json:"emitted"`
	Omitted         int `json:"omitted"`
	Direct          int `json:"direct"`
	Inherited       int `json:"inherited"`
	Uncertain       int `json:"uncertain"`
	Effectful       int `json:"effectful"`
	UnresolvedOnly  int `json:"unresolved_only"`
	Uncharacterized int `json:"no_effects_no_uncertainty"`
}

type Report struct {
	Records []Record      `json:"records"`
	Stats   Stats         `json:"stats"`
	Details *Details      `json:"details,omitempty"`
	Effects *EffectReport `json:"-"`
}

type identity struct{ pkg, name string }

// Record is one function's analysis result. Effects are omitted from text
// output when empty, but Uncertain remains available in JSON.
type Record struct {
	Package   string   `json:"package"`
	Function  string   `json:"function"`
	Effects   []Effect `json:"effects"`
	Uncertain bool     `json:"uncertain"`
}

type Effect struct {
	Identifier string `json:"identifier"`
	Direct     bool   `json:"direct"`
}

// Analyze uses the current build environment (GOOS, GOARCH, build constraints).
// Loading may invoke the Go toolchain, but never application code or tests.
func Analyze(patterns []string, opts Options) ([]Record, error) {
	r, err := AnalyzeReport(patterns, opts)
	return r.Records, err
}

// SelectionPatterns preserves explicit Go patterns/import paths, while local
// directory arguments select their subtree unless Exact was requested.
func SelectionPatterns(patterns []string, opts Options) []string {
	if len(patterns) == 0 {
		patterns = []string{"."}
	}
	result := make([]string, 0, len(patterns))
	for _, pattern := range patterns {
		path := pattern
		if !filepath.IsAbs(path) {
			path = filepath.Join(opts.Dir, path)
		}
		if info, err := os.Stat(path); !strings.Contains(pattern, "...") && err == nil && info.IsDir() {
			pattern = filepath.ToSlash(filepath.Clean(pattern))
			if !filepath.IsAbs(pattern) && pattern != "." && pattern != ".." && !strings.HasPrefix(pattern, "../") {
				pattern = "./" + pattern
			}
			if !opts.Exact {
				pattern += "/..."
			}
		}
		result = append(result, pattern)
	}
	return result
}

func AnalyzeReport(patterns []string, opts Options) (Report, error) {
	patterns = SelectionPatterns(patterns, opts)
	cfg, cleanup, err := loadConfig(opts)
	if err != nil {
		return Report{}, err
	}
	defer cleanup()
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return Report{}, err
	}
	var diagnostics []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			diagnostics = append(diagnostics, e.Error())
		}
	})
	if len(diagnostics) != 0 {
		sort.Strings(diagnostics)
		return Report{}, fmt.Errorf("package loading failed:\n%s", strings.Join(diagnostics, "\n"))
	}
	if len(pkgs) == 0 {
		return Report{}, fmt.Errorf("no packages matched")
	}
	prog, _ := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	g := &graph{prog: prog, nodes: make(map[*ssa.Function]*node), standard: make(map[*types.Package]bool),
		detailed: opts.Details || opts.EffectView, packageIDs: make(map[*types.Package]string), sites: make(map[string]*Site), edges: make(map[string]CallEdge), fileKeys: make(map[string]string), siteNodes: make(map[string]map[*node]bool), candidates: make(map[ssa.CallInstruction][]*ssa.Function)}

	// Module metadata, rather than import-path spelling, identifies standard
	// packages. A loaded package with a module (including module "reffector")
	// is application or third-party code even when its path contains no dot.
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.Types != nil {
			g.packageIDs[p.Types] = p.ID
		}
		if p.Types != nil && p.Module == nil {
			g.standard[p.Types] = true
		}
		for _, file := range p.CompiledGoFiles {
			g.fileKeys[filepath.ToSlash(file)] = p.PkgPath + "/" + filepath.Base(file)
		}
	})

	selected := make(map[*ssa.Function]identity)
	g.selected = selected
	var selectFunction func(*ssa.Function, identity)
	selectFunction = func(fn *ssa.Function, id identity) {
		if fn == nil {
			return
		}
		if sourceFunction(fn) {
			selected[fn] = id
		}
		for _, child := range fn.AnonFuncs {
			selectFunction(child, identity{id.pkg, id.name + strings.TrimPrefix(child.Name(), fn.Name())})
		}
	}
	selectedPackages := make(map[string]bool)
	for _, p := range pkgs {
		// go/packages also returns a generated test executable. Never display it.
		if opts.Tests && p.Name == "main" && strings.HasSuffix(p.PkgPath, ".test") && p.Imports["testing/internal/testdeps"] != nil {
			continue
		}
		selectedPackages[p.PkgPath] = true
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
			// Keep source literals in package variable initializers, but not the
			// synthetic package init (which also calls dependency initializers).
			if init := sp.Func("init"); init != nil {
				for _, child := range init.AnonFuncs {
					selectFunction(child, identity{p.PkgPath, child.Name()})
				}
			}
		}
	}
	g.selectedPackages = selectedPackages
	for fn := range selected {
		g.ensure(fn)
	}
	g.resolveCandidates(selected)
	g.build()
	g.propagate()

	// Test variants and generic instantiations may share a source identity.
	// Merge before emitting so each function/effect pair appears exactly once.
	type sets struct {
		direct, reachable, inherited                   effects.Set
		uncertain, directUncertain, inheritedUncertain bool
	}
	merged := make(map[identity]sets)
	for _, id := range selected {
		merged[id] = sets{}
	}
	for fn, n := range g.nodes {
		source := fn
		if origin := fn.Origin(); origin != nil {
			source = origin
		}
		id, ok := selected[source]
		if !ok {
			continue
		}
		s := merged[id]
		s.direct |= n.direct
		s.reachable |= n.reachable
		s.inherited |= n.inherited
		s.uncertain = s.uncertain || n.uncertain
		s.directUncertain = s.directUncertain || n.directUncertain
		s.inheritedUncertain = s.inheritedUncertain || n.inheritedUncertain
		merged[id] = s
	}
	stats := Stats{Packages: len(selectedPackages), SourceFunctions: len(merged)}
	records := make([]Record, 0, len(merged))
	inventory := make([]Record, 0, len(merged))
	for id, s := range merged {
		set := s.reachable
		if opts.Direct {
			set = s.direct
		}
		direct := make(map[string]bool)
		for _, name := range s.direct.Names() {
			direct[name] = true
		}
		r := Record{Package: id.pkg, Function: id.name, Effects: []Effect{}, Uncertain: s.uncertain}
		for _, name := range set.Names() {
			if matches(name, opts.Effect) {
				r.Effects = append(r.Effects, Effect{Identifier: name, Direct: direct[name]})
			}
		}
		full := Record{Package: id.pkg, Function: id.name, Effects: []Effect{}, Uncertain: s.uncertain}
		for _, name := range s.reachable.Names() {
			full.Effects = append(full.Effects, Effect{Identifier: name, Direct: direct[name]})
		}
		inventory = append(inventory, full)
		if s.reachable != 0 {
			stats.Effectful++
		} else if s.uncertain {
			stats.UnresolvedOnly++
		} else {
			stats.Uncharacterized++
		}
		if s.direct != 0 {
			stats.Direct++
		}
		if s.inherited != 0 {
			stats.Inherited++
		}
		if s.uncertain {
			stats.Uncertain++
		}
		if recordMatches(r, opts) && (opts.Effect == "" || len(r.Effects) != 0) && (opts.All || len(r.Effects) != 0 || r.Uncertain) {
			records = append(records, r)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Package != records[j].Package {
			return records[i].Package < records[j].Package
		}
		return records[i].Function < records[j].Function
	})
	stats.Emitted = len(records)
	stats.Omitted = stats.SourceFunctions - stats.Emitted
	report := Report{Records: records, Stats: stats}
	if opts.EffectView {
		sort.Slice(inventory, func(i, j int) bool {
			return symbolKey(Symbol{inventory[i].Package, inventory[i].Function}) < symbolKey(Symbol{inventory[j].Package, inventory[j].Function})
		})
		report.Effects = g.effectReport(opts, stats, inventory)
	}
	if opts.Details {
		if opts.MaxPaths <= 0 {
			opts.MaxPaths = 1000
		}
		if opts.MaxSites <= 0 {
			opts.MaxSites = 1000
		}
		roots := make(map[Symbol]bool)
		for _, r := range records {
			roots[Symbol{r.Package, r.Function}] = true
		}
		report.Details = g.explain(opts, roots)
		for id, s := range merged {
			symbol := Symbol{id.pkg, id.name}
			if roots[symbol] {
				report.Details.Uncertainty = append(report.Details.Uncertainty, FunctionUncertainty{Symbol: symbol, Direct: s.directUncertain, Inherited: s.inheritedUncertain})
			}
		}
		sort.Slice(report.Details.Uncertainty, func(i, j int) bool {
			return symbolKey(report.Details.Uncertainty[i].Symbol) < symbolKey(report.Details.Uncertainty[j].Symbol)
		})
	}
	return report, nil
}

func (g *graph) isStandardLibrary(fn *ssa.Function) bool {
	if obj, ok := fn.Object().(*types.Func); ok && obj.Pkg() != nil {
		return g.standard[obj.Pkg()]
	}
	return fn.Pkg != nil && fn.Pkg.Pkg != nil && g.standard[fn.Pkg.Pkg]
}

func sourceFunction(fn *ssa.Function) bool {
	if origin := fn.Origin(); origin != nil {
		fn = origin
	}
	switch fn.Syntax().(type) {
	case *ast.FuncDecl, *ast.FuncLit:
		return true
	}
	// Body-less declarations are source functions too; synthetic wrappers and
	// lowered range-over-function yield helpers are not source declarations.
	return fn.Synthetic == "" && fn.Object() != nil && fn.Pos().IsValid()
}

func localName(obj *types.Func) string {
	key := effects.Key(obj)
	return strings.TrimPrefix(key, obj.Pkg().Path()+".")
}

type node struct {
	fn                                  *ssa.Function
	direct, reachable                   effects.Set
	inherited                           effects.Set
	uncertain                           bool
	directUncertain, inheritedUncertain bool
	callers, candidateCallers           map[*node]bool
	incoming                            []callLink // Detailed reverse traversal on the same SSA nodes.
}

type callLink struct {
	caller    *node
	edge      string // Empty only for non-source SSA forwarding instructions.
	candidate bool   // VTA possibility, not an established call.
}

type graph struct {
	prog             *ssa.Program
	nodes            map[*ssa.Function]*node
	pending          []*node
	standard         map[*types.Package]bool
	packageIDs       map[*types.Package]string
	selected         map[*ssa.Function]identity
	selectedPackages map[string]bool
	detailed         bool
	sites            map[string]*Site
	edges            map[string]CallEdge
	fileKeys         map[string]string
	siteNodes        map[string]map[*node]bool
	candidates       map[ssa.CallInstruction][]*ssa.Function
}

func (g *graph) ensure(fn *ssa.Function) *node {
	if n := g.nodes[fn]; n != nil {
		return n
	}
	n := &node{fn: fn, callers: make(map[*node]bool), candidateCallers: make(map[*node]bool)}
	g.nodes[fn] = n
	g.pending = append(g.pending, n)
	return n
}

func summary(fn *ssa.Function) (effects.Set, bool) {
	obj, _ := fn.Object().(*types.Func)
	return effects.Lookup(obj)
}

func summaryCall(fn *ssa.Function, call *ssa.CallCommon) (effects.Set, bool) {
	obj, _ := fn.Object().(*types.Func)
	if s, ok := effects.LookupInterface(obj); ok {
		// A bound interface method still has open implementation dispatch.
		return s | effects.Unknown, true
	}
	return effects.LookupCall(obj, call.Args)
}

func applySummary(n *node, set effects.Set) {
	if set&effects.Unknown != 0 {
		n.uncertain = true
		n.directUncertain = true
		set &^= effects.Unknown
	}
	n.direct |= set
}

func (g *graph) build() {
	for len(g.pending) > 0 {
		n := g.pending[len(g.pending)-1]
		g.pending = g.pending[:len(g.pending)-1]
		if s, ok := summary(n.fn); ok {
			applySummary(n, s)
			// A selected summarized API is a boundary, not a fabricated call
			// to itself. Its opacity can still originate at its declaration.
			if s&effects.Unknown != 0 {
				g.unknown(n, n.fn.Pos(), apiKey(n.fn), summaryReason(apiKey(n.fn), s))
			}
			continue
		}
		if len(n.fn.Blocks) == 0 {
			g.unknown(n, n.fn.Pos(), apiKey(n.fn), "missing_body")
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
						g.unknown(n, site.Pos(), "builtin."+builtin.Name(), "unsupported_resolution")
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
				if call.IsInvoke() {
					if s, ok := effects.LookupInterface(call.Method); ok {
						applySummary(n, s)
						g.addSite(n, site.Pos(), effects.Key(call.Method), s, "interface_summary", "")
					}
				}
				if unknown {
					api, reason := "", "function_value"
					if call.IsInvoke() {
						api, reason = effects.Key(call.Method), "interface_dispatch"
					}
					g.unknown(n, site.Pos(), api, reason)
				}
				for _, target := range targets {
					if s, ok := summaryCall(target, call); ok {
						applySummary(n, s)
						g.addSite(n, site.Pos(), apiKey(target), s, summaryRecognition(target), summaryReason(apiKey(target), s))
					} else if len(target.Blocks) == 0 || g.isStandardLibrary(target) {
						reason := "unrecognized_stdlib"
						if len(target.Blocks) == 0 {
							reason = "missing_body"
						}
						if strings.HasPrefix(apiKey(target), "reflect.") {
							reason = "reflection_opaque"
						}
						g.unknown(n, site.Pos(), apiKey(target), reason)
					} else {
						callee := g.ensure(target)
						callee.callers[n] = true
						if g.detailed {
							callee.incoming = append(callee.incoming, callLink{caller: n, edge: g.addEdge(n, target, site.Pos(), false)})
						}
					}
				}
				// VTA candidates are deliberately additional, lower-confidence
				// relationships. Keep the unresolved boundary above: libraries and
				// external callers may still supply implementations VTA cannot see.
				for _, target := range g.candidates[site] {
					if containsFunction(targets, target) || target == nil || len(target.Blocks) == 0 || g.isStandardLibrary(target) {
						continue
					}
					callee := g.ensure(target)
					callee.candidateCallers[n] = true
					if g.detailed {
						callee.incoming = append(callee.incoming, callLink{caller: n, edge: g.addEdge(n, target, site.Pos(), true), candidate: true})
					}
				}
			}
		}
	}
}

// resolveCandidates runs VTA over selected source functions and their closures.
// This bounds the cost for library investigations while still modeling value
// flow through the code under analysis. Edges are candidate evidence only.
func (g *graph) resolveCandidates(selected map[*ssa.Function]identity) {
	funcs := make(map[*ssa.Function]bool, len(selected))
	for fn := range selected {
		funcs[fn] = true
	}
	for _, n := range vta.CallGraph(funcs, nil).Nodes {
		for _, edge := range n.Out {
			if edge.Site != nil && edge.Callee != nil && edge.Callee.Func != nil && edge.Site.Common().StaticCallee() == nil {
				g.candidates[edge.Site] = append(g.candidates[edge.Site], edge.Callee.Func)
			}
		}
	}
	for site, targets := range g.candidates {
		sort.Slice(targets, func(i, j int) bool { return targets[i].String() < targets[j].String() })
		g.candidates[site] = uniqueFunctions(targets)
	}
}

func uniqueFunctions(in []*ssa.Function) []*ssa.Function {
	out := in[:0]
	for _, fn := range in {
		if len(out) == 0 || out[len(out)-1] != fn {
			out = append(out, fn)
		}
	}
	return out
}

func containsFunction(in []*ssa.Function, want *ssa.Function) bool {
	for _, fn := range in {
		if fn == want {
			return true
		}
	}
	return false
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

// concreteValues resolves only values proven by local SSA flow. It deliberately
// does not use class-hierarchy analysis: an arbitrary implementation of (for
// example) io.Writer is not evidence that it reaches this particular call.
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
	concrete, uncertain := concreteValues(call.Value, make(map[ssa.Value]bool))
	iface := call.Value.Type().Underlying().(*types.Interface)
	var targets []*ssa.Function
	seen := make(map[*ssa.Function]bool)
	for _, t := range concrete {
		if types.IsInterface(t) || !types.Implements(t, iface) {
			uncertain = true
			continue
		}
		sel := g.prog.MethodSets.MethodSet(t).Lookup(call.Method.Pkg(), call.Method.Name())
		if sel == nil {
			uncertain = true
			continue
		}
		if fn := g.prog.MethodValue(sel); fn != nil {
			if !seen[fn] {
				seen[fn] = true
				targets = append(targets, fn)
			}
		} else {
			uncertain = true
		}
	}
	return targets, uncertain || len(targets) == 0
}

// Monotone reverse worklist: nodes gain a bounded number of effect bits.
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
			caller.inherited |= n.reachable
			caller.inheritedUncertain = caller.inheritedUncertain || n.uncertain
			changed := false
			if next := caller.reachable | n.reachable; next != caller.reachable {
				caller.reachable = next
				changed = true
			}
			if n.uncertain && !caller.uncertain {
				caller.uncertain = true
				changed = true
			}
			if changed && !queued[caller] {
				queued[caller] = true
				queue = append(queue, caller)
			}
		}
		// Candidate edges intentionally propagate possible effects for discovery,
		// but are not added to inherited/direct function summaries. Their only
		// representation is the explicitly marked effect-view graph.
	}
}
