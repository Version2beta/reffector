package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestQualifiedSyncOnceReturnedFunctionContracts(t *testing.T) {
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
		t.Fatal("sync.Once returned-function report is not deterministic")
	}
	if report.Coverage.OperationSites != 9 || report.Coverage.SelectedOperationSites != 9 || report.Coverage.UnresolvedSites != 4 {
		t.Fatalf("unique recognized/unresolved operation-site coverage: %+v", report.Coverage)
	}

	inventory := map[string]analyzer.Record{}
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	for _, function := range []string{"OnceFuncResult", "OnceValueResult", "OnceValuesResult", "OnceFuncSameConstructorPhi", "OnceFuncWithCallbackEffect"} {
		record := inventory[function]
		if !hasEffect(record, "concurrency.wait") || !record.Uncertain {
			t.Errorf("proven once-wrapper invocation in %s = %+v", function, record)
		}
	}
	if record := inventory["OnceFuncConstructOnly"]; len(record.Effects) != 0 || record.Uncertain {
		t.Errorf("constructing without invoking a once wrapper gained wait evidence: %+v", record)
	}
	if record := inventory["OnceFuncWithCallbackEffect"]; len(record.Effects) != 1 || !hasEffect(record, "concurrency.wait") {
		t.Errorf("once-wrapper caller inherited callback body effects: %+v", record)
	}
	if record := inventory["OnceFuncWithCallbackEffect$1"]; !hasEffect(record, "fs.create") || !hasEffect(record, "fs.write") {
		t.Errorf("callback body lost its own filesystem operation: %+v", record)
	}
	for _, function := range []string{"FakeOnceFuncResult", "FakeNamedOnceFuncResult", "MixedOnceFuncResult", "HelperReturnedOnceFuncResult"} {
		record := inventory[function]
		if hasEffect(record, "concurrency.wait") || !record.Uncertain {
			t.Errorf("unproven once-wrapper invocation in %s = %+v", function, record)
		}
	}

	wantAPIByFunction := map[string]string{
		"OnceFuncResult":             "sync.OnceFunc",
		"OnceValueResult":            "sync.OnceValue",
		"OnceValuesResult":           "sync.OnceValues",
		"OnceFuncSameConstructorPhi": "sync.OnceFunc",
		"OnceFuncWithCallbackEffect": "sync.OnceFunc",
	}
	apiCounts := map[string]int{}
	for _, site := range report.Sites {
		if site.API == "sync.(*Once).Do" {
			continue
		}
		wantAPI, ok := wantAPIByFunction[site.Function]
		if !ok {
			continue
		}
		apiCounts[site.API]++
		if site.API != wantAPI || site.Kind != "call" || site.ID == "" ||
			site.Recognition != "summary" || !site.Uncertain ||
			!reflect.DeepEqual(site.Effects, []string{"concurrency.wait"}) ||
			!reflect.DeepEqual(site.Reasons, []string{"callback"}) ||
			site.Location.Line < 1 || site.Location.Column < 1 {
			t.Errorf("once-wrapper operation provenance = %+v; want %s with callback uncertainty", site, wantAPI)
		}
	}
	if !reflect.DeepEqual(apiCounts, map[string]int{"sync.OnceFunc": 3, "sync.OnceValue": 1, "sync.OnceValues": 1}) {
		t.Errorf("proven once-wrapper operation sites by API = %v", apiCounts)
	}

	unresolved, _ := effectResult(t, dir, "-all", "-unresolved")
	unresolvedByFunction := map[string]analyzer.Site{}
	for _, site := range unresolved.Sites {
		unresolvedByFunction[site.Function] = site
	}
	for _, function := range []string{"FakeOnceFuncResult", "FakeNamedOnceFuncResult", "MixedOnceFuncResult", "HelperReturnedOnceFuncResult"} {
		site, ok := unresolvedByFunction[function]
		if !ok || site.API != "" || !site.Uncertain || !reflect.DeepEqual(site.Reasons, []string{"function_value"}) {
			t.Errorf("unproven function-value site in %s = %+v", function, site)
		}
	}
}
