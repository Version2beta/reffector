package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestQualifiedContextAfterFuncContracts(t *testing.T) {
	source, err := filepath.Abs("../../testdata/context-afterfunc")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "context-afterfunc")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	report, output := effectResult(t, dir, "-all")
	repeated, repeatedOutput := effectResult(t, dir, "-all")
	if output != repeatedOutput || !reflect.DeepEqual(report.Sites, repeated.Sites) {
		t.Fatal("context.AfterFunc report is not deterministic")
	}
	if report.Coverage.OperationSites != 12 || report.Coverage.SelectedOperationSites != 12 {
		t.Fatalf("unique recognized operation-site coverage: %+v", report.Coverage)
	}

	inventory := map[string]analyzer.Record{}
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	contains := func(record analyzer.Record, effect string) bool {
		for _, got := range record.Effects {
			if got.Identifier == effect {
				return true
			}
		}
		return false
	}
	for _, function := range []string{"Schedule", "StopNow", "StopDeferred", "ReturnStop", "MixedStop", "CallbackEffect"} {
		if record := inventory[function]; !contains(record, "concurrency.callback.register") || !record.Uncertain {
			t.Errorf("AfterFunc callback scheduling evidence in %s = %+v", function, record)
		}
	}
	for _, function := range []string{"StopNow", "StopDeferred"} {
		if record := inventory[function]; !contains(record, "concurrency.callback.stop") || !record.Uncertain {
			t.Errorf("AfterFunc stop evidence in %s = %+v", function, record)
		}
	}
	for _, function := range []string{"CallReturnedStop", "MixedStop", "InvokeArbitraryStop"} {
		record := inventory[function]
		if contains(record, "concurrency.callback.stop") || !record.Uncertain {
			t.Errorf("unproven stop function in %s was promoted or lost uncertainty: %+v", function, record)
		}
	}
	if record := inventory["CallReturnedStop"]; len(record.Effects) != 1 ||
		record.Effects[0].Identifier != "concurrency.callback.register" || record.Effects[0].Direct {
		t.Errorf("helper's known registration should remain inherited while its returned stop stays unresolved: %+v", record)
	}
	if record := inventory["LocalArbitraryStop"]; contains(record, "concurrency.callback.stop") || record.Uncertain {
		t.Errorf("known local arbitrary function was summarized: %+v", record)
	}
	if record := inventory["NilContext"]; contains(record, "concurrency.callback.register") || record.Uncertain {
		t.Errorf("statically nil context was reported as scheduling a callback: %+v", record)
	}
	if record := inventory["FakeAfterFunc"]; contains(record, "concurrency.callback.register") || contains(record, "concurrency.callback.stop") {
		t.Errorf("same-named application method was summarized: %+v", record)
	}
	if record := inventory["CallbackEffect"]; contains(record, "channel.close") {
		t.Errorf("callback body effect was inherited across the AfterFunc boundary: %+v", record)
	}

	seen := map[string]map[string]analyzer.Site{}
	apiCounts := map[string]int{}
	for _, site := range report.Sites {
		for _, operation := range site.Operations {
			apiCounts[operation.API]++
			if operation.API != "context.AfterFunc" && operation.API != "context.AfterFunc.stop" {
				continue
			}
			if site.Kind != "call" && !(site.Function == "StopDeferred" && site.Kind == "defer") {
				t.Errorf("unexpected AfterFunc call form: %+v", site)
			}
			wantEffect, wantReason := "concurrency.callback.register", "callback"
			if operation.API == "context.AfterFunc.stop" {
				wantEffect, wantReason = "concurrency.callback.stop", "state_condition"
			}
			if site.API != operation.API || operation.Recognition != "summary" ||
				!reflect.DeepEqual(operation.Effects, []string{wantEffect}) || !operation.Uncertain ||
				!reflect.DeepEqual(operation.Reasons, []string{wantReason}) ||
				!reflect.DeepEqual(site.Reasons, []string{wantReason}) || site.Location.Line < 1 || site.Location.Column < 1 {
				t.Errorf("AfterFunc operation evidence = %+v", site)
			}
			if seen[site.Function] == nil {
				seen[site.Function] = make(map[string]analyzer.Site)
			}
			seen[site.Function][operation.API] = site
		}
	}
	wantFunctions := map[string]map[string]bool{
		"Schedule":       {"context.AfterFunc": true},
		"StopNow":        {"context.AfterFunc": true, "context.AfterFunc.stop": true},
		"StopDeferred":   {"context.AfterFunc": true, "context.AfterFunc.stop": true},
		"ReturnStop":     {"context.AfterFunc": true},
		"MixedStop":      {"context.AfterFunc": true},
		"PhiStops":       {"context.AfterFunc": true, "context.AfterFunc.stop": true},
		"CallbackEffect": {"context.AfterFunc": true},
	}
	if apiCounts["context.AfterFunc"] != 8 || apiCounts["context.AfterFunc.stop"] != 3 || apiCounts["builtin.close"] != 1 {
		t.Errorf("unique operation-site counts by API = %v", apiCounts)
	}
	for function, apis := range wantFunctions {
		if len(seen[function]) != len(apis) {
			t.Errorf("AfterFunc operation APIs in %s = %v, want %v", function, seen[function], apis)
			continue
		}
		for api := range apis {
			if _, ok := seen[function][api]; !ok {
				t.Errorf("missing %s operation in %s: %v", api, function, seen[function])
			}
		}
	}

	unresolved, _ := effectResult(t, dir, "-all", "-unresolved")
	unresolvedFunctions := map[string]bool{}
	for _, site := range unresolved.Sites {
		unresolvedFunctions[site.Function] = true
	}
	for _, function := range []string{"CallReturnedStop", "MixedStop", "InvokeArbitraryStop"} {
		if !unresolvedFunctions[function] {
			t.Errorf("expected unresolved function-value site in %s: %v", function, unresolvedFunctions)
		}
	}
	if unresolvedFunctions["FakeAfterFunc"] || inventory["FakeAfterFunc"].Uncertain {
		t.Error("known application factory no-op should resolve without lifecycle effects")
	}
	if unresolvedFunctions["NilContext"] {
		t.Errorf("statically nil context produced an unresolved operation: %v", unresolvedFunctions)
	}

	filtered, _ := effectResult(t, dir, "-effect", "concurrency.callback.stop")
	if len(filtered.Sites) != 3 {
		t.Fatalf("callback-stop effect filter selected %d sites, want 3", len(filtered.Sites))
	}
	for _, site := range filtered.Sites {
		if len(site.Operations) != 1 || site.Operations[0].API != "context.AfterFunc.stop" {
			t.Errorf("callback-stop filter returned unrelated operation: %+v", site)
		}
	}
}
