package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func detailed(t *testing.T, dir string, args ...string) analyzer.Report {
	t.Helper()
	var report analyzer.Report
	text := success(t, dir, append([]string{"-details"}, args...)...)
	if err := json.Unmarshal([]byte(text), &report); err != nil || report.Details == nil {
		t.Fatalf("details: %v: %s", err, text)
	}
	return report
}

func provenanceFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/provenance\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "p.go"), `package p
import ("os"; "io")
func Read() { _, _ = os.ReadFile("never") }
func OtherRead() { _, _ = os.ReadFile("never") }
func Middle() { Read(); Read() }
func Top() { Middle() }
func CycleA() { CycleB() }
func CycleB() { CycleA(); Read() }
func TwoSites() { Read(); OtherRead() }
func Open(w io.Writer) { _, _ = w.Write(nil) }
type fake struct{}
func (fake) ReadFile() {}
func Fake() { fake{}.ReadFile() }
`)
	return dir
}

func TestArgumentProvenancePayloadCapExcludesCLIFramingNewline(t *testing.T) {
	dir := provenanceFixture(t)
	stdout, stderr, status := invoke(t, dir, nil, "-argument-provenance", "-argument-max-json-bytes", "10000")
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, stderr)
	}
	if !strings.HasSuffix(stdout, "\n") {
		t.Fatalf("missing CLI framing newline: %q", stdout)
	}
	payload := strings.TrimSuffix(stdout, "\n")
	if len(payload) > 10000 {
		t.Fatalf("payload bytes=%d exceeds cap", len(payload))
	}
	var decoded analyzer.ArgumentProvenance
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatal(err)
	}
	_, stderr, status = invoke(t, dir, nil, "-argument-provenance", "-argument-max-json-bytes", "1")
	if status != 1 || !strings.Contains(stderr, "minimum valid schema") {
		t.Fatalf("tiny cap status=%d stderr=%s", status, stderr)
	}
}

func TestDetailedSourceSitesAndWitnesses(t *testing.T) {
	dir := provenanceFixture(t)
	before := snapshot(t, dir)
	report := detailed(t, dir, "-all", "-effect", "fs.read")
	d := report.Details
	if len(d.Sites) != 2 {
		t.Fatalf("expected two actual sites, not copies per caller: %+v", d.Sites)
	}
	for _, site := range d.Sites {
		if site.Kind != "call" || site.API != "os.ReadFile" || site.Location.Line < 3 || site.Location.Line > 4 || site.Location.Column != map[string]int{"Read": 33, "OtherRead": 38}[site.Function] || filepath.Base(site.Location.File) != "p.go" || site.Recognition != "summary" || site.Uncertain || !reflect.DeepEqual(site.Effects, []string{"fs.read"}) {
			t.Fatalf("bad provenance %+v", site)
		}
	}
	var read analyzer.Site
	for _, s := range d.Sites {
		if s.Function == "Read" {
			read = s
		}
	}
	witnesses := map[string]analyzer.Witness{}
	edges := map[string]analyzer.CallEdge{}
	for _, e := range d.CallEdges {
		edges[e.ID] = e
		if strings.Contains(e.Callee.Function, "Write") {
			t.Fatal("invented interface target")
		}
	}
	for _, w := range read.Witnesses {
		witnesses[w.Root.Function] = w
	}
	for name, depth := range map[string]int{"Read": 0, "Middle": 1, "Top": 2, "CycleA": 2, "CycleB": 1, "TwoSites": 1} {
		w, ok := witnesses[name]
		if !ok || len(w.Edges) != depth || w.Direct != (depth == 0) {
			t.Fatalf("%s: %+v", name, w)
		}
		current := w.Root
		for _, id := range w.Edges {
			edge, ok := edges[id]
			if !ok || edge.Caller != current || edge.Location.Line < 1 {
				t.Fatalf("broken witness: %+v", w)
			}
			current = edge.Callee
		}
		if current != read.Symbol {
			t.Fatalf("path does not reach site: %+v", w)
		}
	}
	if d.Truncated != (analyzer.Truncation{}) {
		t.Fatalf("cycle should terminate without truncation: %+v", d.Truncated)
	}
	onlyTop := detailed(t, dir, "-function", "Top", "-operation", "os.ReadFile")
	if len(onlyTop.Records) != 1 || len(onlyTop.Details.Sites) != 1 || onlyTop.Details.Sites[0].ID != read.ID || len(onlyTop.Details.Sites[0].Witnesses) != 1 {
		t.Fatalf("filter/site identity: %+v", onlyTop)
	}
	if got := success(t, dir, "-details", "-all"); got != success(t, dir, "-details", "-all") {
		t.Fatal("nondeterministic detail JSON")
	}
	if got := detailed(t, dir, "-tests", "-function", "Top"); got.Details.Sites[0].ID != read.ID {
		t.Fatal("test variant changed site identity")
	}
	// Two source call expressions in Middle point to one actual ReadFile site;
	// only one deterministic shortest path per root/site is emitted.
	if len(witnesses["Middle"].Edges) != 1 {
		t.Fatal("duplicate shared-caller path")
	}
	for _, site := range detailed(t, dir, "-direct", "-effect", "fs.read").Details.Sites {
		if len(site.Witnesses) != 1 || !site.Witnesses[0].Direct {
			t.Fatalf("inherited detail in direct mode: %+v", site)
		}
	}
	relocated := t.TempDir()
	if err := os.CopyFS(relocated, os.DirFS(dir)); err != nil {
		t.Fatal(err)
	}
	if got := detailed(t, relocated, "-function", "Top"); got.Details.Sites[0].ID != read.ID {
		t.Fatal("site identity depends on checkout directory")
	}
	if !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Fatal("detailed analysis changed source")
	}
}

func TestDetailedBoundsAndFilters(t *testing.T) {
	dir := provenanceFixture(t)
	shallow := detailed(t, dir, "-function", "Top", "-max-depth", "1")
	if !shallow.Details.Truncated.Depth {
		t.Fatalf("missing depth truncation %+v", shallow.Details)
	}
	for _, s := range shallow.Details.Sites {
		for _, w := range s.Witnesses {
			if len(w.Edges) > 1 {
				t.Fatal("depth exceeded")
			}
		}
	}
	limited := detailed(t, dir, "-effect", "fs.read", "-max-paths", "1", "-max-sites", "1")
	if !limited.Details.Truncated.Paths || !limited.Details.Truncated.Sites {
		t.Fatalf("missing truncation: %+v", limited.Details)
	}
	if len(limited.Details.Sites) != 1 || len(limited.Details.Sites[0].Witnesses) > 1 {
		t.Fatal("limits exceeded")
	}
	filtered := detailed(t, dir, "-effect", "net.")
	if len(filtered.Records) != 0 || len(filtered.Details.Sites) != 0 {
		t.Fatal("effect filtering failed")
	}
	if got := success(t, dir, "-json", "-package", "not-a-package"); strings.TrimSpace(got) != "[]" {
		t.Fatal(got)
	}
	for _, args := range [][]string{{"-max-depth", "-1"}, {"-max-paths", "0"}, {"-operation", "os.ReadFile"}} {
		_, _, status := invoke(t, dir, nil, append([]string{"-view", "functions"}, args...)...)
		if status != 2 {
			t.Fatalf("invalid args %v: %d", args, status)
		}
	}
}
