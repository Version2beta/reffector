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

func investigationFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"go.mod", "operations.go"} {
		data, err := os.ReadFile(filepath.Join("../../testdata/phase1", name))
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(dir, name), string(data))
	}
	return dir
}

func TestInvestigationInterfaceRelease(t *testing.T) {
	dir := investigationFixture(t)
	before := snapshot(t, dir)
	r, text := effectResult(t, dir, "-effect", "resource.release")
	want := map[string]string{
		"ConcreteClose": "os.(*File).Close", "InterfaceClose": "io.(Closer).Close",
		"StreamClose": "io.(Closer).Close", "BoundClose": "io.(Closer).Close",
		"EmbeddedClose": "io.(Closer).Close", "LockedClose": "io.(Closer).Close",
	}
	if len(r.Sites) != len(want) {
		t.Fatalf("release sites: %+v", r.Sites)
	}
	for _, s := range r.Sites {
		if s.API != want[s.Function] || len(s.Operations) != 1 || len(s.APIs) != 1 || s.Kind != "call" || filepath.Base(s.Location.File) != "operations.go" || s.Location.Line < 1 || s.Location.Column < 1 || !reflect.DeepEqual(s.Effects, []string{"resource.release"}) {
			t.Fatalf("contract/source identity: %+v", s)
		}
		// SSA reports the call's opening parenthesis, not the declaration.
		source, err := os.ReadFile(filepath.Join(dir, "operations.go"))
		if err != nil {
			t.Fatal(err)
		}
		line := strings.Split(string(source), "\n")[s.Location.Line-1]
		call := ".Close("
		if s.Function == "BoundClose" {
			call = "close("
		}
		if column := strings.Index(line, call) + len(call); column != s.Location.Column {
			t.Fatalf("location is not the direct call: %+v", s)
		}
		if s.Function == "ConcreteClose" {
			if s.Uncertain || s.Recognition != "summary" {
				t.Fatal(s)
			}
		} else if !s.Uncertain || s.Recognition != "interface_summary" || !reflect.DeepEqual(s.Reasons, []string{"interface_dispatch"}) {
			t.Fatalf("open implementation must remain unknown: %+v", s)
		}
	}
	for _, e := range r.Edges {
		if e.Source == nil || e.Source.Caller.Function != "CloseCaller" || e.Source.Callee.Function != "LockedClose" {
			t.Fatalf("invented implementation edge: %+v", e)
		}
	}
	_, g, nodes, _ := reverseFor(t, r, "LockedClose")
	if len(g.Nodes) != 2 || len(g.Edges) != 1 || nodes[g.Origins[0]].Function != "LockedClose" {
		t.Fatal(g)
	}
	all, _ := effectResult(t, dir, "-unresolved")
	found := false
	for _, s := range all.Sites {
		found = found || s.Function == "UnknownClose"
	}
	if !found {
		t.Fatal("structural Close lookalike must stay unresolved")
	}
	if _, again := effectResult(t, dir, "-effect", "resource.release"); again != text {
		t.Fatal("nondeterministic release report")
	}
	if !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Fatal("analyzed source changed")
	}
}

func TestInvestigationRenameTaxonomy(t *testing.T) {
	dir := investigationFixture(t)
	r, _ := effectResult(t, dir, "-effect", "rename")
	if len(r.Sites) != 2 || len(r.Groups) != 1 || r.Groups[0].Effect != "fs.rename" {
		t.Fatalf("rename discovery: %+v", r)
	}
	ids := map[string]string{}
	for _, s := range r.Sites {
		ids[s.Function] = s.ID
		if s.API != map[string]string{"Rename": "os.Rename", "RootRename": "os.(*Root).Rename"}[s.Function] || !reflect.DeepEqual(s.Effects, []string{"fs.create", "fs.remove", "fs.rename"}) || s.Uncertain {
			t.Fatal(s)
		}
	}
	for _, category := range []string{"fs.create", "fs.remove"} {
		legacy, _ := effectResult(t, dir, "-effect", category)
		for _, s := range legacy.Sites {
			if ids[s.Function] != s.ID {
				t.Fatal("taxonomy changed source identity")
			}
		}
		if len(legacy.Sites) != 2 {
			t.Fatal("lost legacy category")
		}
	}
	for _, args := range [][]string{{"-effect", "fs.write"}, {"-effect", "Rename"}, {"-api", "Rename"}, {"-effect", "rename", "-function", "=UnknownRename"}} {
		none, _ := effectResult(t, dir, args...)
		if len(none.Sites) != 0 || none.Coverage.OperationSites != r.Coverage.OperationSites {
			t.Fatalf("negative query %v: %+v", args, none)
		}
	}
}

func TestInvestigationBoundarySelection(t *testing.T) {
	dir := investigationFixture(t)
	all, _ := effectResult(t, dir, "-unresolved")
	filtered, text := effectResult(t, dir, "-unresolved", "-reason", "interface_dispatch")
	if len(filtered.Sites) >= len(all.Sites) || filtered.Coverage.OperationSites != all.Coverage.OperationSites || filtered.Coverage.UnresolvedSites != all.Coverage.UnresolvedSites {
		t.Fatal("reason filter must narrow, not rewrite coverage")
	}
	for _, s := range filtered.Sites {
		if s.API == "log.Print" || !reflect.DeepEqual(s.Reasons, []string{"interface_dispatch"}) {
			t.Fatal(s)
		}
	}
	api := "example.com/effectinvestigation.(Persistence).Save"
	r, _ := effectResult(t, dir, "-unresolved", "-reason", "interface_dispatch", "-api", api)
	if len(r.Sites) != 2 || len(r.Groups) != 0 || len(r.Unresolved) != 2 {
		t.Fatal(r)
	}
	s, g, nodes, edges := reverseFor(t, r, "Persist")
	if s.API != api || !s.Uncertain || s.Recognition != "unresolved" || len(s.Effects) != 0 || len(g.Origins) != 1 || len(g.Edges) != 2 {
		t.Fatalf("persistence boundary: %+v %+v", s, g)
	}
	callers := map[string]bool{}
	for _, id := range g.Edges {
		e := edges[id]
		if nodes[e.Callee].Function != "Persist" {
			t.Fatal("fabricated backend edge")
		}
		callers[nodes[e.Caller].Function] = true
	}
	if !callers["CallerA"] || !callers["CallerB"] || r.Sites[0].ID == r.Sites[1].ID {
		t.Fatal("shared vs independent sites lost")
	}
	one, _ := effectResult(t, dir, "-unresolved", "-site", s.ID, "-max-depth", "0")
	if len(one.Sites) != 1 || one.Sites[0].ID != s.ID || len(one.Edges) != 0 || !one.Truncated.Depth {
		t.Fatal(one)
	}
	limited, _ := effectResult(t, dir, "-unresolved", "-reason", "interface_dispatch", "-api", api, "-max-sites", "1")
	if len(limited.Sites) != 1 || !limited.Truncated.Sites || limited.Coverage.FilteredSites != 2 {
		t.Fatal(limited)
	}
	limited, _ = effectResult(t, dir, "-unresolved", "-site", s.ID, "-max-edges", "1")
	if !limited.Truncated.Edges || len(limited.Edges) != 1 {
		t.Fatal(limited)
	}
	for _, args := range [][]string{{"-unresolved", "-reason", "absent"}, {"-unresolved", "-reason", "Interface_dispatch"}, {"-unresolved", "-api", api, "-effect", "db.exec"}} {
		none, _ := effectResult(t, dir, args...)
		if len(none.Sites) != 0 {
			t.Fatal("unresolved is not a storage classification")
		}
	}
	known, _ := effectResult(t, dir, "-effect", "resource.release", "-reason", "interface_dispatch")
	for _, s := range known.Sites {
		if s.Function == "ConcreteClose" {
			t.Fatal("reason filter accepted resolved operation")
		}
	}
	if len(known.Sites) != 5 {
		t.Fatal("known contracts must retain dispatch filter")
	}
	if _, again := effectResult(t, dir, "-unresolved", "-reason", "interface_dispatch"); again != text {
		t.Fatal("nondeterministic boundary query")
	}
	for _, args := range [][]string{{"-view", "functions", "-reason", "interface_dispatch"}, {"-details", "-reason", "interface_dispatch"}} {
		_, err, status := invoke(t, dir, nil, args...)
		if status != 2 || !strings.Contains(err, "-reason") {
			t.Fatalf("invalid reason view: %d %s", status, err)
		}
	}
}

func TestInvestigationContractDoesNotDuplicateKnownTargets(t *testing.T) {
	dir := investigationFixture(t)
	mustWrite(t, filepath.Join(dir, "known.go"), `package investigation
import ("io"; "os")
func Known(f *os.File) error { var c io.Closer = f; return c.Close() }
`)
	r, _ := effectResult(t, dir, "-function", "=Known")
	if len(r.Sites) != 1 || len(r.Edges) != 0 {
		t.Fatalf("one source call, no synthetic concrete edge: %+v", r)
	}
	s := r.Sites[0]
	if s.API != "" || !reflect.DeepEqual(s.APIs, []string{"io.(Closer).Close", "os.(*File).Close"}) || s.Uncertain || len(s.Operations) != 2 || !reflect.DeepEqual(s.Effects, []string{"resource.release"}) {
		t.Fatalf("proven target plus contract at the same site: %+v", s)
	}
	var summaries []analyzer.Record
	if err := json.Unmarshal([]byte(success(t, dir, "-all", "-json")), &summaries); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, record := range summaries {
		switch record.Function {
		case "InterfaceClose", "CloseCaller":
			seen[record.Function] = true
			want := []analyzer.Effect{{Identifier: "resource.release", Direct: record.Function == "InterfaceClose"}}
			if record.Function == "CloseCaller" {
				want = []analyzer.Effect{{Identifier: "concurrency.lock"}, {Identifier: "concurrency.unlock"}, {Identifier: "resource.release"}}
			}
			if !record.Uncertain || !reflect.DeepEqual(record.Effects, want) {
				t.Fatalf("direct vs inherited contract: %+v", record)
			}
		case "Persist":
			seen[record.Function] = true
			if !record.Uncertain || len(record.Effects) != 0 {
				t.Fatalf("application contract is not a known storage operation: %+v", record)
			}
		}
	}
	if len(seen) != 3 {
		t.Fatal("missing compatible function summaries", seen)
	}
}
