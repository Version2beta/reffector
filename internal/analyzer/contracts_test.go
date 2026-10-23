package analyzer

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/Version2beta/reffector/internal/effects"
)

// Different SSA variants can share a source implementation without sharing a
// graph node. Check the report boundary independently of VTA's current choices.
func TestContractReportsNormalizeVariantEvidence(t *testing.T) {
	a, b := &ssa.Function{}, &ssa.Function{}
	id := identity{"example.com/variants", "(Backend).Save"}
	g := &graph{
		prog:     &ssa.Program{Fset: token.NewFileSet()},
		selected: map[*ssa.Function]identity{a: id, b: id},
		nodes: map[*ssa.Function]*node{
			a: {reachable: effects.FSWrite, possible: effects.FSRead | effects.FSRemove},
			b: {reachable: effects.FSRead, possible: effects.FSWrite | effects.FSCreate},
		},
		contracts: map[string]*interfaceContract{"site:test": {
			InterfaceContract: InterfaceContract{Site: "site:test", Open: true, Implementations: []ContractImplementation{}},
			implementations:   map[*ssa.Function]string{a: "contract_compatible", b: "candidate_vta"},
		}},
	}
	for i := 0; i < 20; i++ {
		got := g.contractReports(map[string]bool{"site:test": true})
		if len(got) != 1 || !got[0].Open || len(got[0].Implementations) != 1 {
			t.Fatalf("contracts: %+v", got)
		}
		impl := got[0].Implementations[0]
		if impl.Relationship != "candidate_vta" || !impl.Selected || !reflect.DeepEqual(impl.Effects, []string{"fs.read", "fs.write"}) || !reflect.DeepEqual(impl.PossibleEffects, []string{"fs.create", "fs.remove"}) {
			t.Fatalf("non-normalized variant evidence: %+v", impl)
		}
	}
	g.contracts["site:test"].implementations[a] = "established"
	if got := g.contractReports(map[string]bool{"site:test": true}); got[0].Implementations[0].Relationship != "established" {
		t.Fatalf("strongest relationship lost: %+v", got)
	}
	if len(g.nodes) != 2 || g.nodes[a] == g.nodes[b] {
		t.Fatal("report aggregation merged execution nodes")
	}
}

func TestContractImplementationOrderUsesSourcePosition(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "order.go", "package order\nfunc A() {}\nfunc B() {}\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, _, err := ssautil.BuildPackage(&types.Config{}, fset, types.NewPackage("example.com/order", "order"), []*ast.File{file}, 0)
	if err != nil {
		t.Fatal(err)
	}
	a, b := pkg.Func("A"), pkg.Func("B")
	id := identity{"example.com/order", "SameDisplayedSymbol"}
	g := &graph{
		prog:     pkg.Prog,
		selected: map[*ssa.Function]identity{a: id, b: id},
		nodes:    map[*ssa.Function]*node{a: {}, b: {}},
		contracts: map[string]*interfaceContract{"site:test": {
			InterfaceContract: InterfaceContract{Site: "site:test"},
			implementations:   map[*ssa.Function]string{a: "established", b: "candidate_vta"},
		}},
	}
	for i := 0; i < 20; i++ {
		got := g.contractReports(map[string]bool{"site:test": true})[0].Implementations
		if len(got) != 2 || got[0].Location.Line != 2 || got[1].Location.Line != 3 {
			t.Fatalf("source-position ordering lost: %+v", got)
		}
	}
}
