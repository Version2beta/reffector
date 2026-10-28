package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestQualifiedSyncOnceDoContract(t *testing.T) {
	source, err := filepath.Abs("../../testdata/sync-once")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "sync-once")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}

	report, output := effectResult(t, dir, "-all")
	repeated, repeatedOutput := effectResult(t, dir, "-all")
	if output != repeatedOutput || !reflect.DeepEqual(report.Sites, repeated.Sites) {
		t.Fatal("sync.Once.Do report is not deterministic")
	}
	if report.Coverage.OperationSites != 9 || report.Coverage.SelectedOperationSites != 9 {
		t.Fatalf("unique operation-site coverage: %+v", report.Coverage)
	}

	inventory := map[string]analyzer.Record{}
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	for _, function := range []string{"OnceDo", "OnceWithOpaqueCallback"} {
		record := inventory[function]
		if len(record.Effects) != 1 || record.Effects[0].Identifier != "concurrency.wait" || !record.Effects[0].Direct || !record.Uncertain {
			t.Errorf("%s effects = %+v; want direct once-wait effect with callback uncertainty", function, record)
		}
	}
	caller := inventory["OnceCaller"]
	if len(caller.Effects) != 1 || caller.Effects[0].Identifier != "concurrency.wait" || caller.Effects[0].Direct || !caller.Uncertain {
		t.Errorf("inherited Once.Do effect = %+v", caller)
	}
	if fake := inventory["FakeDo"]; len(fake.Effects) != 0 || fake.Uncertain {
		t.Errorf("same-named non-standard method was summarized: %+v", fake)
	}
	if callback := inventory["OnceWithOpaqueCallback$1"]; len(callback.Effects) != 2 || !hasEffect(callback, "fs.create") || !hasEffect(callback, "fs.write") || len(callback.PossibleEffects) != 0 {
		t.Errorf("callback body evidence = %+v; want its own direct filesystem effects only", callback)
	}
	if record := inventory["OnceWithOpaqueCallback"]; hasEffect(record, "fs.create") || hasEffect(record, "fs.write") {
		t.Errorf("opaque callback effects were inherited without a call edge: %+v", record)
	}

	var onceSites int
	for _, site := range report.Sites {
		for _, operation := range site.Operations {
			if operation.API != "sync.(*Once).Do" {
				continue
			}
			onceSites++
			if site.Kind != "call" || operation.Recognition != "summary" ||
				!reflect.DeepEqual(operation.Effects, []string{"concurrency.wait"}) ||
				!operation.Uncertain || !reflect.DeepEqual(operation.Reasons, []string{"callback"}) {
				t.Errorf("sync.Once.Do site evidence = %+v", site)
			}
		}
	}
	if onceSites != 2 {
		t.Errorf("qualified sync.Once.Do sites = %d, want 2", onceSites)
	}
}

func hasEffect(record analyzer.Record, identifier string) bool {
	for _, effect := range record.Effects {
		if effect.Identifier == identifier {
			return true
		}
	}
	return false
}
