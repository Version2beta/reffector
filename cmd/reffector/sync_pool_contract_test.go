package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestQualifiedSyncPoolOperationContracts(t *testing.T) {
	source, err := filepath.Abs("../../testdata/sync-pool")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "sync-pool")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}

	report, output := effectResult(t, dir, "-all")
	repeated, repeatedOutput := effectResult(t, dir, "-all")
	if output != repeatedOutput || !reflect.DeepEqual(report.Sites, repeated.Sites) {
		t.Fatal("sync.Pool report is not deterministic")
	}
	if report.Coverage.OperationSites != 6 || report.Coverage.SelectedOperationSites != 6 || report.Coverage.UnresolvedSites != 0 {
		t.Fatalf("unique operation-site coverage: %+v", report.Coverage)
	}

	counts := map[string]int{"sync.(*Pool).Get": 0, "sync.(*Pool).Put": 0}
	deferredPut := false
	for _, site := range report.Sites {
		if site.API == "os.WriteFile" {
			continue
		}
		if _, ok := counts[site.API]; !ok {
			t.Errorf("unexpected sync.Pool site API %q", site.API)
			continue
		}
		counts[site.API]++
		want := []string{"concurrency.pool.get"}
		wantReasons := []string{"callback"}
		wantUncertain := true
		if site.API == "sync.(*Pool).Put" {
			want = []string{"concurrency.pool.put"}
			wantReasons = []string{}
			wantUncertain = false
		}
		if len(site.Operations) != 1 {
			t.Errorf("sync.Pool operation provenance = %+v", site)
			continue
		}
		operation := site.Operations[0]
		if operation.API != site.API || operation.Recognition != "summary" ||
			!reflect.DeepEqual(operation.Effects, want) ||
			!reflect.DeepEqual(operation.Reasons, wantReasons) || operation.Uncertain != wantUncertain || site.Uncertain != wantUncertain {
			t.Errorf("sync.Pool evidence for %s = %+v, want effects=%v reasons=%v uncertain=%t", site.API, operation, want, wantReasons, wantUncertain)
		}
		if site.API == "sync.(*Pool).Put" && site.Kind == "defer" {
			deferredPut = true
		}
	}
	if counts["sync.(*Pool).Get"] != 2 || counts["sync.(*Pool).Put"] != 3 {
		t.Errorf("qualified sync.Pool operations = %v, want two Get and three Put sites", counts)
	}
	if !deferredPut {
		t.Error("deferred Pool.Put did not retain its invocation kind")
	}

	for effect, count := range map[string]int{"concurrency.pool.get": 2, "concurrency.pool.put": 3} {
		filtered, _ := effectResult(t, dir, "-effect", effect, "-max-depth", "0")
		if len(filtered.Sites) != count {
			t.Errorf("%s filter returned %d unique sites, want %d", effect, len(filtered.Sites), count)
		}
	}

	inventory := make(map[string]analyzer.Record)
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	operations := inventory["PoolOperations"]
	if len(operations.Effects) != 2 || operations.Effects[0].Identifier != "concurrency.pool.get" ||
		operations.Effects[1].Identifier != "concurrency.pool.put" || !operations.Effects[0].Direct ||
		!operations.Effects[1].Direct || !operations.Uncertain {
		t.Errorf("direct Pool operation summary = %+v", operations)
	}
	callback := inventory["PoolWithNewCallback"]
	if len(callback.Effects) != 1 || callback.Effects[0].Identifier != "concurrency.pool.get" || !callback.Effects[0].Direct || !callback.Uncertain {
		t.Errorf("Pool.Get callback boundary summary = %+v", callback)
	}
	caller := inventory["Caller"]
	if len(caller.Effects) != 1 || caller.Effects[0].Identifier != "concurrency.pool.put" ||
		caller.Effects[0].Direct || caller.Uncertain {
		t.Errorf("inherited Pool.Put summary = %+v", caller)
	}
	for _, function := range []string{"SameNamedMethods", "PoolWithNewCallback$1"} {
		record := inventory[function]
		if function == "PoolWithNewCallback$1" {
			if len(record.Effects) != 2 || record.Effects[0].Identifier != "fs.create" || record.Effects[1].Identifier != "fs.write" {
				t.Errorf("callback body operation summary = %+v", record)
			}
			continue
		}
		if len(record.Effects) != 0 || record.Uncertain {
			t.Errorf("negative control %s has sync.Pool evidence: %+v", function, record)
		}
	}
}
