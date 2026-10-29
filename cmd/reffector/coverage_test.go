package main

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestStatsLoadFailureDoesNotEmitPartialCoverage(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/broken\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "broken.go"), "package broken\nfunc Broken( {\n")
	out, diagnostic, status := invoke(t, dir, nil, "-stats", "-json", ".")
	if status == 0 || out != "" || !strings.Contains(diagnostic, "package loading failed") {
		t.Fatalf("load failure status=%d stdout=%q stderr=%q", status, out, diagnostic)
	}
}

func TestCoverageInventoryAndStatistics(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/coverage\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "p.go"), `package p
import "os"
var Callback = func() { }
func init() { }
func Pure() { }
func Leaf() { _ = os.Remove("never") }
func Top() { Leaf() }
func Generic[T any](v T) T { return v }
func Use() { _ = Generic(1); _ = Generic("a") }
`)
	mustWrite(t, filepath.Join(dir, "sub", "p.go"), "package sub\nfunc Pure() {}\n")
	mustWrite(t, filepath.Join(dir, "p_test.go"), "package p\nimport \"testing\"\nfunc TestOnly(t *testing.T) {}\n")
	before := snapshot(t, dir)
	out, diagnostic, status := invoke(t, dir, nil, "-view", "functions", "-all", "-stats", "-json")
	if status != 0 {
		t.Fatalf("%s", diagnostic)
	}
	var stats analyzer.Stats
	if err := json.Unmarshal([]byte(diagnostic), &stats); err != nil {
		t.Fatal(err)
	}
	want := analyzer.Stats{Packages: 2, SourceFunctions: 8, Emitted: 8, Direct: 1, Inherited: 1, Effectful: 2, Uncharacterized: 6}
	if stats.Packages != want.Packages || stats.SourceFunctions != want.SourceFunctions || stats.Emitted != want.Emitted || stats.Direct != want.Direct || stats.Inherited != want.Inherited || stats.Effectful != want.Effectful || stats.Uncharacterized != want.Uncharacterized {
		t.Fatalf("stats got %+v want counts %+v", stats, want)
	}
	if stats.HostOS == "" || stats.HostArch == "" || stats.BuildGOOS == "" || stats.BuildGOARCH == "" ||
		stats.PackageLoad != "successful" ||
		!reflect.DeepEqual(stats.SelectedRoots, []string{"example.com/coverage", "example.com/coverage/sub"}) || stats.MaxSites == 0 {
		t.Fatalf("missing observability metadata: %+v", stats)
	}
	if len(stats.SourceModules) != 1 || stats.SourceModules[0].Path != "example.com/coverage" ||
		stats.SourceRevisionStatus != "unavailable" || stats.SourceRevisionReason == "" ||
		stats.WholeSourceFunctionDenominator != "unavailable" || stats.FunctionDenominatorReason == "" {
		t.Fatalf("missing source identity/denominator limits: %+v", stats)
	}
	var summary map[string]any
	if err := json.Unmarshal([]byte(diagnostic), &summary); err != nil {
		t.Fatal(err)
	}
	if _, ok := summary["execution_duration_ms"]; !ok {
		t.Fatalf("stats duration missing: %s", diagnostic)
	}
	var inventory []analyzer.Record
	if err := json.Unmarshal([]byte(out), &inventory); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var raw []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatal(err)
	}
	for i, r := range inventory {
		key := r.Package + "." + r.Function
		if seen[key] || r.Function == "init" || strings.Contains(r.Function, "[int]") {
			t.Fatalf("synthetic/duplicate %+v", r)
		}
		seen[key] = true
		if len(raw[i]) != 4 || r.Effects == nil {
			t.Fatalf("ordinary JSON schema changed: %v", raw[i])
		}
	}
	recursive := success(t, dir, "-all", "-json", "./...")
	if out != recursive || out != success(t, dir, "-all", "-json", ".", "./sub") {
		t.Fatal("recursive or overlapping selection differs")
	}
	exact := success(t, dir, "-exact", "-all", "-json")
	if strings.Contains(exact, "example.com/coverage/sub") || exact != success(t, dir, "-all", "-json", "example.com/coverage") {
		t.Fatal("exact/import path selection differs")
	}
	if explicit := success(t, dir, "-exact", "-all", "-json", "./..."); explicit != out {
		t.Fatal("exact changed explicit pattern")
	}
	_, diagnostic, status = invoke(t, dir, nil, "-view", "functions", "-stats", "-json")
	if status != 0 {
		t.Fatal(diagnostic)
	}
	if err := json.Unmarshal([]byte(diagnostic), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.Emitted != 2 || stats.Omitted != 6 {
		t.Fatalf("omissions %+v", stats)
	}
	_, diagnostic, status = invoke(t, dir, nil, "-view", "functions", "-tests", "-all", "-stats", "-json")
	if status != 0 {
		t.Fatal(diagnostic)
	}
	if err := json.Unmarshal([]byte(diagnostic), &stats); err != nil {
		t.Fatal(err)
	}
	if stats.SourceFunctions != 9 || stats.Packages != 2 || !stats.TestsIncluded {
		t.Fatalf("test variants not merged: %+v", stats)
	}
	if !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Fatal("source changed")
	}
}
