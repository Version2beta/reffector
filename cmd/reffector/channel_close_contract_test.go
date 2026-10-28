package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestNativeChannelCloseOperation(t *testing.T) {
	source, err := filepath.Abs("../../testdata/channel-close")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "channel-close")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}

	report, output := effectResult(t, dir, "-all")
	repeated, repeatedOutput := effectResult(t, dir, "-all")
	if output != repeatedOutput || !reflect.DeepEqual(report.Sites, repeated.Sites) {
		t.Fatal("channel-close report is not deterministic")
	}
	if report.Coverage.OperationSites != 1 || report.Coverage.SelectedOperationSites != 1 {
		t.Fatalf("unique operation-site coverage: %+v", report.Coverage)
	}

	inventory := map[string]analyzer.Record{}
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	closed := inventory["Close"]
	if len(closed.Effects) != 1 || closed.Effects[0].Identifier != "channel.close" || !closed.Effects[0].Direct || closed.Uncertain {
		t.Errorf("exact close builtin summary = %+v", closed)
	}
	caller := inventory["Caller"]
	if len(caller.Effects) != 1 || caller.Effects[0].Identifier != "channel.close" || caller.Effects[0].Direct || caller.Uncertain {
		t.Errorf("inherited close summary = %+v", caller)
	}
	for _, name := range []string{"ShadowedClose", "NoClose"} {
		if record := inventory[name]; len(record.Effects) != 0 || record.Uncertain {
			t.Errorf("negative control %s gained close evidence: %+v", name, record)
		}
	}

	if len(report.Sites) != 1 {
		t.Fatalf("close operation sites = %+v", report.Sites)
	}
	site := report.Sites[0]
	if site.Function != "Close" || site.API != "builtin.close" || site.Kind != "call" ||
		site.Location.Line < 1 || site.Location.Column < 1 || site.Uncertain ||
		len(site.Operations) != 1 || site.Operations[0].Recognition != "ssa" ||
		!reflect.DeepEqual(site.Operations[0].Effects, []string{"channel.close"}) {
		t.Errorf("channel-close source-site evidence = %+v", site)
	}

	filtered, _ := effectResult(t, dir, "-effect", "channel.close", "-max-depth", "0")
	if len(filtered.Sites) != 1 || filtered.Sites[0].API != "builtin.close" || filtered.Sites[0].Function != "Close" {
		t.Errorf("channel.close effect filter = %+v", filtered.Sites)
	}
}
