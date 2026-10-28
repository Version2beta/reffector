package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestQualifiedContextCancelAndProcessContracts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "effect-contracts")
	if err := os.CopyFS(dir, os.DirFS(effectContractsSource)); err != nil {
		t.Fatal(err)
	}
	report, _ := effectResult(t, dir, "-all")
	if report.Coverage.OperationSites != 11 || report.Coverage.SelectedOperationSites != 11 {
		t.Fatalf("operation-site coverage: %+v", report.Coverage)
	}

	inventory := map[string]analyzer.Record{}
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	for _, function := range []string{"CancelNow", "CancelDeferred", "CancelFromDeadline", "CancelFromDeadlineCause", "CancelFromTimeout", "CancelFromTimeoutCause", "CancelCauseNow"} {
		record := inventory[function]
		if len(record.Effects) != 1 || record.Effects[0].Identifier != "context.cancel" || !record.Effects[0].Direct || !record.Uncertain {
			t.Errorf("recognized %s result = %+v", function, record)
		}
	}
	process := inventory["OSProcessOperations"]
	if len(process.Effects) != 2 || process.Effects[0].Identifier != "process.control" || process.Effects[1].Identifier != "resource.release" || !process.Effects[0].Direct || !process.Effects[1].Direct || process.Uncertain {
		t.Errorf("os.Process operations = %+v", process)
	}
	if fake := inventory["FakeProcessOperations"]; len(fake.Effects) != 0 || fake.Uncertain {
		t.Errorf("same-named local process methods were summarized: %+v", fake)
	}
	for _, function := range []string{"FakeContextCancel", "FakeContextCancelCause"} {
		if fake := inventory[function]; len(fake.Effects) != 0 || fake.Uncertain {
			t.Errorf("arbitrary but locally known %s was summarized: %+v", function, fake)
		}
	}
	for _, function := range []string{"MixedContextCancel", "LocalCancellation"} {
		if record := inventory[function]; len(record.Effects) != 0 || !record.Uncertain {
			t.Errorf("unproven cancellation %s = %+v", function, record)
		}
	}

	cancelSites := map[string]analyzer.Site{}
	for _, site := range report.Sites {
		if site.API == "context.CancelFunc" || site.API == "context.CancelCauseFunc" {
			cancelSites[site.Function] = site
		}
	}
	wantAPIs := map[string]string{
		"CancelNow": "context.CancelFunc", "CancelDeferred": "context.CancelFunc",
		"CancelFromDeadline": "context.CancelFunc", "CancelFromDeadlineCause": "context.CancelFunc",
		"CancelFromTimeout": "context.CancelFunc", "CancelFromTimeoutCause": "context.CancelFunc",
		"CancelCauseNow": "context.CancelCauseFunc",
	}
	for function, api := range wantAPIs {
		site, ok := cancelSites[function]
		wantKind := "call"
		if function == "CancelDeferred" {
			wantKind = "defer"
		}
		if !ok || site.Kind != wantKind || site.Uncertain || len(site.Operations) != 1 || site.Operations[0].API != api {
			t.Errorf("qualified cancellation site %s = %+v", function, site)
		}
	}
}
