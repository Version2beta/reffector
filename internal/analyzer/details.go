package analyzer

import (
	"crypto/sha256"
	"fmt"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/Version2beta/reffector/internal/effects"
)

// Details is opt-in. Sites occur once, at their origin, not once per caller.
// Witness paths reference only SSA-supported source call edges. A witness is
// one shortest path per selected root/site pair, not an enumeration of paths.
type Details struct {
	Sites       []Site                `json:"sites"`
	CallEdges   []CallEdge            `json:"call_edges"`
	Uncertainty []FunctionUncertainty `json:"uncertainty"`
	Truncated   Truncation            `json:"truncated"`
}

// Origins live once in Sites; these booleans distinguish direct/inherited
// uncertainty without duplicating every origin into every affected function.
type FunctionUncertainty struct {
	Symbol
	Direct    bool `json:"direct"`
	Inherited bool `json:"inherited"`
}

type Symbol struct {
	Package  string `json:"package"`
	Function string `json:"function"`
}

type Location struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type Site struct {
	ID string `json:"id"`
	Symbol
	Kind            string              `json:"kind"` // call, go, defer, or declaration for a body/summary boundary origin
	API             string              `json:"api"`
	APIs            []string            `json:"apis"`
	Operations      []OperationEvidence `json:"operations"`
	Reasons         []string            `json:"uncertainty_reasons"`
	Location        Location            `json:"location"`
	Effects         []string            `json:"effects"`
	PossibleEffects []string            `json:"possible_effects,omitempty"`
	Recognition     string              `json:"recognition"`
	Uncertain       bool                `json:"uncertain"`
	Witnesses       []Witness           `json:"witnesses,omitempty"`
	Truncated       Truncation          `json:"truncated,omitzero"`
}

// OperationEvidence keeps API alternatives and their own uncertainty together.
// Site-wide fields are the union, not a claim that every alternative has every effect.
type OperationEvidence struct {
	API             string   `json:"api"`
	Effects         []string `json:"effects"`
	PossibleEffects []string `json:"possible_effects,omitempty"`
	Recognition     string   `json:"recognition"`
	Reasons         []string `json:"uncertainty_reasons"`
	Uncertain       bool     `json:"uncertain"`
}

func mergeRecognition(old, next string) string {
	if old == "" || next == "interface_summary" || ((old == "unresolved" || old == "candidate_function_value") && next == "summary") || (old == "unresolved" && next == "candidate_function_value") {
		return next
	}
	return old
}

func (s *Site) addOperation(api string, set effects.Set, recognition, reason string, possible bool) {
	index := -1
	for i := range s.Operations {
		if s.Operations[i].API == api {
			index = i
			break
		}
	}
	if index == -1 {
		s.Operations = append(s.Operations, OperationEvidence{API: api, Effects: []string{}, Reasons: []string{}})
		index = len(s.Operations) - 1
	}
	o := &s.Operations[index]
	o.Recognition = mergeRecognition(o.Recognition, recognition)
	o.Uncertain = o.Uncertain || set&effects.Unknown != 0
	for _, name := range (set &^ effects.Unknown).Names() {
		if possible {
			o.PossibleEffects = appendUnique(o.PossibleEffects, name)
		} else {
			o.Effects = appendUnique(o.Effects, name)
		}
	}
	if reason != "" {
		o.Reasons = appendUnique(o.Reasons, reason)
	}
	o.PossibleEffects = without(o.PossibleEffects, o.Effects)
	sort.Slice(s.Operations, func(i, j int) bool { return s.Operations[i].API < s.Operations[j].API })
}

type CallEdge struct {
	ID         string   `json:"id"`
	Caller     Symbol   `json:"caller"`
	Callee     Symbol   `json:"callee"`
	Location   Location `json:"location"`
	Kind       string   `json:"kind"`       // call, go, or defer
	Resolution string   `json:"resolution"` // established, candidate_vta, or candidate_function_value
}

type Witness struct {
	Root   Symbol   `json:"root"`
	Direct bool     `json:"direct"`
	Edges  []string `json:"edges"` // root -> ... -> site owner
}

type Truncation struct {
	Sites bool `json:"sites"`
	Paths bool `json:"paths"`
	Depth bool `json:"depth"`
}

func stableID(kind, key string) string {
	return fmt.Sprintf("%s:%x", kind, sha256.Sum256([]byte(key)))
}

func symbolKey(s Symbol) string { return s.Package + "\x00" + s.Function }

func (g *graph) symbol(fn *ssa.Function) Symbol {
	if origin := fn.Origin(); origin != nil {
		fn = origin
	}
	if id, ok := g.selected[fn]; ok {
		return Symbol{id.pkg, id.name}
	}
	if obj, ok := fn.Object().(*types.Func); ok && obj.Pkg() != nil {
		return Symbol{obj.Pkg().Path(), localName(obj.Origin())}
	}
	if parent := fn.Parent(); parent != nil {
		s := g.symbol(parent)
		s.Function += strings.TrimPrefix(fn.Name(), parent.Name())
		return s
	}
	pkg := ""
	if fn.Pkg != nil && fn.Pkg.Pkg != nil {
		pkg = fn.Pkg.Pkg.Path()
	}
	return Symbol{pkg, fn.Name()}
}

func (g *graph) location(pos token.Pos) Location {
	p := g.prog.Fset.PositionFor(pos, false)
	return Location{filepath.ToSlash(p.Filename), p.Line, p.Column}
}

func (g *graph) positionKey(loc Location) string {
	file := g.fileKeys[loc.File]
	if file == "" {
		file = loc.File
	}
	return fmt.Sprintf("%s:%d:%d", file, loc.Line, loc.Column)
}

func apiKey(fn *ssa.Function) string {
	if origin := fn.Origin(); origin != nil {
		fn = origin
	}
	obj, _ := fn.Object().(*types.Func)
	if obj != nil {
		obj = obj.Origin()
	}
	return effects.Key(obj)
}

func (g *graph) addSite(n *node, pos token.Pos, api string, set effects.Set, recognition, reason string) {
	g.addSiteEvidence(n, pos, api, set, recognition, reason, false)
}

func (g *graph) addPossibleSite(n *node, pos token.Pos, api string, set effects.Set, reason string) {
	g.addSiteEvidence(n, pos, api, set, "candidate_function_value", reason, true)
}

func (g *graph) addSiteEvidence(n *node, pos token.Pos, api string, set effects.Set, recognition, reason string, possible bool) {
	if !g.detailed || !pos.IsValid() || !sourceFunction(n.fn) {
		return
	}
	// Effect-free summaries are boundaries, not operation or uncertainty sites.
	if set == 0 {
		return
	}
	loc, symbol := g.location(pos), g.symbol(n.fn)
	id := stableID("site", symbolKey(symbol)+"\x00"+g.positionKey(loc))
	if g.siteNodes[id] == nil {
		g.siteNodes[id] = make(map[*node]bool)
	}
	g.siteNodes[id][n] = true
	site := g.sites[id]
	if site == nil {
		site = &Site{ID: id, Symbol: symbol, API: api, APIs: []string{}, Reasons: []string{}, Location: loc, Effects: []string{}, Recognition: recognition, Witnesses: []Witness{}}
		site.Kind = "call"
		if pos == n.fn.Pos() {
			site.Kind = "declaration"
		}
		g.sites[id] = site
	}
	if kind := g.siteKinds[id]; kind != "" {
		site.Kind = kind
	}
	site.addOperation(api, set, recognition, reason, possible)
	// Deterministic precedence when source/test/generic alternatives merge.
	site.Recognition = mergeRecognition(site.Recognition, recognition)
	if api != "" {
		site.APIs = appendUnique(site.APIs, api)
	}
	if len(site.APIs) == 1 {
		site.API = site.APIs[0]
	} else {
		site.API = ""
	}
	if reason != "" {
		site.Reasons = appendUnique(site.Reasons, reason)
	}
	names := map[string]bool{}
	for _, name := range site.Effects {
		names[name] = true
	}
	for _, name := range (set &^ effects.Unknown).Names() {
		if possible {
			site.PossibleEffects = appendUnique(site.PossibleEffects, name)
		} else {
			names[name] = true
		}
	}
	site.Effects = site.Effects[:0]
	for name := range names {
		site.Effects = append(site.Effects, name)
	}
	sort.Strings(site.Effects)
	site.PossibleEffects = without(site.PossibleEffects, site.Effects)
	site.Uncertain = site.Uncertain || set&effects.Unknown != 0
}

func (g *graph) addEdge(n *node, target *ssa.Function, call ssa.CallInstruction, candidate bool) string {
	if !g.detailed || !call.Pos().IsValid() || !sourceFunction(n.fn) {
		return ""
	}
	caller, callee, loc := g.symbol(n.fn), g.symbol(target), g.location(call.Pos())
	resolution := "established"
	if candidate {
		resolution = "candidate_vta"
		if containsExactFunction(g.functionCandidates[call], target) {
			resolution = "candidate_function_value"
		}
	}
	kind := "call"
	switch call.(type) {
	case *ssa.Go:
		kind = "go"
	case *ssa.Defer:
		kind = "defer"
	}
	id := stableID("edge", symbolKey(caller)+"\x00"+g.positionKey(loc)+"\x00"+symbolKey(callee)+"\x00"+resolution)
	g.edges[id] = CallEdge{ID: id, Caller: caller, Callee: callee, Location: loc, Kind: kind, Resolution: resolution}
	return id
}

func matches(value, filter string) bool { return filter == "" || strings.Contains(value, filter) }

func recordMatches(r Record, opts Options) bool {
	functionMatches := matches(r.Function, opts.Function)
	if strings.HasPrefix(opts.Function, "=") {
		functionMatches = r.Function == strings.TrimPrefix(opts.Function, "=")
	}
	return matches(r.Package, opts.Package) && functionMatches
}

func effectMatches(names []string, filter string) bool {
	if filter == "" {
		return true
	}
	for _, name := range names {
		if matches(name, filter) {
			return true
		}
	}
	return false
}

// explain traverses the reverse source graph once per site. It uses no guessed
// edges and emits root-to-site paths, so the same data supports forward witness
// inspection and reverse caller discovery. Traverse the original SSA nodes, not
// merged source symbols: otherwise incompatible generic variants could form a
// fabricated path. Synthetic forwarding instructions cost zero source edges.
func (g *graph) relevantNodes(opts Options, roots map[Symbol]bool) map[*node]bool {
	for _, n := range g.nodes {
		sort.Slice(n.incoming, func(i, j int) bool {
			a, b := n.incoming[i], n.incoming[j]
			if a.edge != b.edge {
				return a.edge < b.edge
			}
			return g.nodeID(a.caller) < g.nodeID(b.caller)
		})
	}
	relevant := map[*node]bool{}
	if g.contextualVTA && !opts.Direct {
		for fn, context := range g.contextualRoots {
			id, selected := g.selected[canonicalFunction(fn)]
			if !selected || !roots[Symbol{id.pkg, id.name}] {
				continue
			}
			for n := range context.reachable {
				relevant[n] = true
			}
		}
		return relevant
	}
	// Establish relevance without a depth limit first. Otherwise unrelated deep
	// dependency chains could consume the site budget just because they truncate.
	outgoing := map[*node][]*node{}
	for _, callee := range g.nodes {
		for _, link := range callee.incoming {
			outgoing[link.caller] = append(outgoing[link.caller], callee)
		}
	}
	pending := []*node{}
	for _, n := range g.nodes {
		if roots[g.symbol(n.fn)] && sourceFunction(n.fn) {
			relevant[n] = true
			pending = append(pending, n)
		}
	}
	if !opts.Direct {
		for len(pending) > 0 {
			n := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			for _, callee := range outgoing[n] {
				if !relevant[callee] {
					relevant[callee] = true
					pending = append(pending, callee)
				}
			}
		}
	}
	return relevant
}

type contextualSiteGraph struct {
	nodes map[*node]bool
	links map[contextualCandidateLink]bool
}

func (g *graph) siteHasCandidatePath(site string) bool {
	for n := range g.siteNodes[site] {
		if g.candidateReachable[n] {
			return true
		}
	}
	return false
}

func (g *graph) contextualSiteNodes(site string, roots map[Symbol]bool) contextualSiteGraph {
	projection := contextualSiteGraph{nodes: make(map[*node]bool), links: make(map[contextualCandidateLink]bool)}
	for fn, context := range g.contextualRoots {
		id, selected := g.selected[canonicalFunction(fn)]
		if !selected || !roots[Symbol{id.pkg, id.name}] {
			continue
		}
		seen := make(map[candidateContextKey]bool)
		queue := make([]candidateContextKey, 0)
		for n := range g.siteNodes[site] {
			for _, state := range context.statesByNode[n] {
				seen[state] = true
				queue = append(queue, state)
			}
		}
		sort.Slice(queue, func(i, j int) bool {
			a, b := queue[i], queue[j]
			if g.nodeID(context.states[a]) != g.nodeID(context.states[b]) {
				return g.nodeID(context.states[a]) < g.nodeID(context.states[b])
			}
			if a.params != b.params {
				return a.params < b.params
			}
			return !a.possible && b.possible
		})
		for len(queue) > 0 {
			state := queue[0]
			queue = queue[1:]
			if n := context.states[state]; n != nil {
				projection.nodes[n] = true
			}
			if state == context.root {
				continue
			}
			for _, transition := range context.incoming[state] {
				// Different source calls may reach one contextual state; retain each edge.
				projection.links[contextualCandidateLinkKey(context.states[transition.from], context.states[transition.to], transition.edge, transition.candidate)] = true
				if seen[transition.from] {
					continue
				}
				seen[transition.from] = true
				queue = append(queue, transition.from)
			}
		}
	}
	return projection
}

func (g *graph) contextualWitness(site string, context *candidateContextResult, maxDepth int) ([]string, bool, bool) {
	type step struct {
		state candidateContextKey
		path  []string
	}
	queue := []step{}
	distance := make(map[candidateContextKey]int)
	for n := range g.siteNodes[site] {
		for _, state := range context.statesByNode[n] {
			queue = append(queue, step{state: state})
			distance[state] = 0
		}
	}
	sort.Slice(queue, func(i, j int) bool {
		a, b := queue[i].state, queue[j].state
		if g.nodeID(context.states[a]) != g.nodeID(context.states[b]) {
			return g.nodeID(context.states[a]) < g.nodeID(context.states[b])
		}
		if a.params != b.params {
			return a.params < b.params
		}
		return !a.possible && b.possible
	})
	depthTruncated := false
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if len(current.path) != distance[current.state] {
			continue
		}
		if current.state == context.root {
			return current.path, true, depthTruncated
		}
		for _, transition := range context.incoming[current.state] {
			path := current.path
			if transition.edge != "" {
				path = append([]string{transition.edge}, path...)
			}
			if old, ok := distance[transition.from]; ok && old <= len(path) {
				continue
			}
			if len(path) > maxDepth {
				depthTruncated = true
				continue
			}
			distance[transition.from] = len(path)
			next := step{state: transition.from, path: path}
			if transition.edge == "" {
				queue = append([]step{next}, queue...)
			} else {
				queue = append(queue, next)
			}
		}
	}
	return nil, false, depthTruncated
}

func (g *graph) explain(opts Options, roots map[Symbol]bool) *Details {
	d := &Details{Sites: []Site{}, CallEdges: []CallEdge{}, Uncertainty: []FunctionUncertainty{}}
	relevant := g.relevantNodes(opts, roots)
	sites := make([]*Site, 0, len(g.sites))
	for _, s := range g.sites {
		if !effectMatches(s.APIs, opts.Operation) || !effectMatches(s.effectNames(), opts.Effect) {
			continue
		}
		for n := range g.siteNodes[s.ID] {
			if relevant[n] {
				sites = append(sites, s)
				break
			}
		}
	}
	sort.Slice(sites, func(i, j int) bool {
		a, b := sites[i], sites[j]
		ka, kb := symbolKey(a.Symbol), symbolKey(b.Symbol)
		if ka != kb {
			return ka < kb
		}
		if a.Location.File != b.Location.File {
			return a.Location.File < b.Location.File
		}
		if a.Location.Line != b.Location.Line {
			return a.Location.Line < b.Location.Line
		}
		if a.Location.Column != b.Location.Column {
			return a.Location.Column < b.Location.Column
		}
		return a.ID < b.ID
	})
	budget := opts.MaxPaths
	used := map[string]CallEdge{}
	type step struct {
		n    *node
		path []string
	}
	type contextRoot struct {
		fn     *ssa.Function
		symbol Symbol
		result *candidateContextResult
	}
	contextRoots := []contextRoot{}
	if g.contextualVTA && !opts.Direct {
		for fn, result := range g.contextualRoots {
			id, selected := g.selected[canonicalFunction(fn)]
			if selected && roots[Symbol{id.pkg, id.name}] {
				contextRoots = append(contextRoots, contextRoot{fn: fn, symbol: Symbol{id.pkg, id.name}, result: result})
			}
		}
		sort.Slice(contextRoots, func(i, j int) bool {
			if symbolKey(contextRoots[i].symbol) != symbolKey(contextRoots[j].symbol) {
				return symbolKey(contextRoots[i].symbol) < symbolKey(contextRoots[j].symbol)
			}
			return g.variant(contextRoots[i].fn) < g.variant(contextRoots[j].fn)
		})
	}
	for _, s := range sites {
		if len(d.Sites) >= opts.MaxSites {
			d.Truncated.Sites = true
			break
		}
		group := *s
		found := map[Symbol]bool{}
		if len(contextRoots) > 0 && g.siteHasCandidatePath(s.ID) {
			for _, root := range contextRoots {
				if found[root.symbol] {
					continue
				}
				path, ok, depthTruncated := g.contextualWitness(s.ID, root.result, opts.MaxDepth)
				group.Truncated.Depth = group.Truncated.Depth || depthTruncated
				if !ok {
					continue
				}
				if budget == 0 {
					group.Truncated.Paths = true
					break
				}
				found[root.symbol] = true
				group.Witnesses = append(group.Witnesses, Witness{Root: root.symbol, Direct: len(path) == 0, Edges: path})
				budget--
				for _, id := range path {
					used[id] = g.edges[id]
				}
			}
		} else {
			queue := []step{}
			distance := map[*node]int{}
			for n := range g.siteNodes[s.ID] {
				if !relevant[n] {
					continue
				}
				queue = append(queue, step{n: n, path: []string{}})
				distance[n] = 0
			}
			sort.Slice(queue, func(i, j int) bool { return queue[i].n.fn.String() < queue[j].n.fn.String() })
			for len(queue) > 0 {
				current := queue[0]
				queue = queue[1:]
				if len(current.path) != distance[current.n] {
					continue
				}
				symbol := g.symbol(current.n.fn)
				if roots[symbol] && sourceFunction(current.n.fn) && !found[symbol] {
					if budget == 0 {
						group.Truncated.Paths = true
						break
					}
					found[symbol] = true
					group.Witnesses = append(group.Witnesses, Witness{Root: symbol, Direct: len(current.path) == 0, Edges: current.path})
					budget--
					for _, id := range current.path {
						used[id] = g.edges[id]
					}
				}
				for _, link := range current.n.incoming {
					if !relevant[link.caller] {
						continue
					}
					path := current.path
					if link.edge != "" {
						if opts.Direct {
							continue
						}
						path = append([]string{link.edge}, path...)
					}
					if old, ok := distance[link.caller]; ok && old <= len(path) {
						continue
					}
					if len(path) > opts.MaxDepth {
						group.Truncated.Depth = true
						continue
					}
					distance[link.caller] = len(path)
					next := step{n: link.caller, path: path}
					if link.edge == "" {
						queue = append([]step{next}, queue...)
					} else {
						queue = append(queue, next)
					}
				}
			}
		}
		// Filters select witness roots, not helper origins. Do not show unrelated
		// dependency sites. A truncated group may have undiscovered selected roots.
		if len(group.Witnesses) > 0 || group.Truncated.Paths || group.Truncated.Depth {
			d.Sites = append(d.Sites, group)
			d.Truncated.Paths = d.Truncated.Paths || group.Truncated.Paths
			d.Truncated.Depth = d.Truncated.Depth || group.Truncated.Depth
		}
	}
	for _, edge := range used {
		d.CallEdges = append(d.CallEdges, edge)
	}
	sort.Slice(d.CallEdges, func(i, j int) bool { return d.CallEdges[i].ID < d.CallEdges[j].ID })
	return d
}

// Alternative summaries never promote even common effects: this analysis does
// not prove normal invocation (nil alternatives and open flows may remain).
func (s *Site) effectNames() []string {
	return append(append([]string(nil), s.Effects...), s.PossibleEffects...)
}

func without(names, excluded []string) []string {
	var out []string
	for _, name := range names {
		if !contains(excluded, name) {
			out = append(out, name)
		}
	}
	return out
}
