package analyzer

import (
	"go/types"
	"sort"

	"golang.org/x/tools/go/ssa"
)

// EffectReport is a projection of the same graph used for function summaries
// and forward witnesses. Source sites merge; traversal nodes never do.
type EffectReport struct {
	SchemaVersion int                 `json:"schema_version"`
	Groups        []EffectGroup       `json:"groups"`
	Sites         []Site              `json:"sites"`
	Unresolved    []string            `json:"unresolved_sites"`
	Reverse       []ReverseGraph      `json:"reverse"`
	Nodes         []GraphNode         `json:"nodes"`
	Edges         []GraphEdge         `json:"edges"`
	Coverage      EffectCoverage      `json:"coverage"`
	Inventory     []Record            `json:"inventory,omitzero"`
	Contracts     []InterfaceContract `json:"interface_contracts,omitempty"`
	Truncated     GraphTruncation     `json:"truncated"`
}

type EffectGroup struct {
	Effect        string   `json:"effect"`
	Sites         []string `json:"sites"`
	PossibleSites []string `json:"possible_sites,omitempty"`
}

type EffectCoverage struct {
	Stats
	SelectedPackages         []string        `json:"packages"`
	OperationSites           int             `json:"operation_sites_identified"`
	PossibleOperationSites   int             `json:"possible_operation_sites_identified,omitempty"`
	SelectedOperationSites   int             `json:"selected_operation_sites"`
	DependencyOperationSites int             `json:"dependency_operation_sites"`
	UnresolvedSites          int             `json:"unresolved_sites_identified"`
	FilteredSites            int             `json:"sites_matching_filters"`
	EmittedSites             int             `json:"sites_emitted"`
	EstablishedEdges         int             `json:"established_edges_emitted"`
	CandidateEdges           int             `json:"candidate_edges_emitted"`
	Truncated                GraphTruncation `json:"truncated"`
}

type GraphNode struct {
	ID string `json:"id"`
	Symbol
	Variant            string `json:"variant"`
	Selected           bool   `json:"selected"`
	Synthetic          bool   `json:"synthetic"`
	Uncertain          bool   `json:"uncertain"`
	DirectUncertain    bool   `json:"direct_uncertainty"`
	InheritedUncertain bool   `json:"inherited_uncertainty"`
}

type GraphEdge struct {
	ID        string    `json:"id"`
	Caller    string    `json:"caller"`
	Callee    string    `json:"callee"`
	Source    *CallEdge `json:"source,omitempty"`
	Synthetic bool      `json:"synthetic"`
	Candidate bool      `json:"candidate"` // Candidate possibility, not established reachability.
}

type NodeRef struct {
	ID    string `json:"id"`
	Depth int    `json:"depth"`
}

type ReverseGraph struct {
	Site      string          `json:"site"`
	Origins   []string        `json:"origins"`
	Nodes     []NodeRef       `json:"nodes"`
	Edges     []string        `json:"edges"`
	Revisit   bool            `json:"revisit"` // Shared nodes, not dropped relationships.
	Cycle     bool            `json:"cycle"`
	Truncated GraphTruncation `json:"truncated"`
}

type GraphTruncation struct {
	Sites bool `json:"sites"`
	Edges bool `json:"edges"`
	Depth bool `json:"depth"`
}

func (g *graph) variant(fn *ssa.Function) string {
	pkg := fn.Package()
	pkgID := ""
	if pkg != nil {
		pkgID = g.packageIDs[pkg.Pkg]
	}
	if pkgID == "" && fn.Object() != nil {
		pkgID = g.packageIDs[fn.Object().Pkg()]
	}
	if parent := fn.Parent(); parent != nil {
		pkgID += "|parent=" + g.variant(parent)
	}
	// An imported generic can be instantiated with two same-spelled types
	// from different test variants. Include type-package identity, not just
	// fn.String(), which intentionally prints their common import paths.
	qualify := func(p *types.Package) string { return p.Path() + "[" + g.packageIDs[p] + "]" }
	key := pkgID + "|" + fn.String() + "|" + types.TypeString(fn.Signature, qualify) + "|" + fn.Synthetic
	for _, arg := range fn.TypeArgs() {
		key += "|" + types.TypeString(arg, qualify)
	}
	return key
}

func (g *graph) nodeID(n *node) string {
	return stableID("node", g.variant(n.fn)+"\x00"+g.positionKey(g.location(n.fn.Pos())))
}

func (g *graph) effectReport(opts Options, stats Stats, inventory []Record) *EffectReport {
	if opts.MaxSites <= 0 {
		opts.MaxSites = 1000
	}
	if opts.MaxEdges <= 0 {
		opts.MaxEdges = 1000
	}
	r := &EffectReport{SchemaVersion: 1, Groups: []EffectGroup{}, Sites: []Site{}, Unresolved: []string{}, Reverse: []ReverseGraph{}, Nodes: []GraphNode{}, Edges: []GraphEdge{}, Coverage: EffectCoverage{Stats: stats}, Contracts: []InterfaceContract{}}
	if opts.All {
		r.Inventory = inventory
	}
	for pkg := range g.selectedPackages {
		r.Coverage.SelectedPackages = append(r.Coverage.SelectedPackages, pkg)
	}
	sort.Strings(r.Coverage.SelectedPackages)
	// Coverage is computed for the full selected scope, independently of every
	// output filter. Query relevance can then narrow roots without losing it.
	allRoots, roots := map[Symbol]bool{}, map[Symbol]bool{}
	for _, record := range inventory {
		s := Symbol{record.Package, record.Function}
		allRoots[s] = true
		if recordMatches(record, opts) {
			roots[s] = true
		}
	}
	fullOpts := opts
	fullOpts.Direct = false
	full := g.relevantNodes(fullOpts, allRoots)
	relevant := g.relevantNodes(opts, roots)
	sites := []*Site{}
	for _, s := range g.sites {
		inScope, inQuery, selected := false, false, false
		for n := range g.siteNodes[s.ID] {
			inScope = inScope || full[n]
			inQuery = inQuery || relevant[n]
			selected = selected || allRoots[g.symbol(n.fn)]
		}
		if !inScope {
			continue
		}
		if len(s.Effects) > 0 {
			r.Coverage.OperationSites++
			if selected {
				r.Coverage.SelectedOperationSites++
			} else {
				r.Coverage.DependencyOperationSites++
			}
		} else if s.Uncertain {
			r.Coverage.UnresolvedSites++
		}
		if len(s.PossibleEffects) > 0 {
			r.Coverage.PossibleOperationSites++
		}
		if !inQuery || !effectMatches(s.effectNames(), opts.Effect) || !effectMatches(s.APIs, opts.Operation) || !effectMatches(s.Reasons, opts.Reason) || (opts.API != "" && !contains(s.APIs, opts.API)) || (opts.Site != "" && s.ID != opts.Site) || (opts.Unresolved && (len(s.Effects) != 0 || !s.Uncertain)) || (!opts.Unresolved && len(s.effectNames()) == 0 && !(opts.Contracts && g.contracts[s.ID] != nil)) {
			continue
		}
		sites = append(sites, s)
	}
	sort.Slice(sites, func(i, j int) bool {
		a, b := sites[i], sites[j]
		if symbolKey(a.Symbol) != symbolKey(b.Symbol) {
			return symbolKey(a.Symbol) < symbolKey(b.Symbol)
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
	r.Coverage.FilteredSites = len(sites)
	if len(sites) > opts.MaxSites {
		sites = sites[:opts.MaxSites]
		r.Truncated.Sites = true
	}
	groups := map[string][]string{}
	possibleGroups := map[string][]string{}
	usedNodes, usedEdges := map[string]GraphNode{}, map[string]GraphEdge{}
	budget := opts.MaxEdges // Global unique graph-edge budget, shared across sites.
	emittedContracts := map[string]bool{}
	for _, site := range sites {
		r.Sites = append(r.Sites, *site)
		emittedContracts[site.ID] = true
		if len(site.Effects) == 0 && site.Uncertain {
			r.Unresolved = append(r.Unresolved, site.ID)
		}
		for _, effect := range site.Effects {
			if matches(effect, opts.Effect) {
				groups[effect] = append(groups[effect], site.ID)
			}
		}
		for _, effect := range site.PossibleEffects {
			if matches(effect, opts.Effect) {
				possibleGroups[effect] = append(possibleGroups[effect], site.ID)
			}
		}
		siteRelevant := relevant
		var siteLinks map[contextualCandidateLink]bool
		if g.contextualVTA && !opts.Direct && g.siteHasCandidatePath(site.ID) {
			projection := g.contextualSiteNodes(site.ID, roots)
			siteRelevant, siteLinks = projection.nodes, projection.links
		}
		reverse := g.reverseGraph(site.ID, siteRelevant, siteLinks, opts.MaxDepth, &budget, usedNodes, usedEdges)
		r.Truncated.Edges = r.Truncated.Edges || reverse.Truncated.Edges
		r.Truncated.Depth = r.Truncated.Depth || reverse.Truncated.Depth
		r.Reverse = append(r.Reverse, reverse)
	}
	for effect, ids := range groups {
		r.Groups = append(r.Groups, EffectGroup{Effect: effect, Sites: ids, PossibleSites: possibleGroups[effect]})
		delete(possibleGroups, effect)
	}
	for effect, ids := range possibleGroups {
		r.Groups = append(r.Groups, EffectGroup{Effect: effect, Sites: []string{}, PossibleSites: ids})
	}
	sort.Slice(r.Groups, func(i, j int) bool { return r.Groups[i].Effect < r.Groups[j].Effect })
	for _, n := range usedNodes {
		r.Nodes = append(r.Nodes, n)
	}
	for _, e := range usedEdges {
		r.Edges = append(r.Edges, e)
	}
	sort.Slice(r.Nodes, func(i, j int) bool { return r.Nodes[i].ID < r.Nodes[j].ID })
	sort.Slice(r.Edges, func(i, j int) bool { return r.Edges[i].ID < r.Edges[j].ID })
	r.Coverage.EmittedSites = len(r.Sites)
	r.Coverage.Truncated = r.Truncated
	for _, edge := range r.Edges {
		if edge.Candidate {
			r.Coverage.CandidateEdges++
		} else {
			r.Coverage.EstablishedEdges++
		}
	}
	if opts.Contracts {
		r.Contracts = g.contractReports(emittedContracts)
	}
	return r
}

func contains(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func (g *graph) reverseGraph(site string, relevant map[*node]bool, links map[contextualCandidateLink]bool, maxDepth int, budget *int, nodes map[string]GraphNode, edges map[string]GraphEdge) ReverseGraph {
	r := ReverseGraph{Site: site, Origins: []string{}, Nodes: []NodeRef{}, Edges: []string{}}
	distance := map[*node]int{}
	queue := []*node{}
	for n := range g.siteNodes[site] {
		if relevant[n] {
			distance[n] = 0
			queue = append(queue, n)
			r.Origins = append(r.Origins, g.nodeID(n))
		}
	}
	sort.Slice(queue, func(i, j int) bool { return g.nodeID(queue[i]) < g.nodeID(queue[j]) })
	sort.Strings(r.Origins)
	included := map[string]bool{}
	// Zero-cost synthetic forwarding is kept as an explicit edge, not collapsed
	// into source symbols. This also keeps bound methods and variants sound.
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, link := range n.incoming {
			if links != nil && !links[contextualCandidateLinkKey(link.caller, n, link.edge, link.candidate)] {
				continue
			}
			if !relevant[link.caller] || maxDepth == 0 {
				continue
			}
			d := distance[n]
			if link.edge != "" {
				d++
			}
			old, seen := distance[link.caller]
			if d > maxDepth && (!seen || old > maxDepth) {
				r.Truncated.Depth = true
				continue
			}
			id := stableID("link", g.nodeID(link.caller)+"\x00"+g.nodeID(n)+"\x00"+link.edge)
			if !included[id] {
				if _, exists := edges[id]; !exists {
					if *budget == 0 {
						r.Truncated.Edges = true
						continue
					}
					*budget--
					e := GraphEdge{ID: id, Caller: g.nodeID(link.caller), Callee: g.nodeID(n), Synthetic: link.edge == "", Candidate: link.candidate}
					if link.edge != "" {
						source := g.edges[link.edge]
						e.Source = &source
					}
					edges[id] = e
				}
				included[id] = true
				if seen {
					r.Revisit = true
				}
			}
			if !seen || d < old {
				distance[link.caller] = d
				if link.edge == "" {
					queue = append([]*node{link.caller}, queue...)
				} else {
					queue = append(queue, link.caller)
				}
			}
		}
	}
	// Complete the induced subgraph, including boundary-to-boundary edges.
	// Sort before spending the remaining shared budget.
	ordered := make([]*node, 0, len(distance))
	for n := range distance {
		ordered = append(ordered, n)
	}
	sort.Slice(ordered, func(i, j int) bool { return g.nodeID(ordered[i]) < g.nodeID(ordered[j]) })
	r.Truncated.Depth = false
	for _, n := range ordered {
		for _, link := range n.incoming {
			if links != nil && !links[contextualCandidateLinkKey(link.caller, n, link.edge, link.candidate)] {
				continue
			}
			if !relevant[link.caller] {
				continue
			}
			if maxDepth == 0 {
				r.Truncated.Depth = true
				continue
			}
			if _, ok := distance[link.caller]; !ok {
				if link.edge != "" && distance[n] >= maxDepth {
					r.Truncated.Depth = true
				}
				continue
			}
			id := stableID("link", g.nodeID(link.caller)+"\x00"+g.nodeID(n)+"\x00"+link.edge)
			if included[id] {
				continue
			}
			if _, exists := edges[id]; !exists {
				if *budget == 0 {
					r.Truncated.Edges = true
					continue
				}
				*budget--
				e := GraphEdge{ID: id, Caller: g.nodeID(link.caller), Callee: g.nodeID(n), Synthetic: link.edge == "", Candidate: link.candidate}
				if link.edge != "" {
					source := g.edges[link.edge]
					e.Source = &source
				}
				edges[id] = e
			}
			included[id] = true
			r.Revisit = true
		}
	}
	for n, depth := range distance {
		id := g.nodeID(n)
		_, selected := g.selected[n.fn]
		if origin := n.fn.Origin(); origin != nil {
			_, selected = g.selected[origin]
		}
		nodes[id] = GraphNode{ID: id, Symbol: g.symbol(n.fn), Variant: g.variant(n.fn), Selected: selected, Synthetic: !sourceFunction(n.fn), Uncertain: n.uncertain, DirectUncertain: n.directUncertain, InheritedUncertain: n.inheritedUncertain}
		r.Nodes = append(r.Nodes, NodeRef{id, depth})
	}
	for id := range included {
		r.Edges = append(r.Edges, id)
	}
	sort.Strings(r.Edges)
	sort.Slice(r.Nodes, func(i, j int) bool {
		if r.Nodes[i].Depth != r.Nodes[j].Depth {
			return r.Nodes[i].Depth < r.Nodes[j].Depth
		}
		return r.Nodes[i].ID < r.Nodes[j].ID
	})
	// Kahn's algorithm detects cycles without recursion or path enumeration.
	degree, next := map[string]int{}, map[string][]string{}
	for _, n := range r.Nodes {
		degree[n.ID] = 0
	}
	for _, id := range r.Edges {
		e := edges[id]
		degree[e.Caller]++
		next[e.Callee] = append(next[e.Callee], e.Caller)
	}
	ready := []string{}
	for id, d := range degree {
		if d == 0 {
			ready = append(ready, id)
		}
	}
	visited := 0
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		visited++
		for _, caller := range next[id] {
			degree[caller]--
			if degree[caller] == 0 {
				ready = append(ready, caller)
			}
		}
	}
	r.Cycle = visited != len(degree)
	return r
}
