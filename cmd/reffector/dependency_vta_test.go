package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func copyDependencyVTAFixture(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs("../../testdata/dependency-vta")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "dependency-vta")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "app")
}

func inventoryByFunction(report analyzer.EffectReport) map[string]analyzer.Record {
	inventory := make(map[string]analyzer.Record, len(report.Inventory))
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	return inventory
}

func TestDependencyVTAAddsCandidatesAndPreservesUnknownBoundaries(t *testing.T) {
	dir := copyDependencyVTAFixture(t)
	before := snapshot(t, filepath.Dir(dir))

	defaults, _ := effectResult(t, dir, "-all")
	optArgs := []string{"-all", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100"}
	optIn, optText := effectResult(t, dir, optArgs...)
	if _, repeated := effectResult(t, dir, optArgs...); repeated != optText {
		t.Fatal("dependency-VTA output changed between identical runs")
	}

	if defaults.Coverage.DependencyVTA != nil {
		t.Fatalf("default analysis unexpectedly expanded dependency VTA: %+v", defaults.Coverage.DependencyVTA)
	}
	stats := optIn.Coverage.DependencyVTA
	if stats == nil || stats.FunctionsAdded != 2 || stats.EdgesAdded != 3 || stats.Truncated || stats.LimitHit != "" {
		t.Fatalf("unbounded fixture expansion stats = %+v", stats)
	}
	if defaults.Coverage.OperationSites != optIn.Coverage.OperationSites || defaults.Coverage.UnresolvedSites != optIn.Coverage.UnresolvedSites {
		t.Fatalf("VTA changed site counts: default=%+v opt-in=%+v", defaults.Coverage, optIn.Coverage)
	}

	baseRecords, candidateRecords := inventoryByFunction(defaults), inventoryByFunction(optIn)
	for function, wantPossible := range map[string][]string{
		"Root":         {"fs.create", "fs.remove", "fs.write"},
		"OpenBoundary": {"fs.remove"},
	} {
		base := baseRecords[function]
		if len(base.Effects) != 0 || len(base.PossibleEffects) != 0 || !base.Uncertain {
			t.Errorf("default %s boundary = %+v; want unresolved without inferred effects", function, base)
		}
		candidate := candidateRecords[function]
		if len(candidate.Effects) != 0 || !candidate.Uncertain || !reflect.DeepEqual(candidate.PossibleEffects, wantPossible) {
			t.Errorf("opt-in %s evidence = %+v; want only possible %v and retained uncertainty", function, candidate, wantPossible)
		}
	}
	for _, function := range []string{"(Memory).Handle", "(Similar).Handle"} {
		if record := candidateRecords[function]; len(record.Effects) != 0 || len(record.PossibleEffects) != 0 || record.Uncertain {
			t.Errorf("uninstantiated negative-control %s gained evidence: %+v", function, record)
		}
	}

	candidateEdges := make(map[string]string)
	for _, edge := range optIn.Edges {
		if edge.Source == nil {
			continue
		}
		if edge.Source.Resolution == "candidate_vta" || edge.Source.Resolution == "candidate_function_value" {
			if !edge.Candidate {
				t.Errorf("candidate relationship not marked candidate: %+v", edge)
			}
			key := edge.Source.Caller.Package + "." + edge.Source.Caller.Function + "->" + edge.Source.Callee.Package + "." + edge.Source.Callee.Function
			candidateEdges[key] = edge.Source.Resolution
		} else if edge.Source.Resolution != "established" || edge.Candidate {
			t.Errorf("unexpected edge confidence: %+v", edge)
		}
	}
	wantCandidateEdges := map[string]string{
		"example.com/dependency.Dispatch->example.com/dependencyfixture.(Disk).Handle": "candidate_vta",
		"example.com/dependency.Callback->example.com/dependencyfixture.Root$1":        "candidate_function_value",
	}
	if !reflect.DeepEqual(candidateEdges, wantCandidateEdges) {
		t.Errorf("candidate edges = %v, want %v", candidateEdges, wantCandidateEdges)
	}

	defaultUnresolved, _ := effectResult(t, dir, "-all", "-unresolved")
	optUnresolved, _ := effectResult(t, dir, "-all", "-unresolved", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100")
	if !reflect.DeepEqual(defaultUnresolved.Unresolved, optUnresolved.Unresolved) || len(optUnresolved.Unresolved) != 2 {
		t.Fatalf("unresolved site IDs changed/disappeared: default=%v opt-in=%v", defaultUnresolved.Unresolved, optUnresolved.Unresolved)
	}
	unresolvedByFunction := make(map[string]analyzer.Site)
	for _, site := range optUnresolved.Sites {
		unresolvedByFunction[site.Function] = site
	}
	if site := unresolvedByFunction["Dispatch"]; !site.Uncertain || !containsString(site.Reasons, "interface_dispatch") {
		t.Errorf("dependency interface boundary lost uncertainty: %+v", site)
	}
	if site := unresolvedByFunction["Callback"]; !site.Uncertain || !containsString(site.Reasons, "function_value") {
		t.Errorf("dependency callback boundary lost uncertainty: %+v", site)
	}
	if !reflect.DeepEqual(before, snapshot(t, filepath.Dir(dir))) {
		t.Fatal("analysis modified the copied fixture")
	}
}

func TestDependencyVTABudgetBoundariesAreReported(t *testing.T) {
	dir := copyDependencyVTAFixture(t)
	cases := []struct {
		name, functions, edges, limit string
		functionBudget, edgeBudget    int
	}{
		{name: "function budget", functions: "1", edges: "100", limit: "functions", functionBudget: 1, edgeBudget: 100},
		{name: "edge budget", functions: "100", edges: "1", limit: "edges", functionBudget: 100, edgeBudget: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, _ := effectResult(t, dir, "-all", "-dependency-vta-functions", tc.functions, "-dependency-vta-edges", tc.edges)
			stats := report.Coverage.DependencyVTA
			if stats == nil || !stats.Truncated || stats.LimitHit != tc.limit {
				t.Fatalf("budget truncation = %+v, want limit %q", stats, tc.limit)
			}
			if stats.FunctionBudget != tc.functionBudget || stats.EdgeBudget != tc.edgeBudget || stats.FunctionsAdded > tc.functionBudget || stats.EdgesAdded > tc.edgeBudget {
				t.Fatalf("budget accounting exceeded or changed supplied limits: %+v", stats)
			}
			for _, function := range []string{"Root", "OpenBoundary"} {
				if record := inventoryByFunction(report)[function]; !record.Uncertain {
					t.Errorf("truncated analysis implied %s was resolved: %+v", function, record)
				}
			}
		})
	}
}

func TestDependencyVTARequiresBothPositiveBudgets(t *testing.T) {
	dir := copyDependencyVTAFixture(t)
	for _, args := range [][]string{
		{"-dependency-vta-functions", "1"},
		{"-dependency-vta-edges", "1"},
		{"-dependency-vta-functions", "0", "-dependency-vta-edges", "1"},
		{"-dependency-vta-functions", "1", "-dependency-vta-edges", "0"},
	} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			_, diagnostic, status := invoke(t, dir, nil, args...)
			if status != 2 || !strings.Contains(diagnostic, "requires both positive") {
				t.Fatalf("invalid budget args %v: exit=%d stderr=%q", args, status, diagnostic)
			}
		})
	}
}
