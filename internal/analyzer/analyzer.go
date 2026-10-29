// Package analyzer computes possible effects without executing analyzed code.
package analyzer

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/Version2beta/reffector/internal/effects"
)

type Options struct {
	Dir                                        string // Empty means the current directory.
	Tests                                      bool
	Direct                                     bool
	Exact                                      bool // Do not expand local directories to recursive patterns.
	All                                        bool // Include source functions without effects or uncertainty.
	Details                                    bool
	AuditUnresolved, AuditIncludeKnown         bool // JSONL projection without reverse graphs or limits.
	EffectView                                 bool
	Unresolved, Contracts                      bool   // Contracts adds opt-in interface implementation leads.
	API, Site                                  string // Exact identities.
	Reason                                     string // Uncertainty-reason substring in effect view.
	Package, Function, Effect, Operation       string // Substring filters; Function accepts =name for exact matching.
	MaxDepth, MaxPaths, MaxSites, MaxEdges     int
	ArgumentProvenance                         bool
	ControlFlow                                bool
	ControlFlowLimits                          ControlFlowOptions
	ArgumentProvenanceLimits                   ArgumentProvenanceOptions
	DependencyVTAFunctions, DependencyVTAEdges int // Both positive to opt into bounded dependency expansion.
}

// DependencyVTAStats reports dependency-VTA expansion and root-context
// refinement, both enabled only by positive dependency budgets.
type DependencyVTAStats struct {
	FunctionsAdded       int    `json:"functions_added"`
	EdgesAdded           int    `json:"edges_added"`
	FunctionBudget       int    `json:"function_budget"`
	EdgeBudget           int    `json:"edge_budget"`
	Truncated            bool   `json:"truncated"`
	LimitHit             string `json:"limit_hit,omitempty"`
	ContextStates        int    `json:"context_states"`
	ContextBudget        int    `json:"context_budget"`
	ContextTruncated     bool   `json:"context_truncated"`
	ContextRootsFallback int    `json:"context_roots_fallback,omitempty"`
}

// SourceModule identifies module metadata for selected package roots. It does
// not identify a particular checkout revision or source-tree content.
type SourceModule struct {
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
}

// Stats counts functions, not effect pairs. Direct and inherited can overlap.
// Counts describe the selected inventory before output filtering.
type Stats struct {
	Packages        int                 `json:"packages_selected"`
	SourceFunctions int                 `json:"source_functions"`
	Emitted         int                 `json:"emitted"`
	Omitted         int                 `json:"omitted"`
	Direct          int                 `json:"direct"`
	Inherited       int                 `json:"inherited"`
	Uncertain       int                 `json:"uncertain"`
	Effectful       int                 `json:"effectful"`
	UnresolvedOnly  int                 `json:"unresolved_only"`
	Uncharacterized int                 `json:"no_effects_no_uncertainty"`
	DependencyVTA   *DependencyVTAStats `json:"dependency_vta,omitempty"`
	HostOS                string         `json:"host_os"`
	HostArch              string         `json:"host_arch"`
	BuildGOOS             string         `json:"build_goos"`
	BuildGOARCH           string         `json:"build_goarch"`
	SourceModules                  []SourceModule `json:"source_modules"`
	SourceRevisionStatus           string         `json:"source_revision_status"`
	SourceRevisionReason           string         `json:"source_revision_reason"`
	WholeSourceFunctionDenominator string         `json:"whole_source_function_denominator_status"`
	FunctionDenominatorReason      string         `json:"whole_source_function_denominator_reason"`
	TestsIncluded                  bool           `json:"tests_included"`
	PackageLoad           string         `json:"package_load_status"`
	SelectedRoots         []string       `json:"selected_package_roots"`
	MaxSites              int            `json:"max_sites"`
	MaxDepth              int            `json:"max_depth"`
	MaxEdges              int            `json:"max_edges"`
}

type Report struct {
	Records            []Record            `json:"records"`
	Stats              Stats               `json:"stats"`
	Details            *Details            `json:"details,omitempty"`
	Effects            *EffectReport       `json:"-"`
	Audit              *AuditReport        `json:"-"`
	ArgumentProvenance *ArgumentProvenance `json:"argument_provenance,omitempty"`
	ControlFlow        *ControlFlow        `json:"control_flow,omitempty"`
}

type identity struct{ pkg, name string }

// Record is one function's analysis result. Effects are omitted from text
// output when empty, but Uncertain remains available in JSON.
type Record struct {
	Package         string   `json:"package"`
	Function        string   `json:"function"`
	Effects         []Effect `json:"effects"`                    // Direct or established-call effects.
	PossibleEffects []string `json:"possible_effects,omitempty"` // Candidate-only effects.
	Uncertain       bool     `json:"uncertain"`
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
	if opts.DependencyVTAFunctions < 0 || opts.DependencyVTAEdges < 0 || (opts.DependencyVTAFunctions == 0) != (opts.DependencyVTAEdges == 0) {
		return Report{}, fmt.Errorf("dependency VTA requires positive function and edge budgets")
	}
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
		detailed: opts.Details || opts.EffectView || opts.AuditUnresolved || opts.ArgumentProvenance || opts.ControlFlow, audit: opts.AuditUnresolved, packageIDs: make(map[*types.Package]string), sites: make(map[string]*Site), edges: make(map[string]CallEdge), fileKeys: make(map[string]string), siteNodes: make(map[string]map[*node]bool), siteKinds: make(map[string]string), candidates: make(map[ssa.CallInstruction][]*ssa.Function), contracts: make(map[string]*interfaceContract)}

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
	funcs := g.selectedFunctions()
	vtaFuncs, dependencyVTA := g.resolveCandidates(funcs, opts.DependencyVTAFunctions, opts.DependencyVTAEdges)
	g.contractsEnabled = opts.Contracts
	if opts.Contracts {
		g.prepareContracts(funcs)
	}
	g.collectFunctionArguments(vtaFuncs)
	g.build()
	if dependencyVTA != nil {
		g.refineCandidateContexts(dependencyVTA)
	}
	g.propagate()

	// Test variants and generic instantiations may share a source identity.
	// Merge before emitting so each function/effect pair appears exactly once.
	type sets struct {
		direct, reachable, inherited, possible         effects.Set
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
		if g.contextualVTA {
			if context := g.contextualRoots[fn]; context != nil {
				s.possible |= context.possible
			}
		} else {
			s.possible |= n.possible
		}
		s.uncertain = s.uncertain || n.uncertain
		s.directUncertain = s.directUncertain || n.directUncertain
		s.inheritedUncertain = s.inheritedUncertain || n.inheritedUncertain
		merged[id] = s
	}
	selectedRoots := make([]string, 0, len(selectedPackages))
	for pkg := range selectedPackages {
		selectedRoots = append(selectedRoots, pkg)
	}
	sort.Strings(selectedRoots)
	sourceModuleSet := make(map[SourceModule]bool)
	for _, pkg := range pkgs {
		if pkg.Module != nil && pkg.Module.Path != "" {
			sourceModuleSet[SourceModule{Path: pkg.Module.Path, Version: pkg.Module.Version}] = true
		}
	}
	sourceModules := make([]SourceModule, 0, len(sourceModuleSet))
	for module := range sourceModuleSet {
		sourceModules = append(sourceModules, module)
	}
	sort.Slice(sourceModules, func(i, j int) bool {
		if sourceModules[i].Path != sourceModules[j].Path {
			return sourceModules[i].Path < sourceModules[j].Path
		}
		return sourceModules[i].Version < sourceModules[j].Version
	})
	buildGOOS, buildGOARCH := os.Getenv("GOOS"), os.Getenv("GOARCH")
	if buildGOOS == "" {
		buildGOOS = runtime.GOOS
	}
	if buildGOARCH == "" {
		buildGOARCH = runtime.GOARCH
	}
	stats := Stats{Packages: len(selectedPackages), SourceFunctions: len(merged), DependencyVTA: dependencyVTA,
		HostOS: runtime.GOOS, HostArch: runtime.GOARCH, BuildGOOS: buildGOOS, BuildGOARCH: buildGOARCH,
		SourceModules: sourceModules, SourceRevisionStatus: "unavailable",
		SourceRevisionReason: "the analyzer does not inspect VCS metadata or source snapshot provenance",
		WholeSourceFunctionDenominator: "unavailable",
		FunctionDenominatorReason:      "analysis covers selected loaded roots, not all packages or build/platform source variants",
		TestsIncluded: opts.Tests, PackageLoad: "successful", SelectedRoots: selectedRoots,
		MaxSites: opts.MaxSites, MaxDepth: opts.MaxDepth, MaxEdges: opts.MaxEdges}
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
		r := Record{Package: id.pkg, Function: id.name, Effects: []Effect{}, PossibleEffects: []string{}, Uncertain: s.uncertain}
		for _, name := range set.Names() {
			if matches(name, opts.Effect) {
				r.Effects = append(r.Effects, Effect{Identifier: name, Direct: direct[name]})
			}
		}
		if !opts.Direct {
			for _, name := range (s.possible &^ s.reachable).Names() {
				if matches(name, opts.Effect) {
					r.PossibleEffects = append(r.PossibleEffects, name)
				}
			}
		}
		full := Record{Package: id.pkg, Function: id.name, Effects: []Effect{}, PossibleEffects: []string{}, Uncertain: s.uncertain}
		for _, name := range s.reachable.Names() {
			full.Effects = append(full.Effects, Effect{Identifier: name, Direct: direct[name]})
		}
		for _, name := range (s.possible &^ s.reachable).Names() {
			full.PossibleEffects = append(full.PossibleEffects, name)
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
		hasEffects := len(r.Effects) != 0 || len(r.PossibleEffects) != 0
		if recordMatches(r, opts) && (opts.Effect == "" || hasEffects) && (opts.All || hasEffects || r.Uncertain) {
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
	if opts.ArgumentProvenance {
		provenance, err := g.analyzeArgumentProvenance(opts.ArgumentProvenanceLimits)
		if err != nil {
			return Report{}, err
		}
		report.ArgumentProvenance = ptrArgumentProvenance(provenance)
	}
	if opts.AuditUnresolved {
		report.Audit = g.auditReport(opts, stats, inventory)
	}
	if opts.EffectView {
		sort.Slice(inventory, func(i, j int) bool {
			return symbolKey(Symbol{inventory[i].Package, inventory[i].Function}) < symbolKey(Symbol{inventory[j].Package, inventory[j].Function})
		})
		report.Effects = g.effectReport(opts, stats, inventory)
	}
	if opts.ControlFlow {
		if report.Effects == nil {
			return Report{}, fmt.Errorf("control-flow projection requires effect view")
		}
		cf, err := g.analyzeControlFlow(report.Effects.Sites, opts.ControlFlowLimits, report.Effects.Truncated.Sites)
		if err != nil {
			return Report{}, err
		}
		report.ControlFlow = &cf
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
	direct, reachable, possible         effects.Set
	inherited                           effects.Set
	uncertain                           bool
	directUncertain, inheritedUncertain bool
	callers, candidateCallers           map[*node]bool
	functionCandidateCallers            map[*node]bool
	incoming                            []callLink // Detailed reverse traversal on the same SSA nodes.
}

type callLink struct {
	caller    *node
	edge      string // Empty only for non-source SSA forwarding instructions.
	candidate bool   // Candidate possibility, not an established call.
}

type graph struct {
	prog                    *ssa.Program
	nodes                   map[*ssa.Function]*node
	pending                 []*node
	standard                map[*types.Package]bool
	packageIDs              map[*types.Package]string
	selected                map[*ssa.Function]identity
	selectedPackages        map[string]bool
	detailed                bool
	audit                   bool
	auditCalls              map[string][]auditCall
	sites                   map[string]*Site
	edges                   map[string]CallEdge
	fileKeys                map[string]string
	siteNodes               map[string]map[*node]bool
	candidates              map[ssa.CallInstruction][]*ssa.Function
	functionCandidates      map[ssa.CallInstruction][]*ssa.Function
	functionValues          map[ssa.Value]functionProvenance
	functionParamCandidates map[*ssa.Parameter]functionProvenance
	siteKinds               map[string]string
	contracts               map[string]*interfaceContract
	contractsEnabled        bool
	contextualVTA           bool
	contextualRoots         map[*ssa.Function]*candidateContextResult
	candidateReachable      map[*node]bool
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
				if api, set, kind, ok := ssaOperation(instr); ok {
					g.recordSSAOperation(n, instr, api, set, kind)
				}
				site, ok := instr.(ssa.CallInstruction)
				if !ok {
					continue
				}
				call := site.Common()
				if builtin, ok := call.Value.(*ssa.Builtin); ok {
					switch builtin.Name() {
					case "close":
						applySummary(n, effects.ChannelClose)
						g.addSite(n, site.Pos(), "builtin.close", effects.ChannelClose, "ssa", "")
					case "print", "println":
						g.unknown(n, site.Pos(), "builtin."+builtin.Name(), "unsupported_resolution")
						if g.audit {
							g.recordAuditCall(n, site, nil)
						}
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
					targets, unknown = g.functionTargets(site)
				}
				if call.IsInvoke() {
					if s, ok := effects.LookupInterface(call.Method); ok {
						applySummary(n, s)
						g.addSite(n, site.Pos(), effects.Key(call.Method), s, "interface_summary", "")
					}
				}
				valueAPI, valueSet, knownValueCall := effects.LookupCallValue(call.Value)
				if unknown && !knownValueCall {
					api, reason := "", "function_value"
					if call.IsInvoke() {
						api, reason = effects.Key(call.Method), "interface_dispatch"
					}
					g.unknown(n, site.Pos(), api, reason)
				}
				if knownValueCall {
					applySummary(n, valueSet)
					g.addSite(n, site.Pos(), valueAPI, valueSet, "summary", summaryReason(valueAPI, valueSet))
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
						if g.detailed && !g.audit {
							callee.incoming = append(callee.incoming, callLink{caller: n, edge: g.addEdge(n, target, site, false)})
						}
					}
				}
				// Candidate targets are deliberately additional, lower-confidence
				// relationships. Keep the unresolved boundary above: libraries and
				// external callers may still supply implementations VTA cannot see.
				for _, target := range g.callCandidates(site) {
					if containsFunction(targets, target) || target == nil {
						continue
					}
					if containsExactFunction(g.functionCandidates[site], target) {
						if s, ok := summaryCall(target, call); ok {
							n.possible |= s &^ effects.Unknown
							g.addPossibleSite(n, site.Pos(), apiKey(target), s, summaryReason(apiKey(target), s))
							if s&effects.Unknown != 0 {
								g.unknown(n, site.Pos(), apiKey(target), summaryReason(apiKey(target), s))
							}
							continue
						}
						if len(target.Blocks) == 0 || g.isStandardLibrary(target) {
							reason := "unrecognized_stdlib"
							if len(target.Blocks) == 0 {
								reason = "missing_body"
							}
							if strings.HasPrefix(apiKey(target), "reflect.") {
								reason = "reflection_opaque"
							}
							g.unknown(n, site.Pos(), apiKey(target), reason)
							continue
						}
					} else if len(target.Blocks) == 0 || g.isStandardLibrary(target) {
						continue
					}
					callee := g.ensure(target)
					callee.candidateCallers[n] = true
					if containsExactFunction(g.functionCandidates[site], target) {
						if callee.functionCandidateCallers == nil {
							callee.functionCandidateCallers = make(map[*node]bool)
						}
						callee.functionCandidateCallers[n] = true
					}
					if g.detailed && !g.audit {
						callee.incoming = append(callee.incoming, callLink{caller: n, edge: g.addEdge(n, target, site, true), candidate: true})
					}
				}
				if g.audit {
					g.recordAuditCall(n, site, targets)
				}
			}
		}
	}
}

func ssaOperation(instr ssa.Instruction) (api string, set effects.Set, kind string, ok bool) {
	switch instr := instr.(type) {
	case *ssa.Go:
		return "ssa.Go", effects.GoroutineStart, "go", true
	case *ssa.Defer:
		return "ssa.Defer", 0, "defer", true
	case *ssa.Send:
		return "ssa.Send", effects.ChannelSend, "call", true
	case *ssa.Select:
		return "ssa.Select", effects.ChannelSelect, "call", true
	case *ssa.UnOp:
		if instr.Op == token.ARROW {
			return "ssa.Receive", effects.ChannelReceive, "call", true
		}
	}
	return "", 0, "", false
}

func (g *graph) recordSSAOperation(n *node, instr ssa.Instruction, api string, set effects.Set, kind string) {
	applySummary(n, set)
	g.addSite(n, instr.Pos(), api, set, "ssa", "")
	if g.detailed && (kind == "go" || kind == "defer") {
		loc := g.location(instr.Pos())
		id := stableID("site", symbolKey(g.symbol(n.fn))+"\x00"+g.positionKey(loc))
		g.siteKinds[id] = kind
		if site := g.sites[id]; site != nil {
			site.Kind = kind
		}
	}
}

// selectedFunctions adds local instantiations and closures referenced by the
// selected source functions, including methods of local concrete interface values.
// Do not enumerate AllFunctions: it walks dependency
// bodies and materializes unrelated method wrappers before we can filter them.
// Keep both discovery and VTA bounded to the selected packages, and do not add
// synthetic package initializers (which would expand the previous VTA scope).
func (g *graph) selectedFunctions() map[*ssa.Function]bool {
	funcs := make(map[*ssa.Function]bool)
	var visit func(*ssa.Function)
	visit = func(fn *ssa.Function) {
		if fn == nil || funcs[fn] {
			return
		}
		if _, selected := g.selected[fn]; !selected {
			pkg := fn.Package()
			if origin := fn.Origin(); origin != nil {
				pkg = origin.Package()
			}
			if pkg == nil || pkg.Pkg == nil || !g.selectedPackages[pkg.Pkg.Path()] {
				return
			}
		}
		funcs[fn] = true
		for _, child := range fn.AnonFuncs {
			visit(child)
		}
		var operands [10]*ssa.Value
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				if value, ok := instruction.(*ssa.MakeInterface); ok {
					// Interface-only generic instantiations have no direct function
					// operand. Materialize only local receiver methods, not every
					// method of every runtime type in the dependency program.
					typ := types.Unalias(value.X.Type())
					if ptr, ok := typ.(*types.Pointer); ok {
						typ = types.Unalias(ptr.Elem())
					}
					if named, ok := typ.(*types.Named); ok && named.Obj().Pkg() != nil && g.selectedPackages[named.Obj().Pkg().Path()] {
						for method := range g.prog.MethodSets.MethodSet(value.X.Type()).Methods() {
							visit(g.prog.MethodValue(method))
						}
					}
				}
				for _, operand := range instruction.Operands(operands[:0]) {
					if target, ok := (*operand).(*ssa.Function); ok {
						visit(target)
					}
				}
			}
		}
	}
	for fn := range g.selected {
		visit(fn)
	}
	return funcs
}

// resolveCandidates runs VTA over selected source functions, instantiations and closures.
// Optional dependency expansion follows only static calls into loaded, non-selected
// packages and is bounded before those functions are passed to VTA. VTA edges are
// always candidate evidence, never established calls.
func (g *graph) resolveCandidates(funcs map[*ssa.Function]bool, maxFunctions, maxEdges int) (map[*ssa.Function]bool, *DependencyVTAStats) {
	var expansion *DependencyVTAStats
	if maxFunctions > 0 && maxEdges > 0 {
		funcs, expansion = g.expandDependencyFunctions(funcs, maxFunctions, maxEdges)
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
	return funcs, expansion
}

type candidateContextState struct {
	concrete map[types.Type]bool
	unknown  bool
}

type functionContext struct {
	params []candidateContextState
}

type candidateContextKey struct {
	fn       *ssa.Function
	possible bool
	params   string
}

type contextualTransition struct {
	from, to  candidateContextKey
	edge      string
	candidate bool
}

type candidateContextResult struct {
	possible     effects.Set
	reachable    map[*node]bool
	states       map[candidateContextKey]*node
	statesByNode map[*node][]candidateContextKey
	incoming     map[candidateContextKey][]contextualTransition
	root         candidateContextKey
}

type contextualCandidateLink struct {
	caller, callee *node
	edge           string
	candidate      bool
}

type contextualCandidate struct {
	fn       *ssa.Function
	receiver candidateContextState
}

func contextualCandidateLinkKey(caller, callee *node, edge string, candidate bool) contextualCandidateLink {
	return contextualCandidateLink{caller: caller, callee: callee, edge: edge, candidate: candidate}
}

func unknownContextState() candidateContextState {
	return candidateContextState{unknown: true}
}

func knownContextState(typ types.Type) candidateContextState {
	return candidateContextState{concrete: map[types.Type]bool{typ: true}}
}

func cloneContextState(state candidateContextState) candidateContextState {
	out := candidateContextState{unknown: state.unknown}
	if len(state.concrete) > 0 {
		out.concrete = make(map[types.Type]bool, len(state.concrete))
		for typ := range state.concrete {
			out.concrete[typ] = true
		}
	}
	return out
}

func mergeContextState(dst *candidateContextState, src candidateContextState) bool {
	changed := false
	if src.unknown && !dst.unknown {
		dst.unknown = true
		changed = true
	}
	if len(src.concrete) > 0 && dst.concrete == nil {
		dst.concrete = make(map[types.Type]bool, len(src.concrete))
	}
	for typ := range src.concrete {
		if !dst.concrete[typ] {
			dst.concrete[typ] = true
			changed = true
		}
	}
	return changed
}

func (g *graph) contextFingerprint(context *functionContext) string {
	var fingerprint strings.Builder
	qualifier := func(pkg *types.Package) string {
		return pkg.Path() + "[" + g.packageIDs[pkg] + "]"
	}
	for _, parameter := range context.params {
		if parameter.unknown {
			fingerprint.WriteByte('?')
		} else {
			fingerprint.WriteByte('!')
		}
		typeNames := make([]string, 0, len(parameter.concrete))
		for typ := range parameter.concrete {
			typeNames = append(typeNames, types.TypeString(typ, qualifier))
		}
		sort.Strings(typeNames)
		for _, typ := range typeNames {
			fmt.Fprintf(&fingerprint, "%d:%s", len(typ), typ)
		}
		fingerprint.WriteByte(';')
	}
	return fingerprint.String()
}

// refineCandidateContexts computes candidate effects separately for each
// selected root. Concrete interface values can rule out incompatible VTA
// implementations; open inputs retain all candidates. An incomplete context
// budget falls back to the original conservative candidate propagation.
func (g *graph) refineCandidateContexts(stats *DependencyVTAStats) {
	maxInt := int(^uint(0) >> 1)
	budget := stats.FunctionBudget
	if budget > maxInt-stats.EdgeBudget {
		budget = maxInt
	} else {
		budget += stats.EdgeBudget
	}
	stats.ContextBudget = budget
	if len(g.candidates) == 0 && len(g.functionCandidates) == 0 {
		return
	}
	g.candidateReachable = g.structuralCandidateReachability()

	type root struct {
		fn *ssa.Function
		id identity
	}
	roots := make([]root, 0, len(g.nodes))
	for fn := range g.nodes {
		source := fn
		if origin := fn.Origin(); origin != nil {
			source = origin
		}
		id, selected := g.selected[source]
		if selected && sourceFunction(fn) {
			roots = append(roots, root{fn: fn, id: id})
		}
	}
	sort.Slice(roots, func(i, j int) bool {
		a, b := roots[i], roots[j]
		ak, bk := symbolKey(Symbol{a.id.pkg, a.id.name}), symbolKey(Symbol{b.id.pkg, b.id.name})
		if ak != bk {
			return ak < bk
		}
		if a.fn.Pos() != b.fn.Pos() {
			return a.fn.Pos() < b.fn.Pos()
		}
		return g.variant(a.fn) < g.variant(b.fn)
	})

	results := make(map[*ssa.Function]*candidateContextResult, len(roots))
	for _, root := range roots {
		result, states, complete := g.candidateContextForRoot(root.fn, budget-stats.ContextStates)
		stats.ContextStates += states
		if !complete {
			stats.ContextTruncated = true
			stats.ContextRootsFallback = len(roots)
			return
		}
		results[root.fn] = result
	}
	g.contextualRoots = results
	g.contextualVTA = true
}

func (g *graph) candidateContextForRoot(root *ssa.Function, budget int) (*candidateContextResult, int, bool) {
	result := &candidateContextResult{
		reachable:    make(map[*node]bool),
		states:       make(map[candidateContextKey]*node),
		statesByNode: make(map[*node][]candidateContextKey),
		incoming:     make(map[candidateContextKey][]contextualTransition),
	}
	states := make(map[candidateContextKey]*functionContext)
	queue := make([]candidateContextKey, 0)
	stateCount := 0
	complete := true

	add := func(fn *ssa.Function, possible bool, incoming *functionContext) (candidateContextKey, bool) {
		key := candidateContextKey{fn: fn, possible: possible, params: g.contextFingerprint(incoming)}
		if _, exists := states[key]; exists {
			return key, true
		}
		if stateCount == budget {
			complete = false
			return candidateContextKey{}, false
		}
		state := &functionContext{params: make([]candidateContextState, len(incoming.params))}
		for i := range incoming.params {
			state.params[i] = cloneContextState(incoming.params[i])
		}
		states[key] = state
		result.states[key] = g.nodes[fn]
		result.statesByNode[g.nodes[fn]] = append(result.statesByNode[g.nodes[fn]], key)
		stateCount++
		queue = append(queue, key)
		return key, true
	}

	initial := &functionContext{params: make([]candidateContextState, len(root.Params))}
	for i := range initial.params {
		initial.params[i] = unknownContextState()
	}
	rootKey, ok := add(root, false, initial)
	if !ok {
		return result, stateCount, false
	}
	result.root = rootKey
	for len(queue) > 0 && complete {
		key := queue[0]
		queue = queue[1:]
		current := states[key]
		n := g.nodes[key.fn]
		if n == nil {
			continue
		}
		result.reachable[n] = true
		result.possible |= n.possible // Before propagation these are local candidate summaries.
		if key.possible {
			result.possible |= n.direct
		}
		for _, block := range key.fn.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}
				established := g.contextEstablishedTargets(call)
				for _, target := range established {
					if g.nodes[target] == nil {
						continue
					}
					incoming := g.contextForCall(call, key.fn, current, target, nil)
					next, ok := add(target, key.possible, incoming)
					if !ok {
						break
					}
					edge := g.addEdge(n, target, call, false)
					result.incoming[next] = append(result.incoming[next], contextualTransition{from: key, to: next, edge: edge})
				}
				if !complete {
					break
				}
				for _, candidate := range g.contextualCandidates(call, key.fn, current) {
					if containsFunction(established, candidate.fn) || g.nodes[candidate.fn] == nil {
						continue
					}
					incoming := g.contextForCall(call, key.fn, current, candidate.fn, &candidate.receiver)
					next, ok := add(candidate.fn, true, incoming)
					if !ok {
						break
					}
					edge := g.addEdge(n, candidate.fn, call, true)
					result.incoming[next] = append(result.incoming[next], contextualTransition{from: key, to: next, edge: edge, candidate: true})
				}
				if !complete {
					break
				}
			}
			if !complete {
				break
			}
		}
	}
	return result, stateCount, complete
}

// structuralCandidateReachability finds nodes reachable from a selected source
// root through at least one global VTA edge. It bounds per-site context projection
// to origins whose reverse graph could otherwise mix candidate paths.
func (g *graph) structuralCandidateReachability() map[*node]bool {
	type edge struct {
		callee    *node
		candidate bool
	}
	type state struct {
		n        *node
		possible bool
	}
	outgoing := make(map[*node][]edge)
	for _, callee := range g.nodes {
		for caller := range callee.callers {
			outgoing[caller] = append(outgoing[caller], edge{callee: callee})
		}
		for caller := range callee.candidateCallers {
			outgoing[caller] = append(outgoing[caller], edge{callee: callee, candidate: true})
		}
	}
	for caller := range outgoing {
		sort.Slice(outgoing[caller], func(i, j int) bool {
			a, b := outgoing[caller][i], outgoing[caller][j]
			if g.nodeID(a.callee) != g.nodeID(b.callee) {
				return g.nodeID(a.callee) < g.nodeID(b.callee)
			}
			return !a.candidate && b.candidate
		})
	}
	roots := make([]*node, 0)
	for fn := range g.nodes {
		if _, selected := g.selected[canonicalFunction(fn)]; selected && sourceFunction(fn) {
			roots = append(roots, g.nodes[fn])
		}
	}
	sort.Slice(roots, func(i, j int) bool { return g.nodeID(roots[i]) < g.nodeID(roots[j]) })
	queue := make([]state, 0, len(roots))
	seen := make(map[state]bool)
	for _, root := range roots {
		item := state{n: root}
		if !seen[item] {
			seen[item] = true
			queue = append(queue, item)
		}
	}
	possible := make(map[*node]bool)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current.possible {
			possible[current.n] = true
		}
		for _, edge := range outgoing[current.n] {
			next := state{n: edge.callee, possible: current.possible || edge.candidate}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	return possible
}

func (g *graph) contextEstablishedTargets(call ssa.CallInstruction) []*ssa.Function {
	common := call.Common()
	var targets []*ssa.Function
	if fn := common.StaticCallee(); fn != nil {
		targets = []*ssa.Function{fn}
	} else if common.IsInvoke() {
		targets, _ = g.invoke(common)
	} else {
		targets, _ = g.functionTargets(call)
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].String() < targets[j].String() })
	return uniqueFunctions(targets)
}

func (g *graph) contextualCandidates(call ssa.CallInstruction, fn *ssa.Function, context *functionContext) []contextualCandidate {
	candidates := g.callCandidates(call)
	if len(candidates) == 0 {
		return nil
	}
	common := call.Common()
	if !common.IsInvoke() {
		out := make([]contextualCandidate, 0, len(candidates))
		for _, candidate := range candidates {
			out = append(out, contextualCandidate{fn: candidate, receiver: unknownContextState()})
		}
		return out
	}
	iface, ok := types.Unalias(common.Value.Type()).Underlying().(*types.Interface)
	if !ok {
		return unknownContextCandidates(candidates)
	}
	facts := g.contextualValueTypes(common.Value, fn, context, make(map[ssa.Value]bool))
	if facts.unknown || len(facts.concrete) == 0 {
		return unknownContextCandidates(candidates)
	}
	matched := make(map[*ssa.Function]candidateContextState)
	for typ := range facts.concrete {
		if _, isTypeParameter := types.Unalias(typ).(*types.TypeParam); isTypeParameter || types.IsInterface(typ) || !types.Implements(typ, iface) {
			return unknownContextCandidates(candidates)
		}
		selection := g.prog.MethodSets.MethodSet(typ).Lookup(common.Method.Pkg(), common.Method.Name())
		if selection == nil {
			return unknownContextCandidates(candidates)
		}
		method := g.prog.MethodValue(selection)
		if method == nil {
			return unknownContextCandidates(candidates)
		}
		for _, candidate := range candidates {
			if containsFunction([]*ssa.Function{method}, candidate) {
				merge := matched[candidate]
				if merge.concrete == nil {
					merge.concrete = make(map[types.Type]bool)
				}
				merge.concrete[typ] = true
				matched[candidate] = merge
			}
		}
	}
	out := make([]contextualCandidate, 0, len(matched))
	for _, candidate := range candidates {
		if receiver, ok := matched[candidate]; ok {
			out = append(out, contextualCandidate{fn: candidate, receiver: receiver})
		}
	}
	return out
}

func unknownContextCandidates(candidates []*ssa.Function) []contextualCandidate {
	out := make([]contextualCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		out = append(out, contextualCandidate{fn: candidate, receiver: unknownContextState()})
	}
	return out
}

func (g *graph) contextForCall(call ssa.CallInstruction, fn *ssa.Function, context *functionContext, target *ssa.Function, candidateReceiver *candidateContextState) *functionContext {
	out := &functionContext{params: make([]candidateContextState, len(target.Params))}
	for i := range out.params {
		out.params[i] = unknownContextState()
	}
	offset := 0
	if call.Common().IsInvoke() && target.Signature.Recv() != nil && len(out.params) > 0 {
		if candidateReceiver != nil {
			out.params[0] = cloneContextState(*candidateReceiver)
		} else {
			out.params[0] = g.contextualValueTypes(call.Common().Value, fn, context, make(map[ssa.Value]bool))
		}
		offset = 1
	}
	args := call.Common().Args
	if len(args) != len(out.params)-offset {
		return out
	}
	for i, arg := range args {
		out.params[i+offset] = g.contextualValueTypes(arg, fn, context, make(map[ssa.Value]bool))
	}
	return out
}

func (g *graph) contextualValueTypes(value ssa.Value, fn *ssa.Function, context *functionContext, seen map[ssa.Value]bool) candidateContextState {
	if value == nil || seen[value] {
		return candidateContextState{}
	}
	seen[value] = true
	switch value := value.(type) {
	case *ssa.MakeInterface:
		if types.IsInterface(value.X.Type()) {
			return g.contextualValueTypes(value.X, fn, context, seen)
		}
		if _, isTypeParameter := types.Unalias(value.X.Type()).(*types.TypeParam); isTypeParameter {
			return unknownContextState()
		}
		return knownContextState(value.X.Type())
	case *ssa.ChangeInterface:
		return g.contextualValueTypes(value.X, fn, context, seen)
	case *ssa.Phi:
		var out candidateContextState
		for _, edge := range value.Edges {
			mergeContextState(&out, g.contextualValueTypes(edge, fn, context, seen))
		}
		return out
	case *ssa.Parameter:
		if value.Parent() != fn {
			return unknownContextState()
		}
		for i, parameter := range fn.Params {
			if parameter == value && i < len(context.params) {
				return cloneContextState(context.params[i])
			}
		}
		return unknownContextState()
	default:
		concrete, unknown := concreteValues(value, make(map[ssa.Value]bool))
		out := candidateContextState{unknown: unknown}
		if len(concrete) > 0 {
			out.concrete = make(map[types.Type]bool, len(concrete))
			for _, typ := range concrete {
				out.concrete[typ] = true
			}
		}
		return out
	}
}

// expandDependencyFunctions makes a deterministic breadth-first walk from the
// VTA input through static calls in loaded packages outside the selected source
// packages. It has one visited set and counts unique SSA call instructions, so
// recursion and cycles cannot trigger repeated traversal.
func (g *graph) expandDependencyFunctions(initial map[*ssa.Function]bool, maxFunctions, maxEdges int) (map[*ssa.Function]bool, *DependencyVTAStats) {
	functions := make(map[*ssa.Function]bool, len(initial))
	queue := make([]*ssa.Function, 0, len(initial))
	for fn := range initial {
		functions[fn] = true
		queue = append(queue, fn)
	}
	sort.Slice(queue, func(i, j int) bool {
		a, b := g.variant(queue[i]), g.variant(queue[j])
		if a != b {
			return a < b
		}
		return queue[i].Pos() < queue[j].Pos()
	})
	stats := &DependencyVTAStats{FunctionBudget: maxFunctions, EdgeBudget: maxEdges}
	visited := make(map[*ssa.Function]bool)
	edges := make(map[ssa.CallInstruction]bool)
	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		if visited[fn] {
			continue
		}
		visited[fn] = true
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				call, ok := instruction.(ssa.CallInstruction)
				if !ok {
					continue
				}
				target := call.Common().StaticCallee()
				if target == nil || len(target.Blocks) == 0 || g.isStandardLibrary(target) || target.Package() == nil || target.Package().Pkg == nil || g.selectedPackages[target.Package().Pkg.Path()] {
					continue
				}
				if edges[call] {
					continue
				}
				if stats.EdgesAdded == maxEdges {
					stats.Truncated, stats.LimitHit = true, "edges"
					return functions, stats
				}
				if !functions[target] && stats.FunctionsAdded == maxFunctions {
					stats.Truncated, stats.LimitHit = true, "functions"
					return functions, stats
				}
				edges[call] = true
				stats.EdgesAdded++
				if !functions[target] {
					functions[target] = true
					stats.FunctionsAdded++
					queue = append(queue, target)
				}
			}
		}
	}
	return functions, stats
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

func canonicalFunction(fn *ssa.Function) *ssa.Function {
	if fn != nil && fn.Origin() != nil {
		return fn.Origin()
	}
	return fn
}

func containsFunction(in []*ssa.Function, want *ssa.Function) bool {
	want = canonicalFunction(want)
	for _, fn := range in {
		if canonicalFunction(fn) == want {
			return true
		}
	}
	return false
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
			if next := caller.possible | n.possible; next != caller.possible {
				caller.possible = next
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
		for caller := range n.candidateCallers {
			changed := false
			if next := caller.possible | n.reachable | n.possible; next != caller.possible {
				caller.possible, changed = next, true
			}
			// A closed alternative set resolves identity, not opaque work inside
			// its bodies. Preserve that incompleteness without promoting effects.
			if n.uncertain && n.functionCandidateCallers[caller] {
				caller.inheritedUncertain = true
				if !caller.uncertain {
					caller.uncertain, changed = true, true
				}
			}
			if changed && !queued[caller] {
				queued[caller] = true
				queue = append(queue, caller)
			}
		}
	}
}
