package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func copyOpaqueDependencyVTAFixture(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs("../../testdata/dependency-vta-opaque")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "dependency-vta-opaque")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "app")
}

func TestDependencyVTAOpaqueReturnsCapturesAndBudgetRemainExplicit(t *testing.T) {
	dir := copyOpaqueDependencyVTAFixture(t)
	before := snapshot(t, filepath.Dir(dir))
	defaults, _ := effectResult(t, dir, "-all", "-exact")
	args := []string{"-all", "-exact", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100"}
	complete, output := effectResult(t, dir, args...)
	if _, repeated := effectResult(t, dir, args...); repeated != output {
		t.Fatal("opaque-flow dependency-VTA output changed between identical runs")
	}
	if defaults.Coverage.OperationSites != 3 || complete.Coverage.OperationSites != 3 || defaults.Coverage.UnresolvedSites != 3 || complete.Coverage.UnresolvedSites != 3 {
		t.Fatalf("candidate analysis changed unique operation/unresolved counts: default=%+v opt-in=%+v", defaults.Coverage, complete.Coverage)
	}
	stats := complete.Coverage.DependencyVTA
	if stats == nil || stats.Truncated || stats.ContextTruncated || stats.ContextRootsFallback != 0 || stats.ContextStates == 0 || stats.ContextStates > stats.ContextBudget {
		t.Fatalf("complete VTA/context budgets = %+v", stats)
	}

	inventory := inventoryByFunction(complete)
	returned := inventory["ReturnBoundary"]
	if len(returned.Effects) != 0 || !returned.Uncertain || !reflect.DeepEqual(returned.PossibleEffects, []string{"concurrency.task", "concurrency.wait"}) {
		t.Errorf("opaque returned Empty was promoted or uncertainty lost: %+v", returned)
	}
	closed := inventory["ClosedBoundary"]
	if len(closed.Effects) != 0 || len(closed.PossibleEffects) != 0 || !closed.Uncertain {
		t.Errorf("direct Empty negative control gained Waiter effects or lost dispatch uncertainty: %+v", closed)
	}
	for _, function := range []string{"CaptureBoundary", "CaptureBoundary$1"} {
		if record := inventory[function]; len(record.Effects) != 0 || !record.Uncertain {
			t.Errorf("captured Empty dispatch was treated as established or resolved in %s: %+v", function, record)
		}
	}

	var candidateDispatch bool
	for _, edge := range complete.Edges {
		if edge.Source == nil || edge.Source.Caller.Package != "example.com/dependency" || edge.Source.Caller.Function != "Dispatch" || edge.Source.Callee.Package != "example.com/opaquefixture" || edge.Source.Callee.Function != "(Waiter).Handle" {
			continue
		}
		candidateDispatch = edge.Candidate && edge.Source.Resolution == "candidate_vta"
	}
	if !candidateDispatch {
		t.Fatal("Waiter dispatch must remain a candidate-only VTA relationship")
	}

	baseUnresolved, _ := effectResult(t, dir, "-all", "-exact", "-unresolved")
	completeUnresolved, _ := effectResult(t, dir, "-all", "-exact", "-unresolved", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100")
	if !reflect.DeepEqual(baseUnresolved.Unresolved, completeUnresolved.Unresolved) || len(completeUnresolved.Unresolved) != 3 {
		t.Fatalf("complete VTA changed unresolved IDs: default=%v opt-in=%v", baseUnresolved.Unresolved, completeUnresolved.Unresolved)
	}
	unresolvedByFunction := map[string]analyzer.Site{}
	for _, site := range completeUnresolved.Sites {
		unresolvedByFunction[site.Function] = site
	}
	for function, reason := range map[string]string{
		"Dispatch":          "interface_dispatch",
		"CaptureBoundary$1": "interface_dispatch",
	} {
		site, ok := unresolvedByFunction[function]
		if !ok || !site.Uncertain || !containsString(site.Reasons, reason) {
			t.Errorf("opaque %s boundary lost %s uncertainty: %+v", function, reason, site)
		}
	}

	limited, _ := effectResult(t, dir, "-all", "-exact", "-dependency-vta-functions", "1", "-dependency-vta-edges", "1")
	limitedStats := limited.Coverage.DependencyVTA
	if limitedStats == nil || !limitedStats.Truncated || !limitedStats.ContextTruncated || limitedStats.ContextRootsFallback == 0 || limitedStats.ContextStates != limitedStats.ContextBudget {
		t.Fatalf("combined dependency/context truncation was not explicit: %+v", limitedStats)
	}
	if limited.Coverage.OperationSites != complete.Coverage.OperationSites || limited.Coverage.UnresolvedSites != complete.Coverage.UnresolvedSites {
		t.Fatalf("truncated VTA changed operation/unresolved counts: complete=%+v limited=%+v", complete.Coverage, limited.Coverage)
	}
	limitedUnresolved, _ := effectResult(t, dir, "-all", "-exact", "-unresolved", "-dependency-vta-functions", "1", "-dependency-vta-edges", "1")
	if !reflect.DeepEqual(baseUnresolved.Unresolved, limitedUnresolved.Unresolved) {
		t.Fatalf("truncated VTA changed unresolved IDs: default=%v limited=%v", baseUnresolved.Unresolved, limitedUnresolved.Unresolved)
	}
	if !reflect.DeepEqual(before, snapshot(t, filepath.Dir(dir))) {
		t.Fatal("analysis modified the copied opaque-flow fixture")
	}
}
