package main

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func effectResult(t *testing.T, dir string, args ...string) (analyzer.EffectReport, string) {
	t.Helper()
	out, diagnostic, status := invoke(t, dir, nil, append([]string{"-json"}, args...)...)
	if status != 0 || diagnostic != "" {
		t.Fatalf("%v: exit %d: %s", args, status, diagnostic)
	}
	var r analyzer.EffectReport
	if err := json.Unmarshal([]byte(out), &r); err != nil || r.SchemaVersion != 1 {
		t.Fatalf("effect schema: %v: %s", err, out)
	}
	return r, out
}

func reverseFor(t *testing.T, r analyzer.EffectReport, function string) (analyzer.Site, analyzer.ReverseGraph, map[string]analyzer.GraphNode, map[string]analyzer.GraphEdge) {
	t.Helper()
	nodes, edges := map[string]analyzer.GraphNode{}, map[string]analyzer.GraphEdge{}
	for _, n := range r.Nodes {
		nodes[n.ID] = n
	}
	for _, e := range r.Edges {
		edges[e.ID] = e
		if e.Source != nil && (e.Source.Location.Line < 1 || e.Source.Location.Column < 1) {
			t.Fatalf("missing source evidence: %+v", e)
		}
	}
	for _, s := range r.Sites {
		if s.Function != function {
			continue
		}
		for _, g := range r.Reverse {
			if g.Site == s.ID {
				return s, g, nodes, edges
			}
		}
	}
	t.Fatalf("no reverse graph for %s", function)
	return analyzer.Site{}, analyzer.ReverseGraph{}, nil, nil
}

func effectFixture(t *testing.T) string {
	t.Helper()
	dir := provenanceFixture(t)
	mustWrite(t, filepath.Join(dir, "diamond.go"), `package p
import "os"
func Left() { Read() }
func Right() { Read() }
func Diamond() { Left(); Right() }
func Independent() { _ = os.WriteFile("never", nil, 0600); _ = os.WriteFile("never", nil, 0600) }
func Closure() { fn := func() { Read() }; fn() }
`)
	return dir
}

func TestEffectViewPreservesAllCallerRelationships(t *testing.T) {
	dir := effectFixture(t)
	before := snapshot(t, dir)
	r, text := effectResult(t, dir, "-effect", "fs.read", "-max-depth", "20")
	if len(r.Unresolved) != 0 {
		t.Fatal("default operation view includes unresolved-only leads")
	}
	if len(r.Sites) != 2 || len(r.Groups) != 1 || len(r.Groups[0].Sites) != 2 {
		t.Fatalf("not canonical groups: %+v", r.Groups)
	}
	var established, candidate int
	for _, edge := range r.Edges {
		if edge.Candidate {
			candidate++
		} else {
			established++
		}
	}
	if r.Coverage.EstablishedEdges != established || r.Coverage.CandidateEdges != candidate {
		t.Fatalf("edge coverage mismatch: %+v edges=%+v", r.Coverage, r.Edges)
	}
	site, g, nodes, edges := reverseFor(t, r, "Read")
	if site.API != "os.ReadFile" || site.Location.Line != 3 || site.Location.Column != 33 || site.Recognition != "summary" {
		t.Fatalf("source identity: %+v", site)
	}
	if !g.Revisit || !g.Cycle || g.Truncated != (analyzer.GraphTruncation{}) {
		t.Fatalf("shared/cycle handling: %+v", g)
	}
	pairs := map[string]int{}
	for _, id := range g.Edges {
		e := edges[id]
		pairs[nodes[e.Caller].Function+"->"+nodes[e.Callee].Function]++
	}
	for pair, count := range map[string]int{"Middle->Read": 2, "Top->Middle": 1, "Left->Read": 1, "Right->Read": 1, "Diamond->Left": 1, "Diamond->Right": 1, "CycleA->CycleB": 1, "CycleB->CycleA": 1, "CycleB->Read": 1, "Closure->Closure$1": 1, "Closure$1->Read": 1} {
		if pairs[pair] != count {
			t.Fatalf("missing/duplicate %s: got %v", pair, pairs)
		}
	}
	refs := map[string]bool{}
	for _, ref := range g.Nodes {
		refs[ref.ID] = true
		if nodes[ref.ID].ID == "" {
			t.Fatalf("dangling node %s", ref.ID)
		}
	}
	for _, id := range g.Edges {
		e := edges[id]
		if !refs[e.Caller] || !refs[e.Callee] {
			t.Fatalf("edge outside site subgraph: %+v", e)
		}
	}
	if len(g.Origins) != 1 || nodes[g.Origins[0]].Function != "Read" {
		t.Fatalf("origins: %+v", g.Origins)
	}
	if _, again := effectResult(t, dir, "-effect", "fs.read", "-max-depth", "20"); again != text {
		t.Fatal("nondeterministic reverse graph")
	}
	if !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Fatal("analysis changed source")
	}
}

func TestEffectDepthLimitsAndSelectorsPreserveProvenance(t *testing.T) {
	dir := effectFixture(t)
	full, _ := effectResult(t, dir)
	site, _, _, _ := reverseFor(t, full, "Read")
	for _, depth := range []string{"0", "1", "2"} {
		r, _ := effectResult(t, dir, "-site", site.ID, "-max-depth", depth)
		if len(r.Sites) != 1 || r.Sites[0].ID != site.ID {
			t.Fatal("site selector changed identity")
		}
		_, g, nodes, _ := reverseFor(t, r, "Read")
		seen := map[string]int{}
		for _, ref := range g.Nodes {
			seen[nodes[ref.ID].Function] = ref.Depth
		}
		if depth == "0" && (len(g.Nodes) != 1 || len(g.Edges) != 0 || !g.Truncated.Depth) {
			t.Fatalf("depth zero: %+v", g)
		}
		if depth == "1" {
			if _, ok := seen["Top"]; ok {
				t.Fatal("grandcaller at depth one")
			}
			if seen["Middle"] != 1 {
				t.Fatal(seen)
			}
		}
		if depth == "2" && seen["Top"] != 2 {
			t.Fatal(seen)
		}
	}
	for _, args := range [][]string{{"-api", "os.ReadFile"}, {"-operation", "ReadFile"}, {"-package", "example.com/provenance", "-function", "Top"}} {
		r, _ := effectResult(t, dir, args...)
		found := false
		for _, s := range r.Sites {
			found = found || s.ID == site.ID
		}
		if !found || r.Coverage.SourceFunctions != full.Coverage.SourceFunctions || r.Coverage.OperationSites != full.Coverage.OperationSites {
			t.Fatalf("filter lost origin/inventory: %v %+v", args, r.Coverage)
		}
	}
	none, _ := effectResult(t, dir, "-api", "ReadFile", "-all", "-effect", "not-an-effect")
	if len(none.Sites) != 0 || len(none.Inventory) != full.Coverage.SourceFunctions || none.Coverage.OperationSites != full.Coverage.OperationSites {
		t.Fatalf("exact API/full inventory: %+v", none)
	}
	limited, _ := effectResult(t, dir, "-max-sites", "1")
	if len(limited.Sites) != 1 || !limited.Truncated.Sites || limited.Coverage.Truncated != limited.Truncated {
		t.Fatalf("site limit/coverage truncation: %+v %+v", limited.Truncated, limited.Coverage.Truncated)
	}
	limited, _ = effectResult(t, dir, "-site", site.ID, "-max-edges", "1")
	if len(limited.Edges) != 1 || !limited.Truncated.Edges || limited.Coverage.EstablishedEdges+limited.Coverage.CandidateEdges != len(limited.Edges) {
		t.Fatalf("edge limit/coverage: %+v", limited.Coverage)
	}
	unresolved, _ := effectResult(t, dir, "-unresolved")
	if len(unresolved.Groups) != 0 || len(unresolved.Sites) != 1 || unresolved.Sites[0].Function != "Open" || unresolved.Sites[0].Reasons[0] != "interface_dispatch" {
		t.Fatalf("unresolved leads: %+v", unresolved)
	}
	if len(unresolved.Edges) != 0 {
		t.Fatal("invented interface edge")
	}
	missing, _ := effectResult(t, dir, "-site", "site:absent")
	if len(missing.Sites) != 0 || missing.Coverage.OperationSites == 0 {
		t.Fatal("site filter damaged coverage")
	}
}

func TestMultiCategorySitesAreDefinedOnceAndTextIsReadable(t *testing.T) {
	dir := effectFixture(t)
	r, _ := effectResult(t, dir, "-function", "Independent")
	if len(r.Sites) != 2 || len(r.Groups) != 2 || !reflect.DeepEqual(r.Groups[0].Sites, r.Groups[1].Sites) {
		t.Fatalf("multi-category IDs: %+v", r.Groups)
	}
	if r.Sites[0].ID == r.Sites[1].ID || r.Sites[0].Location.Column == r.Sites[1].Location.Column {
		t.Fatal("independent operations merged")
	}
	out, diagnostic, status := invoke(t, dir, nil, "-function", "Diamond", "-max-depth", "3")
	if status != 0 || diagnostic != "" {
		t.Fatal(diagnostic)
	}
	for _, want := range []string{"fs.read (1 sites)", "os.ReadFile", "reverse callers:", ".Left", ".Right", ".Diamond", " -> ", "shared/revisited=true", "Coverage:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("unreadable/missing %q: %s", want, out)
		}
	}
	for _, args := range [][]string{{"-view", "invalid"}, {"-max-edges", "0"}, {"-view", "functions", "-api", "os.ReadFile"}, {"-details", "-view", "effects"}, {"-max-paths", "10"}, {"-details", "-max-edges", "10"}} {
		_, _, status := invoke(t, dir, nil, args...)
		if status != 2 {
			t.Fatalf("invalid flags %v: %d", args, status)
		}
	}
}

func TestEffectGraphDoesNotCrossGenericVariants(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/genericgraph\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "p.go"), `package p
import "os"
type readAction struct{}
func (readAction) Run() { _, _ = os.ReadFile("never") }
type envAction struct{}
func (envAction) Run() { _ = os.Setenv("never","never") }
func Apply[T interface{ Run() }](v T) { v.Run() }
func CallRead() { Apply(readAction{}) }
func CallEnv() { Apply(envAction{}) }
`)
	r, _ := effectResult(t, dir, "-max-depth", "10")
	for _, function := range []string{"(readAction).Run", "(envAction).Run"} {
		_, g, nodes, edges := reverseFor(t, r, function)
		// Walk actual node references, not source-symbol names. No incompatible
		// instantiation may be reached, even though the Apply summaries merge.
		seen := map[string]bool{}
		for _, id := range g.Origins {
			seen[id] = true
		}
		for changed := true; changed; {
			changed = false
			for _, id := range g.Edges {
				e := edges[id]
				if seen[e.Callee] && !seen[e.Caller] {
					seen[e.Caller] = true
					changed = true
				}
			}
		}
		found := false
		for id := range seen {
			name := nodes[id].Function
			if function == "(readAction).Run" && name == "CallEnv" || function == "(envAction).Run" && name == "CallRead" {
				t.Fatalf("fabricated path through merged generic symbol: %+v", g)
			}
			found = found || name == "CallRead" && function == "(readAction).Run" || name == "CallEnv" && function == "(envAction).Run"
		}
		if !found {
			t.Fatalf("missing actual generic path: %+v", g)
		}
	}
	filtered, _ := effectResult(t, dir, "-function", "CallRead")
	for _, s := range filtered.Sites {
		if s.Function == "(envAction).Run" {
			t.Fatal("function filter crossed generic variants")
		}
	}
}
