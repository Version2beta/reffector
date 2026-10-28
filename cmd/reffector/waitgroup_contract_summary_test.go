package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func copyWaitGroupVTAFixture(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs("../../testdata/waitgroup-vta")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "waitgroup-vta")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, "app")
}

func TestQualifiedWaitGroupContracts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "waitgroup")
	if err := os.CopyFS(dir, os.DirFS(waitGroupSource)); err != nil {
		t.Fatal(err)
	}
	report, output := effectResult(t, dir, "-all")
	repeated, repeatedOutput := effectResult(t, dir, "-all")
	if output != repeatedOutput || !reflect.DeepEqual(report.Sites, repeated.Sites) {
		t.Fatal("WaitGroup operation report is not deterministic")
	}
	if report.Coverage.OperationSites != 4 || report.Coverage.SelectedOperationSites != 4 {
		t.Fatalf("unique operation-site coverage: %+v", report.Coverage)
	}
	if report.Coverage.UnresolvedSites != 1 {
		t.Fatalf("unresolved sites: %+v", report.Coverage)
	}

	inventory := map[string]analyzer.Record{}
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	wantEffects := map[string][]string{
		"AddDoneWait": {"concurrency.task", "concurrency.wait"},
		"StartWithGo": {"concurrency.goroutine", "concurrency.task"},
		"Caller":      {"concurrency.task", "concurrency.wait"},
	}
	for function, want := range wantEffects {
		record := inventory[function]
		got := make([]string, len(record.Effects))
		for i, effect := range record.Effects {
			got[i] = effect.Identifier
			if function == "Caller" && effect.Direct {
				t.Errorf("%s unexpectedly has direct effect %s", function, effect.Identifier)
			}
			if function != "Caller" && !effect.Direct {
				t.Errorf("%s lost direct effect %s", function, effect.Identifier)
			}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s effects = %v, want %v", function, got, want)
		}
	}
	if !inventory["StartWithGo"].Uncertain {
		t.Error("WaitGroup.Go callback opacity was not retained")
	}
	for _, function := range []string{"FakeOperations"} {
		if record := inventory[function]; len(record.Effects) != 0 || record.Uncertain {
			t.Errorf("same-named methods on another type were summarized: %+v", record)
		}
	}
	if record := inventory["OpenAdd"]; len(record.Effects) != 0 || !record.Uncertain {
		t.Errorf("open Add interface lost uncertainty or gained a WaitGroup effect: %+v", record)
	}

	wantAPIs := map[string]string{
		"sync.(*WaitGroup).Add":  "concurrency.task",
		"sync.(*WaitGroup).Done": "concurrency.task",
		"sync.(*WaitGroup).Wait": "concurrency.wait",
		"sync.(*WaitGroup).Go":   "concurrency.goroutine",
	}
	seen := map[string]bool{}
	for _, site := range report.Sites {
		if !site.Uncertain && len(site.Effects) == 0 {
			t.Errorf("empty operation site: %+v", site)
		}
		for _, operation := range site.Operations {
			wantEffect, ok := wantAPIs[operation.API]
			if !ok {
				continue
			}
			seen[operation.API] = true
			if site.Location.Line < 1 || site.Location.Column < 1 || site.Kind != "call" || operation.Recognition != "summary" {
				t.Errorf("missing qualified source evidence: %+v", site)
			}
			if operation.API == "sync.(*WaitGroup).Go" {
				if !operation.Uncertain ||
					!reflect.DeepEqual(operation.Reasons, []string{"callback"}) ||
					!reflect.DeepEqual(operation.Effects, []string{"concurrency.goroutine", "concurrency.task"}) {
					t.Errorf("WaitGroup.Go evidence = %+v", operation)
				}
			} else if operation.Uncertain {
				t.Errorf("unexpected uncertainty on %s: %+v", operation.API, operation)
			}
			if operation.API != "sync.(*WaitGroup).Go" && !reflect.DeepEqual(operation.Effects, []string{wantEffect}) {
				t.Errorf("%s effects = %v, want %s", operation.API, operation.Effects, wantEffect)
			}
		}
	}
	if len(seen) != len(wantAPIs) {
		t.Errorf("qualified WaitGroup APIs seen = %v", seen)
	}
}

func TestDependencyVTAContextsStayRootLocalAcrossSelectedPackages(t *testing.T) {
	dir := copyWaitGroupVTAFixture(t)
	defaults, _ := effectResult(t, dir, "-all", "-exact", "./...")
	args := []string{"-all", "-exact", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100", "./..."}
	optIn, output := effectResult(t, dir, args...)
	repeated, repeatedOutput := effectResult(t, dir, args...)
	if output != repeatedOutput || !reflect.DeepEqual(optIn.Sites, repeated.Sites) {
		t.Fatal("multi-package candidate-context output changed between identical runs")
	}
	if defaults.Coverage.OperationSites != 3 || optIn.Coverage.OperationSites != 3 || defaults.Coverage.UnresolvedSites != 1 || optIn.Coverage.UnresolvedSites != 1 {
		t.Fatalf("shared dependency changed unique site coverage: default=%+v opt-in=%+v", defaults.Coverage, optIn.Coverage)
	}
	defaultUnresolved, _ := effectResult(t, dir, "-all", "-exact", "-unresolved", "./...")
	optUnresolved, _ := effectResult(t, dir, "-all", "-exact", "-unresolved", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100", "./...")
	if !reflect.DeepEqual(defaultUnresolved.Unresolved, optUnresolved.Unresolved) || len(optUnresolved.Unresolved) != 1 {
		t.Fatalf("shared dependency changed unresolved origins: default=%v opt-in=%v", defaultUnresolved.Unresolved, optUnresolved.Unresolved)
	}

	for _, test := range []struct {
		packageSuffix string
		function      string
		possible      []string
	}{
		{packageSuffix: "example.com/waitgroupfixture", function: "Root", possible: []string{"concurrency.task", "concurrency.wait"}},
		{packageSuffix: "example.com/waitgroupfixture/other", function: "EmptyRoot", possible: []string{}},
		{packageSuffix: "example.com/waitgroupfixture/other", function: "WaiterRoot", possible: []string{"concurrency.task", "concurrency.wait"}},
	} {
		var record analyzer.Record
		found := false
		for _, candidate := range optIn.Inventory {
			if candidate.Package == test.packageSuffix && candidate.Function == test.function {
				record, found = candidate, true
				break
			}
		}
		if !found {
			t.Errorf("missing selected root %s.%s", test.packageSuffix, test.function)
			continue
		}
		if len(record.Effects) != 0 || !record.Uncertain || len(record.PossibleEffects) != len(test.possible) || (len(test.possible) > 0 && !reflect.DeepEqual(record.PossibleEffects, test.possible)) {
			t.Errorf("root %s.%s evidence = %+v; want possible %v only, with uncertainty retained", test.packageSuffix, test.function, record, test.possible)
		}
	}
	stats := optIn.Coverage.DependencyVTA
	if stats == nil || stats.ContextTruncated || stats.ContextRootsFallback != 0 || stats.ContextStates == 0 || stats.ContextStates > stats.ContextBudget {
		t.Errorf("multi-package candidate-context budget evidence = %+v", stats)
	}
	for _, edge := range optIn.Edges {
		if edge.Source == nil || edge.Source.Caller.Package != "example.com/dependency" || edge.Source.Caller.Function != "Dispatch" || edge.Source.Callee.Package != "example.com/waitgroupfixture" || edge.Source.Callee.Function != "(Waiter).Handle" {
			continue
		}
		if edge.Source.Resolution != "candidate_vta" || !edge.Candidate {
			t.Fatalf("shared dependency implementation was promoted from candidate: %+v", edge)
		}
		return
	}
	t.Fatal("shared dependency Waiter implementation candidate edge was not retained")
}

func TestWaitGroupEffectsStayPossibleAcrossVTAEdges(t *testing.T) {
	dir := copyWaitGroupVTAFixture(t)
	defaults, _ := effectResult(t, dir, "-all", "-exact")
	args := []string{"-all", "-exact", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100"}
	optIn, optText := effectResult(t, dir, args...)
	if _, repeated := effectResult(t, dir, args...); repeated != optText {
		t.Fatal("contextual dependency-VTA output changed between identical runs")
	}
	if defaults.Coverage.OperationSites != 3 || optIn.Coverage.OperationSites != 3 || defaults.Coverage.SelectedOperationSites != 3 || optIn.Coverage.SelectedOperationSites != 3 {
		t.Fatalf("VTA changed unique operation-site counts: default=%+v opt-in=%+v", defaults.Coverage, optIn.Coverage)
	}
	if defaults.Coverage.UnresolvedSites != 1 || optIn.Coverage.UnresolvedSites != 1 {
		t.Fatalf("VTA changed unresolved-site count: default=%+v opt-in=%+v", defaults.Coverage, optIn.Coverage)
	}
	baseRecords, candidateRecords := map[string]analyzer.Record{}, map[string]analyzer.Record{}
	for _, record := range defaults.Inventory {
		baseRecords[record.Function] = record
	}
	for _, record := range optIn.Inventory {
		candidateRecords[record.Function] = record
	}
	for _, function := range []string{"Root", "forward", "OpenBoundary", "UnknownGlobal", "MixedBoundary"} {
		if record := baseRecords[function]; len(record.Effects) != 0 || len(record.PossibleEffects) != 0 || !record.Uncertain {
			t.Errorf("default %s boundary = %+v", function, record)
		}
		if record := candidateRecords[function]; len(record.Effects) != 0 || !reflect.DeepEqual(record.PossibleEffects, []string{"concurrency.task", "concurrency.wait"}) || !record.Uncertain {
			t.Errorf("VTA %s evidence = %+v; want candidate-only WaitGroup effects and retained uncertainty", function, record)
		}
	}
	closed := candidateRecords["ClosedBoundary"]
	if len(closed.Effects) != 0 || len(closed.PossibleEffects) != 0 || !closed.Uncertain {
		t.Errorf("proven Empty-only caller gained candidate effects or lost unresolved dispatch: %+v", closed)
	}
	if stats := optIn.Coverage.DependencyVTA; stats == nil || stats.ContextTruncated || stats.ContextRootsFallback != 0 || stats.ContextStates == 0 || stats.ContextStates > stats.ContextBudget {
		t.Errorf("candidate-context budget evidence = %+v", stats)
	}
	if record := candidateRecords["(Waiter).Handle"]; !reflect.DeepEqual(record.Effects, []analyzer.Effect{{Identifier: "concurrency.task", Direct: true}, {Identifier: "concurrency.wait", Direct: true}}) {
		t.Errorf("Waiter.Handle direct effects = %+v", record)
	}
	var foundCandidate bool
	for _, edge := range optIn.Edges {
		if edge.Source == nil || edge.Source.Caller.Package != "example.com/dependency" || edge.Source.Caller.Function != "Dispatch" || edge.Source.Callee.Package != "example.com/waitgroupfixture" || edge.Source.Callee.Function != "(Waiter).Handle" {
			continue
		}
		foundCandidate = edge.Source.Resolution == "candidate_vta" && edge.Candidate
	}
	if !foundCandidate {
		t.Fatal("WaitGroup implementation was not retained as a candidate-only VTA edge")
	}
	closedSites, _ := effectResult(t, dir, "-all", "-exact", "-function", "=ClosedBoundary", "-effect", "concurrency.wait", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100")
	if len(closedSites.Sites) != 0 || closedSites.Coverage.OperationSites != 3 || closedSites.Coverage.UnresolvedSites != 1 {
		t.Fatalf("ClosedBoundary site view should retain coverage but exclude the incompatible Waiter path: sites=%+v coverage=%+v", closedSites.Sites, closedSites.Coverage)
	}
	rootSites, _ := effectResult(t, dir, "-all", "-exact", "-function", "=Root", "-effect", "concurrency.wait", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100")
	if len(rootSites.Sites) != 1 || rootSites.Sites[0].Function != "(Waiter).Handle" {
		t.Fatalf("Root did not retain the compatible candidate operation site: %+v", rootSites.Sites)
	}
	allSiteView, _ := effectResult(t, dir, "-all", "-exact", "-effect", "concurrency.wait", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100")
	sawWaiterReverse := false
	for _, reverse := range allSiteView.Reverse {
		if reverse.Site != allSiteView.Sites[0].ID {
			continue
		}
		sawWaiterReverse = true
		byID := map[string]analyzer.GraphNode{}
		for _, node := range allSiteView.Nodes {
			byID[node.ID] = node
		}
		for _, ref := range reverse.Nodes {
			if byID[ref.ID].Function == "ClosedBoundary" {
				t.Fatal("incompatible ClosedBoundary was retained as a caller witness to Waiter.Handle")
			}
		}
		byIDEdge := map[string]analyzer.GraphEdge{}
		for _, edge := range allSiteView.Edges {
			byIDEdge[edge.ID] = edge
		}
		mixedLines := []int{}
		for _, edgeID := range reverse.Edges {
			edge := byIDEdge[edgeID]
			if edge.Source != nil && edge.Source.Caller.Function == "MixedBoundary" && edge.Source.Callee.Function == "forward" {
				mixedLines = append(mixedLines, edge.Source.Location.Line)
			}
		}
		sort.Ints(mixedLines)
		if !reflect.DeepEqual(mixedLines, []int{36, 37}) {
			t.Fatalf("MixedBoundary reverse graph should retain both Waiter argument calls and exclude Empty, got lines %v", mixedLines)
		}
	}
	if !sawWaiterReverse {
		t.Fatal("Waiter.Handle reverse graph was not emitted")
	}
	detailText, diagnostic, status := invoke(t, dir, nil, "-details", "-all", "-exact", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100")
	if status != 0 || !strings.Contains(diagnostic, "context_states=") || !strings.Contains(diagnostic, "fallback_roots=0") {
		t.Fatalf("details run/context statistics: exit=%d stderr=%q", status, diagnostic)
	}
	var detailReport analyzer.Report
	if err := json.Unmarshal([]byte(detailText), &detailReport); err != nil {
		t.Fatalf("details JSON: %v", err)
	}
	detailEdges := map[string]analyzer.CallEdge{}
	for _, edge := range detailReport.Details.CallEdges {
		detailEdges[edge.ID] = edge
	}
	foundMixedWitness := false
	for _, site := range detailReport.Details.Sites {
		if site.Function != "(Waiter).Handle" {
			continue
		}
		for _, witness := range site.Witnesses {
			if witness.Root.Function == "ClosedBoundary" {
				t.Fatal("incompatible ClosedBoundary was retained as a function-view witness to Waiter.Handle")
			}
			if witness.Root.Function == "MixedBoundary" {
				foundMixedWitness = true
				mixedLines := []int{}
				for _, edgeID := range witness.Edges {
					edge := detailEdges[edgeID]
					if edge.Caller.Function == "MixedBoundary" && edge.Callee.Function == "forward" {
						mixedLines = append(mixedLines, edge.Location.Line)
					}
				}
				if !reflect.DeepEqual(mixedLines, []int{36}) {
					t.Fatalf("MixedBoundary witness should use only the Waiter argument call, got lines %v", mixedLines)
				}
			}
		}
	}
	if !foundMixedWitness {
		t.Fatal("MixedBoundary's compatible path to Waiter.Handle was not retained")
	}
	limited, _ := effectResult(t, dir, "-all", "-exact", "-dependency-vta-functions", "1", "-dependency-vta-edges", "1")
	limitedStats := limited.Coverage.DependencyVTA
	if limitedStats == nil || !limitedStats.ContextTruncated || limitedStats.ContextRootsFallback == 0 || limitedStats.ContextStates != limitedStats.ContextBudget {
		t.Fatalf("truncated context refinement was not reported: %+v", limitedStats)
	}
	limitedRecords := map[string]analyzer.Record{}
	for _, record := range limited.Inventory {
		limitedRecords[record.Function] = record
	}
	if record := limitedRecords["ClosedBoundary"]; !reflect.DeepEqual(record.PossibleEffects, []string{"concurrency.task", "concurrency.wait"}) || !record.Uncertain {
		t.Fatalf("truncated context analysis must fall back to conservative VTA effects: %+v", record)
	}
	baseUnresolved, _ := effectResult(t, dir, "-all", "-exact", "-unresolved")
	candidateUnresolved, _ := effectResult(t, dir, "-all", "-exact", "-unresolved", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100")
	if !reflect.DeepEqual(baseUnresolved.Unresolved, candidateUnresolved.Unresolved) || len(candidateUnresolved.Unresolved) != 1 {
		t.Fatalf("VTA changed unresolved IDs: default=%v opt-in=%v", baseUnresolved.Unresolved, candidateUnresolved.Unresolved)
	}
}
