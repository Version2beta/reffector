package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestQualifiedSyncAtomicContracts(t *testing.T) {
	source, err := filepath.Abs("../../testdata/sync-atomic")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "sync-atomic")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}

	report, output := effectResult(t, dir, "-all")
	repeated, repeatedOutput := effectResult(t, dir, "-all")
	if output != repeatedOutput || !reflect.DeepEqual(report.Sites, repeated.Sites) {
		t.Fatal("sync/atomic report is not deterministic")
	}
	if report.Coverage.OperationSites != 18 || report.Coverage.SelectedOperationSites != 18 || report.Coverage.UnresolvedSites != 0 {
		t.Fatalf("unique operation-site coverage: %+v", report.Coverage)
	}

	wantAPIs := map[string]bool{
		"sync/atomic.(*Bool).Store": true, "sync/atomic.(*Bool).Load": true,
		"sync/atomic.(*Bool).Swap": true, "sync/atomic.(*Bool).CompareAndSwap": true,
		"sync/atomic.(*Int32).Add": true, "sync/atomic.(*Int32).Store": true,
		"sync/atomic.(*Int32).Load": true, "sync/atomic.(*Int32).Swap": true,
		"sync/atomic.(*Int32).CompareAndSwap": true, "sync/atomic.(*Int32).And": true,
		"sync/atomic.(*Int32).Or": true, "sync/atomic.(*Int64).Add": true,
		"sync/atomic.(*Int64).Load": true, "sync/atomic.LoadInt32": true,
		"sync/atomic.(*Pointer[T]).Load": true, "sync/atomic.(*Pointer[T]).Store": true,
		"sync/atomic.(*Pointer[T]).CompareAndSwap": true, "sync/atomic.(*Value).Load": true,
	}
	sites := make(map[string]analyzer.Site)
	for _, site := range report.Sites {
		if site.Function != "AtomicOperations" {
			t.Errorf("unexpected atomic site in %s: %+v", site.Function, site)
			continue
		}
		if !wantAPIs[site.API] {
			t.Errorf("unqualified or unexpected API %q", site.API)
		}
		if site.Kind != "call" || site.Location.Line < 1 || site.Location.Column < 1 || site.Uncertain ||
			len(site.Operations) != 1 || site.Operations[0].Recognition != "summary" ||
			!reflect.DeepEqual(site.Operations[0].Effects, []string{"concurrency.atomic"}) {
			t.Errorf("atomic operation evidence = %+v", site)
		}
		sites[site.API] = site
	}
	if len(sites) != len(wantAPIs) {
		t.Errorf("qualified atomic APIs = %v, want %v", sites, wantAPIs)
	}
	filtered, _ := effectResult(t, dir, "-effect", "concurrency.atomic", "-max-depth", "0")
	if len(filtered.Sites) != 18 {
		t.Errorf("atomic effect filter returned %d sites, want 18", len(filtered.Sites))
	}

	inventory := make(map[string]analyzer.Record)
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	atomicOperations := inventory["AtomicOperations"]
	if len(atomicOperations.Effects) != 1 || atomicOperations.Effects[0].Identifier != "concurrency.atomic" ||
		!atomicOperations.Effects[0].Direct || atomicOperations.Uncertain {
		t.Errorf("atomic operation summary = %+v", atomicOperations)
	}
	caller := inventory["Caller"]
	if len(caller.Effects) != 1 || caller.Effects[0].Identifier != "concurrency.atomic" ||
		caller.Effects[0].Direct || caller.Uncertain {
		t.Errorf("inherited atomic operation summary = %+v", caller)
	}
	for _, function := range []string{"SameNamedMethod", "LocalMutation"} {
		if record := inventory[function]; len(record.Effects) != 0 || record.Uncertain {
			t.Errorf("negative control %s has atomic evidence: %+v", function, record)
		}
	}
}
