package analyzer

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/Version2beta/reffector/internal/effects"
	"golang.org/x/tools/go/ssa"
)

// AuditCoverage counts source-equivalent sites, not affected functions or SSA variants.
type AuditCoverage struct {
	Stats
	SelectedPackages    []string `json:"packages"`
	UncertainSites      int      `json:"uncertain_sites"`
	UnresolvedSites     int      `json:"unresolved_sites"`
	KnownUncertainSites int      `json:"known_uncertain_sites"`
	EmittedSites        int      `json:"sites_emitted"`
	DeclarationSites    int      `json:"declaration_sites_emitted"`
	FunctionRecords     int      `json:"function_records_emitted"`
}

// AuditReport retains the existing graph for bounded, one-record-at-a-time encoding.
// It does not construct reverse edges or traverse the program per site.
type AuditReport struct {
	Coverage  AuditCoverage
	g         *graph
	sites     []string
	functions []AuditFunction
}

type auditCall struct {
	dispatch   string
	candidates []*ssa.Function
	supported  map[*ssa.Function]bool
}

// AuditCandidate identifies an existing VTA alternative, including SSA variants.
type AuditCandidate struct {
	ID string `json:"id"`
	Symbol
	Variant   string `json:"variant"`
	API       string `json:"api"`
	Supported bool   `json:"effect_propagation_supported"`
}

// AuditSite is source-equivalent evidence; node IDs retain separate SSA identities.
type AuditSite struct {
	RecordType    string `json:"record_type"`
	SchemaVersion int    `json:"schema_version"`
	Site
	PrimaryReason   string           `json:"primary_reason"`
	UnknownAPI      bool             `json:"unknown_api"`
	Selected        bool             `json:"selected"`
	Origin          string           `json:"origin"`
	Dispatch        []string         `json:"dispatch_kinds"`
	Nodes           []GraphNode      `json:"ssa_nodes"`
	PossibleEffects []string         `json:"possible_effects"`
	Candidates      []AuditCandidate `json:"vta_candidates"`
	CandidateCount  int              `json:"vta_candidate_count"`
}

// AuditFunction is the full selected source summary, suitable for control selection.
type AuditFunction struct {
	RecordType    string `json:"record_type"`
	SchemaVersion int    `json:"schema_version"`
	Record
	Locations []Location `json:"locations"`
}

func (g *graph) recordAuditCall(n *node, call ssa.CallInstruction, targets []*ssa.Function) {
	if !call.Pos().IsValid() || !sourceFunction(n.fn) {
		return
	}
	id := stableID("site", symbolKey(g.symbol(n.fn))+"\x00"+g.positionKey(g.location(call.Pos())))
	if g.sites[id] == nil {
		return
	}
	a := auditCall{dispatch: "other", candidates: g.candidates[call], supported: map[*ssa.Function]bool{}}
	if call.Common().IsInvoke() {
		a.dispatch = "interface"
	} else if call.Common().StaticCallee() == nil {
		a.dispatch = "function_value"
		if _, builtin := call.Common().Value.(*ssa.Builtin); builtin {
			a.dispatch = "other"
		}
	}
	for _, target := range a.candidates {
		if target != nil && !containsFunction(targets, target) && len(target.Blocks) > 0 && !g.isStandardLibrary(target) {
			a.supported[target] = true
		}
	}
	if g.auditCalls == nil {
		g.auditCalls = map[string][]auditCall{}
	}
	g.auditCalls[id] = append(g.auditCalls[id], a)
}

func (g *graph) auditReport(opts Options, stats Stats, inventory []Record) *AuditReport {
	r := &AuditReport{g: g, Coverage: AuditCoverage{Stats: stats}}
	for pkg := range g.selectedPackages {
		r.Coverage.SelectedPackages = append(r.Coverage.SelectedPackages, pkg)
	}
	sort.Strings(r.Coverage.SelectedPackages)
	for id, s := range g.sites {
		if !s.Uncertain {
			continue
		}
		r.Coverage.UncertainSites++
		if len(s.Effects) == 0 {
			r.Coverage.UnresolvedSites++
		} else {
			r.Coverage.KnownUncertainSites++
		}
		if !opts.AuditIncludeKnown && len(s.Effects) > 0 {
			continue
		}
		r.sites = append(r.sites, id)
		if s.Kind == "declaration" {
			r.Coverage.DeclarationSites++
		}
	}
	sort.Strings(r.sites)
	r.Coverage.EmittedSites = len(r.sites)
	if opts.All {
		locations := map[Symbol][]Location{}
		for fn, id := range g.selected {
			s := Symbol{id.pkg, id.name}
			loc := g.location(fn.Pos())
			duplicate := false
			for _, old := range locations[s] {
				duplicate = duplicate || old == loc
			}
			if !duplicate {
				locations[s] = append(locations[s], loc)
			}
		}
		sort.Slice(inventory, func(i, j int) bool {
			return symbolKey(Symbol{inventory[i].Package, inventory[i].Function}) < symbolKey(Symbol{inventory[j].Package, inventory[j].Function})
		})
		for _, rec := range inventory {
			locs := locations[Symbol{rec.Package, rec.Function}]
			sort.Slice(locs, func(i, j int) bool {
				if locs[i].File != locs[j].File {
					return locs[i].File < locs[j].File
				}
				if locs[i].Line != locs[j].Line {
					return locs[i].Line < locs[j].Line
				}
				return locs[i].Column < locs[j].Column
			})
			r.functions = append(r.functions, AuditFunction{"function", 1, rec, locs})
		}
	}
	r.Coverage.FunctionRecords = len(r.functions)
	return r
}

// WriteJSONL writes only records. Possible effects come solely from candidates
// attached to this call instruction and accepted by the existing propagation;
// never from unrelated operations in the enclosing function. Candidate summaries
// are transitive and may themselves contain uncertainty, not proof of execution.
func (r *AuditReport) WriteJSONL(w io.Writer) error {
	enc := json.NewEncoder(w)
	nodes := map[*node]GraphNode{}
	candidateIDs := map[*ssa.Function]AuditCandidate{}
	for _, id := range r.sites {
		s := r.g.sites[id]
		a := AuditSite{RecordType: "site", SchemaVersion: 1, Site: *s, Origin: "dependency", Dispatch: []string{}, Nodes: []GraphNode{}, PossibleEffects: []string{}, Candidates: []AuditCandidate{}}
		if len(s.Reasons) > 0 {
			a.PrimaryReason = s.Reasons[0]
		} else {
			a.PrimaryReason = "unknown"
		}
		for _, op := range s.Operations {
			a.UnknownAPI = a.UnknownAPI || op.API == ""
		}
		for n := range r.g.siteNodes[id] {
			gn, ok := nodes[n]
			if !ok {
				_, selected := r.g.selected[canonicalFunction(n.fn)]
				gn = GraphNode{ID: r.g.nodeID(n), Symbol: r.g.symbol(n.fn), Variant: r.g.variant(n.fn), Selected: selected, Synthetic: !sourceFunction(n.fn), Uncertain: n.uncertain, DirectUncertain: n.directUncertain, InheritedUncertain: n.inheritedUncertain}
				nodes[n] = gn
			}
			a.Selected = a.Selected || gn.Selected
			a.Nodes = append(a.Nodes, gn)
		}
		if a.Selected {
			a.Origin = "selected"
		}
		sort.Slice(a.Nodes, func(i, j int) bool { return a.Nodes[i].ID < a.Nodes[j].ID })
		candidates := map[string]AuditCandidate{}
		var possible effects.Set
		for _, call := range r.g.auditCalls[id] {
			a.Dispatch = appendUnique(a.Dispatch, call.dispatch)
			for _, fn := range call.candidates {
				c, ok := candidateIDs[fn]
				if !ok {
					c = AuditCandidate{ID: stableID("node", r.g.variant(fn)+"\x00"+r.g.positionKey(r.g.location(fn.Pos()))), Symbol: r.g.symbol(fn), Variant: r.g.variant(fn), API: apiKey(fn)}
					candidateIDs[fn] = c
				}
				old := candidates[c.ID]
				c.Supported = old.Supported || call.supported[fn]
				candidates[c.ID] = c
				if call.supported[fn] {
					if n := r.g.nodes[fn]; n != nil {
						possible |= n.reachable | n.possible
					}
				}
			}
		}
		if len(a.Dispatch) == 0 {
			a.Dispatch = []string{"unknown"}
		}
		for _, c := range candidates {
			a.Candidates = append(a.Candidates, c)
		}
		sort.Slice(a.Candidates, func(i, j int) bool { return a.Candidates[i].ID < a.Candidates[j].ID })
		a.CandidateCount = len(a.Candidates)
		for _, name := range possible.Names() {
			if !contains(s.Effects, name) {
				a.PossibleEffects = append(a.PossibleEffects, name)
			}
		}
		if err := enc.Encode(a); err != nil {
			return err
		}
	}
	for _, f := range r.functions {
		if err := enc.Encode(f); err != nil {
			return err
		}
	}
	return nil
}
