package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestQualifiedSyncCondOperationContracts(t *testing.T) {
	source, err := filepath.Abs("../../testdata/sync-cond")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "sync-cond")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}

	report, output := effectResult(t, dir, "-all")
	repeated, repeatedOutput := effectResult(t, dir, "-all")
	if output != repeatedOutput || !reflect.DeepEqual(report.Sites, repeated.Sites) {
		t.Fatal("sync.Cond report is not deterministic")
	}
	if report.Coverage.OperationSites != 5 || report.Coverage.SelectedOperationSites != 5 {
		t.Fatalf("unique operation-site coverage: %+v", report.Coverage)
	}

	want := map[string]struct {
		effects []string
		reasons []string
	}{
		"sync.(*Cond).Wait":      {effects: []string{"concurrency.lock", "concurrency.unlock", "concurrency.wait"}, reasons: []string{"interface_dispatch"}},
		"sync.(*Cond).Signal":    {effects: []string{"concurrency.condition.signal"}, reasons: []string{"state_condition"}},
		"sync.(*Cond).Broadcast": {effects: []string{"concurrency.condition.broadcast"}, reasons: []string{"state_condition"}},
	}
	sites := make(map[string]analyzer.Site)
	for _, site := range report.Sites {
		if site.Function != "ConditionOperations" {
			continue
		}
		wantAPI, ok := want[site.API]
		if !ok {
			t.Errorf("unexpected sync.Cond API %q", site.API)
			continue
		}
		if site.Kind != "call" || site.Location.Line < 1 || site.Location.Column < 1 || len(site.Operations) != 1 {
			t.Errorf("sync.Cond operation provenance = %+v", site)
			continue
		}
		operation := site.Operations[0]
		if operation.API != site.API || operation.Recognition != "summary" ||
			!reflect.DeepEqual(operation.Effects, wantAPI.effects) ||
			!reflect.DeepEqual(operation.Reasons, wantAPI.reasons) || !operation.Uncertain || !site.Uncertain {
			t.Errorf("sync.Cond evidence for %s = %+v, want effects=%v reasons=%v uncertain=true", site.API, operation, wantAPI.effects, wantAPI.reasons)
		}
		sites[site.API] = site
	}
	if len(sites) != len(want) {
		t.Errorf("qualified sync.Cond APIs = %v, want %v", sites, want)
	}

	inventory := make(map[string]analyzer.Record)
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	conditionOperations := inventory["ConditionOperations"]
	wantFunctionEffects := []string{
		"concurrency.condition.broadcast",
		"concurrency.condition.signal",
		"concurrency.lock",
		"concurrency.unlock",
		"concurrency.wait",
	}
	if len(conditionOperations.Effects) != len(wantFunctionEffects) || !conditionOperations.Uncertain {
		t.Errorf("direct sync.Cond summary = %+v", conditionOperations)
	} else {
		for i, effect := range conditionOperations.Effects {
			if effect.Identifier != wantFunctionEffects[i] || !effect.Direct {
				t.Errorf("direct sync.Cond effects = %+v, want %v", conditionOperations.Effects, wantFunctionEffects)
				break
			}
		}
	}
	lockerWait := inventory["WaitWithOpaqueLocker"]
	if len(lockerWait.Effects) != 3 || !lockerWait.Uncertain ||
		!hasEffect(lockerWait, "concurrency.wait") || !hasEffect(lockerWait, "concurrency.lock") ||
		!hasEffect(lockerWait, "concurrency.unlock") || hasEffect(lockerWait, "fs.create") || hasEffect(lockerWait, "fs.write") {
		t.Errorf("custom Locker effects leaked through Cond.Wait or known wait evidence was lost: %+v", lockerWait)
	}
	lockerUnlock := inventory["(*OpaqueLocker).Unlock"]
	if !hasEffect(lockerUnlock, "fs.create") || !hasEffect(lockerUnlock, "fs.write") {
		t.Errorf("custom Locker negative-control effects = %+v", lockerUnlock)
	}
	caller := inventory["Caller"]
	if len(caller.Effects) != len(wantFunctionEffects) || !caller.Uncertain {
		t.Errorf("inherited sync.Cond summary = %+v", caller)
	} else {
		for i, effect := range caller.Effects {
			if effect.Identifier != wantFunctionEffects[i] || effect.Direct {
				t.Errorf("inherited sync.Cond effects = %+v, want %v", caller.Effects, wantFunctionEffects)
				break
			}
		}
	}
	for _, function := range []string{"SameNamedOperations", "OpenConditionOperations"} {
		record := inventory[function]
		if len(record.Effects) != 0 {
			t.Errorf("negative control %s gained sync.Cond effects: %+v", function, record)
		}
		if function == "SameNamedOperations" && record.Uncertain {
			t.Errorf("same-named concrete methods were treated as uncertain: %+v", record)
		}
		if function == "OpenConditionOperations" && !record.Uncertain {
			t.Errorf("open interface calls lost unresolved uncertainty: %+v", record)
		}
	}

	for effect, wantCount := range map[string]int{
		"concurrency.condition.signal":    1,
		"concurrency.condition.broadcast": 1,
		"concurrency.wait":                2,
	} {
		filtered, _ := effectResult(t, dir, "-effect", effect, "-max-depth", "0")
		if len(filtered.Sites) != wantCount {
			t.Errorf("%s filter returned %d unique sites, want %d", effect, len(filtered.Sites), wantCount)
		}
	}
}
