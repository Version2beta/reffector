package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestQualifiedSyncMapOperationContracts(t *testing.T) {
	source, err := filepath.Abs("../../testdata/sync-map")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "sync-map")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}

	report, output := effectResult(t, dir, "-all")
	repeated, repeatedOutput := effectResult(t, dir, "-all")
	if output != repeatedOutput || !reflect.DeepEqual(report.Sites, repeated.Sites) {
		t.Fatal("sync.Map report is not deterministic")
	}
	if report.Coverage.OperationSites != 11 || report.Coverage.SelectedOperationSites != 11 || report.Coverage.UnresolvedSites != 0 {
		t.Fatalf("unique operation-site coverage: %+v", report.Coverage)
	}

	want := map[string]struct {
		effects   []string
		reasons   []string
		uncertain bool
	}{
		"sync.(*Map).Load":             {effects: []string{"concurrency.map.read"}, reasons: []string{}},
		"sync.(*Map).Store":            {effects: []string{"concurrency.map.write"}, reasons: []string{}},
		"sync.(*Map).Delete":           {effects: []string{"concurrency.map.write"}, reasons: []string{}},
		"sync.(*Map).Clear":            {effects: []string{"concurrency.map.write"}, reasons: []string{}},
		"sync.(*Map).LoadAndDelete":    {effects: []string{"concurrency.map.read", "concurrency.map.write"}, reasons: []string{"state_condition"}, uncertain: true},
		"sync.(*Map).LoadOrStore":      {effects: []string{"concurrency.map.read", "concurrency.map.write"}, reasons: []string{"state_condition"}, uncertain: true},
		"sync.(*Map).Swap":             {effects: []string{"concurrency.map.read", "concurrency.map.write"}, reasons: []string{}},
		"sync.(*Map).CompareAndSwap":   {effects: []string{"concurrency.map.read", "concurrency.map.write"}, reasons: []string{"state_condition"}, uncertain: true},
		"sync.(*Map).CompareAndDelete": {effects: []string{"concurrency.map.read", "concurrency.map.write"}, reasons: []string{"state_condition"}, uncertain: true},
		"sync.(*Map).Range":            {effects: []string{"concurrency.map.read"}, reasons: []string{"callback"}, uncertain: true},
	}
	sites := make(map[string]analyzer.Site)
	for _, site := range report.Sites {
		if site.Function == "MapOperations" {
			wantAPI, ok := want[site.API]
			if !ok {
				t.Errorf("unexpected sync.Map API %q", site.API)
				continue
			}
			if site.Kind != "call" || site.Location.Line < 1 || site.Location.Column < 1 || len(site.Operations) != 1 {
				t.Errorf("map operation provenance = %+v", site)
				continue
			}
			operation := site.Operations[0]
			if operation.API != site.API || operation.Recognition != "summary" ||
				!reflect.DeepEqual(operation.Effects, wantAPI.effects) ||
				!reflect.DeepEqual(operation.Reasons, wantAPI.reasons) || operation.Uncertain != wantAPI.uncertain || site.Uncertain != wantAPI.uncertain {
				t.Errorf("sync.Map evidence for %s = %+v, want effects=%v reasons=%v uncertain=%t", site.API, operation, wantAPI.effects, wantAPI.reasons, wantAPI.uncertain)
			}
			sites[site.API] = site
		}
	}
	if len(sites) != len(want) {
		t.Errorf("qualified sync.Map APIs = %v, want %v", sites, want)
	}

	for effect, count := range map[string]int{"concurrency.map.read": 7, "concurrency.map.write": 9} {
		filtered, _ := effectResult(t, dir, "-effect", effect, "-max-depth", "0")
		if len(filtered.Sites) != count {
			t.Errorf("%s filter returned %d unique sites, want %d", effect, len(filtered.Sites), count)
		}
	}

	inventory := make(map[string]analyzer.Record)
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	mapOperations := inventory["MapOperations"]
	if len(mapOperations.Effects) != 2 || mapOperations.Effects[0].Identifier != "concurrency.map.read" ||
		mapOperations.Effects[1].Identifier != "concurrency.map.write" || !mapOperations.Effects[0].Direct ||
		!mapOperations.Effects[1].Direct || !mapOperations.Uncertain {
		t.Errorf("direct map operation summary = %+v", mapOperations)
	}
	caller := inventory["Caller"]
	if len(caller.Effects) != 1 || caller.Effects[0].Identifier != "concurrency.map.write" ||
		caller.Effects[0].Direct || caller.Uncertain {
		t.Errorf("inherited map operation summary = %+v", caller)
	}
	for _, function := range []string{"SameNamedMethods", "PlainMapOperations"} {
		if record := inventory[function]; len(record.Effects) != 0 || record.Uncertain {
			t.Errorf("negative control %s has sync.Map evidence: %+v", function, record)
		}
	}
}
