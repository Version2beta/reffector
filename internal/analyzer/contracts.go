package analyzer

import (
	"go/token"
	"go/types"
	"sort"

	"golang.org/x/tools/go/ssa"

	"github.com/Version2beta/reffector/internal/effects"
)

// InterfaceContract is an opt-in inventory of implementations that satisfy one
// invoked interface method. It is not a call graph: relationship describes the
// evidence available for this exact source invocation.
type InterfaceContract struct {
	Site              string                   `json:"site"`
	Method            string                   `json:"method"`
	Open              bool                     `json:"open"`
	AvailablePackages []string                 `json:"available_packages"`
	Implementations   []ContractImplementation `json:"implementations"`
}

type ContractImplementation struct {
	Symbol          Symbol   `json:"symbol"`
	Location        Location `json:"location"`
	Relationship    string   `json:"relationship"` // established, candidate_vta, contract_compatible
	Effects         []string `json:"effects"`
	PossibleEffects []string `json:"possible_effects,omitempty"`
	Selected        bool     `json:"selected"`
}

type interfaceContract struct {
	InterfaceContract
	implementations map[*ssa.Function]string
}

func (g *graph) contractSiteID(n *node, pos token.Pos) string {
	return stableID("site", symbolKey(g.symbol(n.fn))+"\x00"+g.positionKey(g.location(pos)))
}

// prepareContracts inventories method-set-compatible implementations before
// graph construction so their recognized effects are available to the opt-in
// report. It never creates execution edges.
func (g *graph) prepareContracts(funcs map[*ssa.Function]bool) {
	for fn := range funcs {
		for _, block := range fn.Blocks {
			for _, instruction := range block.Instrs {
				site, ok := instruction.(ssa.CallInstruction)
				if !ok || !site.Common().IsInvoke() {
					continue
				}
				call := site.Common()
				iface, ok := call.Value.Type().Underlying().(*types.Interface)
				if !ok {
					continue
				}
				id := g.contractSiteID(g.ensure(fn), site.Pos())
				c, exists := g.contracts[id]
				targets, open := g.invoke(call)
				if !exists {
					c = &interfaceContract{InterfaceContract: InterfaceContract{
						Site: id, Method: effects.Key(call.Method), Open: open,
						AvailablePackages: g.availablePackages(), Implementations: []ContractImplementation{},
					}, implementations: make(map[*ssa.Function]string)}
					g.contracts[id] = c
				} else {
					c.Open = c.Open || open
				}
				implementations := g.implementations(iface, call.Method)
				// Instantiated generic receiver types are not package members.
				// Keep actual SSA targets as evidence as well as the compatible
				// source inventory; only the report merges their source identities.
				implementations = append(implementations, targets...)
				implementations = append(implementations, g.candidates[site]...)
				for _, target := range implementations {
					relationship := "contract_compatible"
					if containsFunction(targets, target) {
						relationship = "established"
					} else if containsFunction(g.candidates[site], target) {
						relationship = "candidate_vta"
					}
					old := c.implementations[target]
					if relationshipRank(relationship) > relationshipRank(old) {
						c.implementations[target] = relationship
					}
					g.ensure(target)
				}
			}
		}
	}
}

func (g *graph) availablePackages() []string {
	seen := map[string]bool{}
	for _, pkg := range g.prog.AllPackages() {
		if pkg != nil && pkg.Pkg != nil {
			seen[pkg.Pkg.Path()] = true
		}
	}
	out := make([]string, 0, len(seen))
	for path := range seen {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func (g *graph) implementations(iface *types.Interface, method *types.Func) []*ssa.Function {
	seen := map[*ssa.Function]bool{}
	var out []*ssa.Function
	for _, pkg := range g.prog.AllPackages() {
		if pkg == nil {
			continue
		}
		for _, member := range pkg.Members {
			typ, ok := member.(*ssa.Type)
			if !ok || types.IsInterface(typ.Type()) {
				continue
			}
			for _, candidate := range []types.Type{typ.Type(), types.NewPointer(typ.Type())} {
				if !types.Implements(candidate, iface) {
					continue
				}
				sel := g.prog.MethodSets.MethodSet(candidate).Lookup(method.Pkg(), method.Name())
				if sel == nil {
					continue
				}
				fn := canonicalFunction(g.prog.MethodValue(sel))
				if fn != nil && !seen[fn] {
					seen[fn] = true
					out = append(out, fn)
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return g.variant(out[i]) < g.variant(out[j]) })
	return out
}

func relationshipRank(relationship string) int {
	switch relationship {
	case "established":
		return 3
	case "candidate_vta":
		return 2
	case "contract_compatible":
		return 1
	default:
		return 0
	}
}

func (g *graph) contractReports(sites map[string]bool) []InterfaceContract {
	var out []InterfaceContract
	for id, contract := range g.contracts {
		if !sites[id] {
			continue
		}
		result := contract.InterfaceContract
		unique := map[string]ContractImplementation{}
		for fn, relationship := range contract.implementations {
			n := g.nodes[fn]
			if n == nil {
				continue
			}
			_, selected := g.selected[fn]
			if origin := fn.Origin(); origin != nil {
				_, selected = g.selected[origin]
			}
			implementation := ContractImplementation{
				Symbol: g.symbol(fn), Location: g.location(fn.Pos()), Relationship: relationship,
				Effects: n.reachable.Names(), PossibleEffects: (n.possible &^ n.reachable).Names(), Selected: selected,
			}
			key := symbolKey(implementation.Symbol) + "\x00" + g.positionKey(implementation.Location)
			old, exists := unique[key]
			if !exists {
				unique[key] = implementation
				continue
			}
			old.Selected = old.Selected || implementation.Selected
			for _, effect := range implementation.Effects {
				old.Effects = appendUnique(old.Effects, effect)
			}
			for _, effect := range implementation.PossibleEffects {
				old.PossibleEffects = appendUnique(old.PossibleEffects, effect)
			}
			if relationshipRank(implementation.Relationship) > relationshipRank(old.Relationship) {
				old.Relationship = implementation.Relationship
			}
			unique[key] = old
		}
		for _, implementation := range unique {
			// A variant can establish an effect that was only possible in another.
			// Normalize after the union, not only on each individual SSA node.
			possible := implementation.PossibleEffects[:0]
			for _, effect := range implementation.PossibleEffects {
				if !contains(implementation.Effects, effect) {
					possible = append(possible, effect)
				}
			}
			implementation.PossibleEffects = possible
			sort.Strings(implementation.Effects)
			sort.Strings(implementation.PossibleEffects)
			result.Implementations = append(result.Implementations, implementation)
		}
		sort.Slice(result.Implementations, func(i, j int) bool {
			a, b := result.Implementations[i], result.Implementations[j]
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
			return a.Relationship < b.Relationship
		})
		out = append(out, result)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Site < out[j].Site })
	return out
}
