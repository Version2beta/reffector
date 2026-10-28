package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func copyFunctionProvenanceFixture(t *testing.T) string {
	t.Helper()
	source, err := filepath.Abs("../../testdata/function-provenance")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "provenance")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	return dir
}

func effectIdentifiers(r analyzer.Record) []string {
	out := []string{}
	for _, e := range r.Effects {
		out = append(out, e.Identifier)
	}
	return out
}

func TestFunctionProvenanceReturnsAndBoundaries(t *testing.T) {
	dir := copyFunctionProvenanceFixture(t)
	before := snapshot(t, dir)
	r, text := effectResult(t, dir, "-exact", "-all", "-max-depth", "12")
	_, repeat := effectResult(t, dir, "-exact", "-all", "-max-depth", "12")
	if repeat != text {
		t.Fatal("non-deterministic function provenance")
	}
	byName := inventoryByFunction(r)
	for _, tc := range []struct {
		name      string
		effects   []string
		uncertain bool
	}{
		{"DirectAssignment", []string{"fs.read"}, false},
		{"Factory", []string{"fs.read"}, false},
		{"Helpers", []string{"fs.read"}, false},
		{"Tuple", []string{"fs.read"}, false},
		{"GenericReturn", []string{"fs.read"}, false},
		{"NamedReturn", []string{"fs.read"}, false},
		{"Dependency", []string{"fs.create", "fs.write"}, false},
		{"OrdinaryCapture", []string{"fs.create", "fs.write"}, false},
		{"OrdinaryUnknownCapture", []string{"fs.create", "fs.write"}, false},
		{"SameCapture", []string{"fs.create", "fs.write"}, false},
		{"SamePhi", []string{"fs.read"}, false},
		{"Deferred", []string{"fs.read"}, false},
		{"Goroutine", []string{"concurrency.goroutine", "fs.read"}, false},
		{"ReturnedSummary", []string{"fs.remove"}, false},
		{"SingleSummary", []string{"fs.remove"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			x, ok := byName[tc.name]
			if !ok || !reflect.DeepEqual(effectIdentifiers(x), tc.effects) || x.Uncertain != tc.uncertain || len(x.PossibleEffects) != 0 {
				t.Errorf("got %+v, want established %v", x, tc.effects)
			}
		})
	}
	// Open roots, mutable storage and closure environments are not established
	// from an identity or from observing another caller's arguments.
	for _, name := range []string{"OpaqueReturned", "OpenReturned", "ClosedWithOpaqueBody", "Parameter", "DependencyInvoke", "External", "ExternalIdentity", "CapturedFunction", "CaptureFunction$1", "Aliased", "Struct", "Interface", "Global", "OpaqueCall", "Cyclic", "Recursive", "MixedExternal"} {
		x, ok := byName[name]
		if !ok || !x.Uncertain || len(x.Effects) != 0 {
			t.Errorf("open boundary %s gained certainty: %+v", name, x)
		}
	}
	if x := byName["DeferredFactoryResult"]; !x.Uncertain || !reflect.DeepEqual(effectIdentifiers(x), []string{"fs.read"}) {
		t.Errorf("defer result-cell loads must stay open, independent of factory effects: %+v", x)
	}
	if x := byName["Synchronous"]; !x.Uncertain || len(x.Effects) != 0 || !reflect.DeepEqual(x.PossibleEffects, []string{"fs.create", "fs.read", "fs.write"}) {
		t.Errorf("helper-parameter callback must remain unresolved with candidate effects: %+v", x)
	}
	// Passing, retaining or returning a callback does not execute it.
	for _, name := range []string{"Retained", "RetainedDependency", "ReturnOnly", "factory", "identity", "layer", "makeWriter", "CaptureFunction", "conditionalFactory"} {
		x, ok := byName[name]
		if !ok || x.Uncertain || len(x.Effects) != 0 || len(x.PossibleEffects) != 0 {
			t.Errorf("non-invocation %s gained effects: %+v", name, x)
		}
	}
	// Complete alternatives are possible targets, not an open boundary; nil
	// alternatives also forbid promotion. No common-effect guarantee is assumed.
	for _, name := range []string{"Conditional", "ConditionalReturn", "NilAlternative", "SummaryConditional"} {
		x, ok := byName[name]
		if !ok || x.Uncertain || len(x.Effects) != 0 || len(x.PossibleEffects) == 0 {
			t.Errorf("closed alternatives %s: %+v", name, x)
		}
	}
	if x := byName["SummaryOpen"]; !x.Uncertain || len(x.Effects) != 0 || !reflect.DeepEqual(x.PossibleEffects, []string{"fs.write"}) {
		t.Errorf("open summary: %+v", x)
	}
	if !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Fatal("analyzed source changed")
	}
}

func TestFunctionProvenanceInvocationSitesAndFilters(t *testing.T) {
	dir := copyFunctionProvenanceFixture(t)
	for _, tc := range []struct{ root, target, effect, kind, resolution string }{
		{"Factory", "read", "fs.read", "call", "established"},
		{"Helpers", "read", "fs.read", "call", "established"},
		{"Tuple", "read", "fs.read", "call", "established"},
		{"Dependency", "makeWriter$1", "fs.write", "call", "established"},
		{"Synchronous", "makeWriter$1", "fs.write", "call", "candidate_function_value"},
		{"Goroutine", "read", "fs.read", "go", "established"},
		{"Deferred", "read", "fs.read", "defer", "established"},
		{"ConditionalReturn", "read", "fs.read", "call", "candidate_function_value"},
		{"ConditionalGoroutine", "read", "fs.read", "go", "candidate_function_value"},
		{"ConditionalDeferred", "read", "fs.read", "defer", "candidate_function_value"},
		{"OpenReturned", "read", "fs.read", "call", "candidate_function_value"},
		{"MixedExternal", "read", "fs.read", "call", "candidate_function_value"},
	} {
		t.Run(tc.root, func(t *testing.T) {
			args := []string{"-all", "-function", "=" + tc.root, "-package", "example.com/provenance", "-effect", tc.effect, "-operation", "os.", "-max-depth", "1"}
			r, _ := effectResult(t, dir, append([]string{"-exact"}, args...)...)
			d := detailed(t, dir, append(append([]string{"-exact"}, args...), "-max-paths", "1")...)
			if len(r.Sites) != 1 || len(d.Details.Sites) != 1 || r.Sites[0].ID != d.Details.Sites[0].ID {
				t.Fatalf("views disagree: effects=%+v details=%+v", r.Sites, d.Details.Sites)
			}
			s := d.Details.Sites[0]
			if s.Function != tc.target {
				t.Fatalf("wrong operation target: %+v", s)
			}
			if tc.root == "Synchronous" {
				if len(s.Witnesses) != 0 {
					t.Fatalf("candidate callback was presented as established execution: %+v", s.Witnesses)
				}
				return
			}
			if tc.resolution == "candidate_function_value" {
				if len(s.Witnesses) != 1 || s.Witnesses[0].Root.Function != tc.root || len(d.Details.CallEdges) != 1 || d.Details.CallEdges[0].Resolution != tc.resolution {
					t.Fatalf("candidate callback evidence lost: %+v %+v", s.Witnesses, d.Details.CallEdges)
				}
				return
			}
			if len(s.Witnesses) != 1 || s.Witnesses[0].Root.Function != tc.root || len(s.Witnesses[0].Edges) != 1 {
				t.Fatalf("wrong true operation/witness: %+v", s)
			}
			if len(d.Details.CallEdges) != 1 {
				t.Fatalf("wrong path edges: %+v", d.Details.CallEdges)
			}
			e := d.Details.CallEdges[0]
			if e.Caller.Function != tc.root || e.Callee.Function != tc.target || e.Kind != tc.kind || e.Resolution != tc.resolution {
				t.Errorf("wrong invocation: %+v", e)
			}
			data, err := os.ReadFile(filepath.Join(dir, "corpus.go"))
			if err != nil {
				t.Fatal(err)
			}
			line := strings.Split(string(data), "\n")[e.Location.Line-1]
			if !strings.Contains(line, "f()") && !strings.Contains(line, "g()") {
				t.Errorf("edge is not actual invocation site: %s", line)
			}
			for _, edge := range r.Edges {
				if edge.Source == nil || edge.Source.Resolution != tc.resolution || edge.Candidate != (tc.resolution != "established") {
					t.Errorf("effect edge confidence: %+v", edge)
				}
			}
		})
	}
}

func TestFunctionProvenanceSummaryAlternativesArePossibleOnly(t *testing.T) {
	dir := copyFunctionProvenanceFixture(t)
	for _, name := range []string{"SummaryConditional", "SummaryOpen"} {
		args := []string{"-all", "-function", "=" + name, "-effect", "fs.write", "-max-depth", "0"}
		r, text := effectResult(t, dir, append([]string{"-exact"}, args...)...)
		d := detailed(t, dir, append(append([]string{"-exact"}, args...), "-max-paths", "1")...)
		if len(r.Sites) != 1 || len(d.Details.Sites) != 1 || r.Sites[0].ID != d.Details.Sites[0].ID {
			t.Fatalf("possible-only summary missing from views: %+v %+v", r.Sites, d.Details.Sites)
		}
		s := r.Sites[0]
		if s.Function != name || len(s.Effects) != 0 || !containsString(s.PossibleEffects, "fs.write") || s.Uncertain != (name == "SummaryOpen") {
			t.Errorf("summary candidate misrepresented: %+v", s)
		}
		for _, op := range s.Operations {
			if len(op.Effects) != 0 {
				t.Errorf("candidate summary promoted: %+v", op)
			}
		}
		if len(r.Groups) != 1 || len(r.Groups[0].Sites) != 0 || !reflect.DeepEqual(r.Groups[0].PossibleSites, []string{s.ID}) || r.Coverage.PossibleOperationSites != 2 {
			t.Errorf("candidate grouping/coverage: %+v %+v", r.Groups, r.Coverage)
		}
		if !strings.Contains(text, "\"possible_effects\"") || len(r.Edges) != 0 {
			t.Errorf("candidate API must stay at invocation, not fabricated declaration: %s", text)
		}
	}
	u, _ := effectResult(t, dir, "-exact", "-unresolved", "-function", "=SummaryOpen", "-reason", "function_value")
	if len(u.Sites) != 1 || !u.Sites[0].Uncertain || !containsString(u.Sites[0].Reasons, "function_value") {
		t.Fatalf("open summary boundary disappeared: %+v", u.Sites)
	}
	// New optional fields must be absent when unused, in both JSON projections.
	for _, detail := range []bool{false, true} {
		var text string
		if detail {
			out, _, code := invoke(t, dir, nil, "-exact", "-details", "-function", "=SingleSummary", "-effect", "fs.remove")
			if code != 0 {
				t.Fatal(code)
			}
			text = out
		} else {
			_, text = effectResult(t, dir, "-exact", "-function", "=SingleSummary", "-effect", "fs.remove")
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(text), &raw); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(text, "possible_effects") || strings.Contains(text, "possible_sites") || strings.Contains(text, "candidate_function_value") {
			t.Fatalf("unused additive evidence leaked: %s", text)
		}
	}
}

func TestFunctionProvenanceBoundsAndContextProjection(t *testing.T) {
	dir := copyFunctionProvenanceFixture(t)
	var source strings.Builder
	source.WriteString("package provenance\nfunc Deep() { f:=deep0(); f() }\n")
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&source, "func deep%d() func() { return deep%d() }\n", i, i+1)
	}
	source.WriteString("func deep80() func() { return read }\n")
	// One large factory exhausts the work budget even without recursion. The
	// extra ordinary work cannot be skipped to claim a closed return summary.
	source.WriteString("func wide() func() { n:=0\n")
	for i := 0; i < 5000; i++ {
		source.WriteString("n=work(n)\n")
	}
	source.WriteString("_ = n; return read }\nfunc work(n int) int { return n+1 }\nfunc Wide() { f:=wide(); f() }\n")
	mustWrite(t, filepath.Join(dir, "bounded.go"), source.String())
	r, _ := effectResult(t, dir, "-exact", "-all")
	for _, name := range []string{"Deep", "Wide"} {
		x := inventoryByFunction(r)[name]
		if !x.Uncertain || len(x.Effects) != 0 {
			t.Errorf("exhausted provenance budget claimed certainty: %+v", x)
		}
	}
	// Existing opt-in contextual VTA must traverse new established/candidate
	// function relationships without changing identity, kind, or root filters.
	for _, name := range []string{"Factory", "ConditionalReturn", "SummaryConditional"} {
		args := []string{"-exact", "-all", "-function", "=" + name, "-effect", "fs.", "-max-depth", "2", "-dependency-vta-functions", "10000", "-dependency-vta-edges", "10000"}
		e, text := effectResult(t, dir, args...)
		_, repeat := effectResult(t, dir, args...)
		if text != repeat {
			t.Fatal("contextual function provenance non-deterministic")
		}
		if e.Coverage.DependencyVTA.ContextTruncated {
			t.Fatal("unexpected contextual truncation")
		}
		text, diagnostic, status := invoke(t, dir, nil, append([]string{"-details", "-max-paths", "2"}, args...)...)
		if status != 0 {
			t.Fatalf("contextual details: %d %s", status, diagnostic)
		}
		var d analyzer.Report
		if err := json.Unmarshal([]byte(text), &d); err != nil {
			t.Fatal(err)
		}
		if len(e.Sites) == 0 || len(d.Details.Sites) == 0 {
			t.Fatalf("contextual projection dropped provenance: %s", name)
		}
		for _, site := range d.Details.Sites {
			for _, w := range site.Witnesses {
				if w.Root.Function != name {
					t.Errorf("unrequested contextual witness: %+v", w)
				}
			}
		}
	}
}
