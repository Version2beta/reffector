package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func argumentFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/provenance\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	src := `package fixture
import ("os"; "os/exec"; "io/ioutil")
func helper(p string) string { return string([]byte(p)) }
func writer(p string) { _ = os.WriteFile(helper(p), []byte("data"), 0600) }
func first() { writer("one") }
func second() { writer("two") }
func run(c *exec.Cmd) { _ = c.Run() }
func runOutput(c *exec.Cmd) { _, _ = c.Output(); _, _ = c.CombinedOutput() }
func branch(b bool) { p := "left"; if b { p = "right" }; _ = os.WriteFile(p, nil, 0600) }
func transform(p string) string { return string([]byte(p)) }
func transformed(p string) { _ = os.WriteFile(transform(p), nil, 0600) }
func transformFirst() { transformed("first") }
func transformSecond() { transformed("second") }
func pair() (string, string) { return "chosen", "excluded" }
func tuple() { a, _ := pair(); _ = os.WriteFile(a, nil, 0600) }
func globalBoundary() { _ = os.WriteFile(globalPath, nil, 0600) }
var globalPath = "global"
func heapBoundary(p string) { x := &p; _ = os.WriteFile(*x, nil, 0600) }
func closureBoundary(p string) { defer func() { _ = os.Remove(p) }() }
type mutablePath struct { path string }
func mutableFieldBoundary(state *mutablePath) { _ = os.Remove(state.path) }
func externalBoundary(p string) string { return external(p) }
func external(string) string
func recursivePhi(b bool) { p := "cycle"; for b { p = p }; _ = os.WriteFile(p, nil, 0600) }
func candidateRemove(b bool) { f := os.Remove; if b { f = os.RemoveAll }; _ = f("candidate") }
func localRemove(p string) error { return os.Remove(p) }
func localRemoveAll(p string) error { return os.RemoveAll(p) }
func candidateLocalWrapper(b bool) { f := localRemove; if b { f = localRemoveAll }; _ = f("wrapper") }
type argvParts struct { arg string }
func aggregateArguments() {
 parts := argvParts{arg: "struct-field"}
 appended := append([]string{"slice-literal"}, "append-value")
 _, _ = os.StartProcess("program", appended, nil)
 _, _ = os.StartProcess("program", []string{parts.arg}, nil)
 made := make([]string, 2)
 _, _ = os.StartProcess("program", made, nil)
}
func allOperations(root *os.Root) {
 _ = os.WriteFile("a", nil, 0600); _ = ioutil.WriteFile("b", nil, 0600)
 _ = os.Remove("c"); _ = os.Rename("d", "e"); _, _ = os.StartProcess("f", []string{"f"}, nil)
 _, _ = os.OpenInRoot("g", "h"); _ = root.Rename("i", "j"); _ = root.WriteFile("k", nil, 0600)
 _ = os.Link("l1", "l2"); _ = os.Symlink("s1", "s2"); _ = root.Link("r1", "r2"); _ = root.Symlink("t1", "t2")
}
`
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(src), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}
func TestArgumentProvenanceFixtureAndStableReferences(t *testing.T) {
	dir := argumentFixture(t)
	run := func() []byte {
		r, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, ArgumentProvenance: true})
		if err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(r.ArgumentProvenance)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	first, second := run(), run()
	if string(first) != string(second) {
		i := 0
		for i < len(first) && i < len(second) && first[i] == second[i] {
			i++
		}
		t.Fatalf("projection is nondeterministic at %d: %s <> %s", i, first[max(0, i-100):min(len(first), i+250)], second[max(0, i-100):min(len(second), i+250)])
	}
	var p ArgumentProvenance
	if err := json.Unmarshal(first, &p); err != nil {
		t.Fatal(err)
	}
	if p.SchemaVersion != 1 || len(p.Sites) == 0 || len(p.Nodes) == 0 {
		t.Fatalf("missing projection evidence: sites=%d nodes=%d", len(p.Sites), len(p.Nodes))
	}
	seenSite := map[string]bool{}
	for _, s := range p.Sites {
		if !strings.HasPrefix(s.ID, "site:") && !strings.HasPrefix(s.ID, "operation:") {
			t.Fatalf("unstable operation site id %q", s.ID)
		}
		if s.SiteRef != "" && !strings.HasPrefix(s.SiteRef, "site:") {
			t.Fatalf("invalid effect site ref: %+v", s)
		}
		seenSite[s.ID] = true
	}
	seenValue := map[string]bool{}
	for _, n := range p.Nodes {
		if !strings.HasPrefix(n.ID, "value:") {
			t.Fatalf("unstable value id %q", n.ID)
		}
		seenValue[n.ID] = true
	}
	if len(seenSite) == 0 || len(seenValue) == 0 {
		t.Fatal("references were not emitted")
	}
	kinds := map[string]bool{}
	for _, n := range p.Nodes {
		kinds[n.Kind] = true
	}
	for _, kind := range []string{"constant", "parameter", "conversion", "phi", "call_result"} {
		if !kinds[kind] {
			t.Errorf("fixture lacks %s evidence: %v", kind, kinds)
		}
	}
	if len(p.Calls) == 0 {
		t.Fatal("helper call/return references missing")
	}
	callIDs := map[string]bool{}
	for _, c := range p.Calls {
		callIDs[c.ID] = true
	}
	values := map[string]ArgumentValue{}
	for _, n := range p.Nodes {
		values[n.ID] = n
	}
	literalOrigins := map[string]bool{}
	for _, n := range p.Nodes {
		if n.Kind == "constant" && (n.Label == `"one"` || n.Label == `"two"`) {
			literalOrigins[n.ID] = true
		}
	}
	if len(literalOrigins) != 2 {
		t.Fatalf("distinct caller literals absent: %#v", literalOrigins)
	}
	foundCallerEdge := false
	for _, e := range p.Edges {
		if literalOrigins[e.From] && e.CallRef != "" && callIDs[e.CallRef] {
			foundCallerEdge = true
		}
	}
	if !foundCallerEdge {
		t.Fatal("caller literal has no resolvable call edge")
	}
	for _, e := range p.Edges {
		if e.CallRef != "" && !callIDs[e.CallRef] {
			t.Fatalf("dangling edge call_ref %q", e.CallRef)
		}
	}
	for _, s := range p.Sites {
		if s.CallRef != "" && !callIDs[s.CallRef] {
			t.Fatalf("dangling site call_ref %q", s.CallRef)
		}
	}
	_ = values
	cmdBoundary := false
	for _, s := range p.Sites {
		if s.API == "os/exec.(*Cmd).Run" && s.Resolution == "unresolved_mutable_receiver" && len(s.Roles) == 1 && s.Roles[0].Boundary != "" {
			cmdBoundary = true
		}
	}
	if !cmdBoundary {
		t.Fatal("Cmd mutable receiver boundary missing")
	}
}
func TestArgumentProvenanceCandidateWrapperAndAggregateBoundaries(t *testing.T) {
	dir := argumentFixture(t)
	report, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, ArgumentProvenance: true})
	if err != nil {
		t.Fatal(err)
	}
	p := report.ArgumentProvenance
	source, err := os.ReadFile(filepath.Join(dir, "fixture.go"))
	if err != nil {
		t.Fatal(err)
	}
	internalLine := 0
	for i, line := range strings.Split(string(source), "\n") {
		if strings.Contains(line, "func localRemove(p string) error { return os.Remove(p) }") {
			internalLine = i + 1
		}
	}
	var internal *ArgumentSite
	for i := range p.Sites {
		site := &p.Sites[i]
		if site.API == "os.Remove" && site.Location.Line == internalLine {
			internal = site
			break
		}
	}
	if internal == nil || len(internal.Roles) != 1 || internal.SiteRef == "" {
		var observed []ArgumentSite
		for _, site := range p.Sites {
			if site.API == "os.Remove" {
				observed = append(observed, site)
			}
		}
		t.Fatalf("internal wrapper operation line %d does not join its effect site: %+v", internalLine, observed)
	}
	var wrapperCall *ArgumentCall
	for _, edge := range p.Edges {
		if edge.To != internal.Roles[0].ValueRef || !strings.HasPrefix(edge.Resolution, "candidate_") {
			continue
		}
		for i := range p.Calls {
			call := &p.Calls[i]
			if call.ID == edge.CallRef && strings.Contains(call.Caller, "candidateLocalWrapper") {
				wrapperCall = call
			}
		}
	}
	if wrapperCall == nil {
		t.Fatalf("candidate wrapper call_ref is not linked into exact operation argument %s", internal.Roles[0].ValueRef)
	}
	labels := map[string]bool{}
	boundaries := map[string]bool{}
	for _, node := range p.Nodes {
		labels[node.Label] = true
		boundaries[node.Boundary] = true
	}
	if !labels["append"] {
		t.Fatalf("append aggregate node missing: labels=%v", labels)
	}
	if !boundaries["mutable_heap_state"] {
		t.Fatalf("slice literal element-store boundary missing: %v", boundaries)
	}
	for _, site := range p.Sites {
		if site.SiteRef == "" && site.Resolution == "established" {
			t.Fatalf("site without effect join presented as established: %+v", site)
		}
	}
}

func TestArgumentProvenanceLabelsCapturedAndMutableLoadsAsExplicitBoundaries(t *testing.T) {
	dir := argumentFixture(t)
	report, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, ArgumentProvenance: true})
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(filepath.Join(dir, "fixture.go"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(source), "\n")
	cases := []struct {
		function string
		boundary string
	}{
		{function: "func closureBoundary", boundary: "closure_capture"},
		{function: "func mutableFieldBoundary", boundary: "mutable_heap_state"},
	}
	for _, tc := range cases {
		t.Run(tc.function, func(t *testing.T) {
			line := 0
			for i, sourceLine := range lines {
				if strings.Contains(sourceLine, tc.function) {
					line = i + 1
					break
				}
			}
			if line == 0 {
				t.Fatalf("source line for %q not found", tc.function)
			}
			var site *ArgumentSite
			for i := range report.ArgumentProvenance.Sites {
				candidate := &report.ArgumentProvenance.Sites[i]
				if candidate.API == "os.Remove" && candidate.Location.Line == line {
					site = candidate
					break
				}
			}
			if site == nil || len(site.Roles) != 1 {
				t.Fatalf("os.Remove site at line %d is missing: %+v", line, site)
			}
			root := site.Roles[0].ValueRef
			nodes := map[string]ArgumentValue{}
			for _, node := range report.ArgumentProvenance.Nodes {
				nodes[node.ID] = node
			}
			if got := nodes[root].Boundary; got != tc.boundary || nodes[root].Kind != "unresolved" {
				t.Fatalf("root value=%+v, want unresolved %q boundary", nodes[root], tc.boundary)
			}
			foundBoundaryInput := false
			for _, edge := range report.ArgumentProvenance.Edges {
				if edge.To == root && nodes[edge.From].Boundary == tc.boundary {
					foundBoundaryInput = true
					break
				}
			}
			if !foundBoundaryInput {
				t.Fatalf("root %s does not retain an explicit %q input boundary", root, tc.boundary)
			}
		})
	}
}

func TestArgumentProvenanceOperationRoleIndices(t *testing.T) {
	cases := map[string][]ArgumentRole{
		"os.WriteFile":        {{Role: "path", Index: 0}, {Role: "data", Index: 1}},
		"io/ioutil.WriteFile": {{Role: "path", Index: 0}, {Role: "data", Index: 1}},
		"os.Remove":           {{Role: "path", Index: 0}},
		"os.Rename":           {{Role: "old_path", Index: 0}, {Role: "new_path", Index: 1}},
		"os.StartProcess":     {{Role: "path", Index: 0}, {Role: "argv", Index: 1}},
		"os.(*Root).Rename":   {{Role: "old_path", Index: 1}, {Role: "new_path", Index: 2}},
		"os.OpenInRoot":       {{Role: "dir", Index: 0}, {Role: "path", Index: 1}},
		"os.Link":             {{Role: "old_path", Index: 0}, {Role: "new_path", Index: 1}},
		"os.Symlink":          {{Role: "old_path", Index: 0}, {Role: "new_path", Index: 1}},
		"os.(*Root).Link":     {{Role: "old_path", Index: 1}, {Role: "new_path", Index: 2}},
		"os.(*Root).Symlink":  {{Role: "old_path", Index: 1}, {Role: "new_path", Index: 2}},
	}
	for api, want := range cases {
		got := argumentRoles(api)
		if len(got) != len(want) {
			t.Errorf("%s roles=%+v, want %+v", api, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s role %d=%+v, want %+v", api, i, got[i], want[i])
			}
		}
	}
}

func TestArgumentProvenanceMinimumJSONLimitAndDefaultOptIn(t *testing.T) {
	dir := argumentFixture(t)
	if _, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, ArgumentProvenance: true, ArgumentProvenanceLimits: ArgumentProvenanceOptions{MaxJSONBytes: 1}}); err == nil || !strings.Contains(err.Error(), "minimum valid schema") {
		t.Fatalf("tiny positive byte cap must fail, got %v", err)
	}
	with, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, ArgumentProvenance: true})
	if err != nil {
		t.Fatal(err)
	}
	without, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true})
	if err != nil {
		t.Fatal(err)
	}
	if without.ArgumentProvenance != nil {
		t.Fatal("provenance enabled without opt-in")
	}
	withoutJSON, err := json.Marshal(without)
	if err != nil {
		t.Fatal(err)
	}
	withoutAgain, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true})
	if err != nil {
		t.Fatal(err)
	}
	againJSON, err := json.Marshal(withoutAgain)
	if err != nil {
		t.Fatal(err)
	}
	if string(withoutJSON) != string(againJSON) || with.ArgumentProvenance == nil {
		t.Fatal("default output changed or opt-in missing")
	}
}

func TestArgumentProvenanceIndependentLimits(t *testing.T) {
	dir := argumentFixture(t)
	cases := []struct {
		name  string
		opts  ArgumentProvenanceOptions
		limit string
	}{{"depth", ArgumentProvenanceOptions{MaxDepth: 1}, "depth"}, {"nodes", ArgumentProvenanceOptions{MaxNodes: 1}, "ssa_value_nodes"}, {"calls", ArgumentProvenanceOptions{MaxCalls: 1}, "call_relations"}, {"alternatives", ArgumentProvenanceOptions{MaxAlternatives: 1}, "alternative_origins"}, {"json", ArgumentProvenanceOptions{MaxJSONBytes: 400}, "json_bytes"}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, ArgumentProvenance: true, ArgumentProvenanceLimits: tc.opts})
			if err != nil {
				t.Fatal(err)
			}
			p := r.ArgumentProvenance
			b, err := json.Marshal(p)
			if err != nil {
				t.Fatal(err)
			}
			if tc.name == "json" && len(b) > 400 {
				t.Fatalf("payload %d exceeds cap", len(b))
			}
			{
				nodes, calls := map[string]bool{}, map[string]bool{}
				for _, n := range p.Nodes {
					nodes[n.ID] = true
				}
				for _, c := range p.Calls {
					calls[c.ID] = true
				}
				for _, e := range p.Edges {
					if !nodes[e.From] || !nodes[e.To] || e.CallRef != "" && !calls[e.CallRef] {
						t.Fatalf("dangling trimmed edge: %+v", e)
					}
				}
				for _, c := range p.Calls {
					for _, ref := range append(append([]string{}, c.Arguments...), c.Results...) {
						if !nodes[ref] {
							t.Fatalf("dangling trimmed call value: %+v ref=%s", c, ref)
						}
					}
				}
				for _, s := range p.Sites {
					if s.CallRef != "" && !calls[s.CallRef] {
						t.Fatalf("dangling trimmed call: %+v", s)
					}
					for _, r := range s.Roles {
						if r.ValueRef != "" && !nodes[r.ValueRef] {
							t.Fatalf("dangling trimmed value: %+v", r)
						}
					}
				}
			}
			found := false
			for _, x := range p.Truncation {
				if x.Limit == tc.limit {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing truncation %q: %+v", tc.limit, p.Truncation)
			}
		})
	}
}
