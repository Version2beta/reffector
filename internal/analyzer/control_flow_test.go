package analyzer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestControlFlowProjectionOptInAndStable(t *testing.T) {
	dir := argumentFixture(t)
	base, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, EffectView: true})
	if err != nil {
		t.Fatal(err)
	}
	if base.ControlFlow != nil {
		t.Fatal("control flow emitted without opt-in")
	}
	run := func() []byte {
		r, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, EffectView: true, ControlFlow: true})
		if err != nil {
			t.Fatal(err)
		}
		if r.ControlFlow == nil || r.ControlFlow.Schema != "reffector.control-flow.v1" {
			t.Fatal("versioned projection missing")
		}
		if len(r.ControlFlow.Limitations) == 0 || r.ControlFlow.Limitations[0] != "SSA control-flow facts are structural evidence only; they do not establish execution, feasible paths, safety, authorization, or cleanup success." {
			t.Fatalf("structural-only limitation missing: %#v", r.ControlFlow.Limitations)
		}
		b, err := json.Marshal(r.ControlFlow)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	first, second := run(), run()
	if string(first) != string(second) {
		t.Fatal("control-flow IDs/output are unstable")
	}
	var projection ControlFlow
	if err := json.Unmarshal(first, &projection); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range projection.Rows {
		if row.API == "os.WriteFile" && len(row.Branches) > 0 {
			found = true
			if !row.Reachable {
				t.Fatal("matched branch call is unreachable")
			}
		}
	}
	if !found {
		t.Fatal("diamond condition/call projection missing")
	}
}

func TestControlFlowReportsEffectReportSiteTruncation(t *testing.T) {
	dir := argumentFixture(t)
	report, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, EffectView: true, ControlFlow: true, MaxSites: 1})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Effects.Truncated.Sites || report.Effects.Coverage.FilteredSites <= len(report.Effects.Sites) {
		t.Fatalf("fixture did not truncate EffectReport sites: %#v", report.Effects)
	}
	if !report.ControlFlow.Truncated["effect_report_sites"] {
		t.Fatalf("control-flow projection hid the upstream site cap: %#v", report.ControlFlow)
	}
	if len(report.ControlFlow.Limitations) < 2 || report.ControlFlow.Limitations[1] != "Rows cover only EffectReport sites retained by its site limit." {
		t.Fatalf("upstream truncation limitation missing: %#v", report.ControlFlow.Limitations)
	}
	for _, row := range report.ControlFlow.Rows {
		if row.Complete {
			t.Fatalf("row claims complete after its site inventory was truncated: %#v", row)
		}
	}
}

func TestControlFlowBranchDependenciesAndReturnBypassAreStructural(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/controlpaths\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source := `package p
import "os"
func gated(b bool) error { if b { _ = os.Remove("gated") }; return nil }
func merged(b bool) { path := "left"; if b { path = "right" }; _ = os.Remove(path) }
func early(b bool) error { if b { return nil }; _ = os.Remove("after"); return nil }
func looping(n int) { for n > 0 { _ = os.Remove("loop"); n-- } }
func spinning() { for { _ = os.Remove("forever") } }
`
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, EffectView: true, Operation: "os.Remove", ControlFlow: true})
	if err != nil {
		t.Fatal(err)
	}
	byFunction := map[string]ControlFlowRow{}
	for _, row := range report.ControlFlow.Rows {
		byFunction[row.Function.Function] = row
	}
	gated := byFunction["gated"]
	gatedByBranch := false
	for _, branch := range gated.Branches {
		for _, control := range branch.Controls {
			gatedByBranch = gatedByBranch || control.Block == gated.Block
		}
	}
	if gated.ID == "" || !gatedByBranch {
		t.Fatalf("condition does not structurally control gated call: %#v", gated)
	}
	merged := byFunction["merged"]
	if merged.ID == "" || len(merged.Branches) == 0 {
		t.Fatalf("merged call or its branch evidence missing: %#v", merged)
	}
	for _, branch := range merged.Branches {
		for _, control := range branch.Controls {
			if control.Block == merged.Block {
				t.Fatalf("branch incorrectly controls a call after the merge: %#v", merged)
			}
		}
	}
	early := byFunction["early"]
	if early.ID == "" || len(early.ReturnEvidence) != 1 {
		t.Fatalf("early return bypass evidence missing or post-call return misclassified: %#v", early)
	}
	if len(early.ReturnEvidence[0].ResultTypes) != 1 || early.ReturnEvidence[0].ResultTypes[0] != "error" {
		t.Fatalf("return result type not preserved: %#v", early.ReturnEvidence[0])
	}
	looping := byFunction["looping"]
	if looping.ID == "" || looping.Status != "complete" || len(looping.Branches) == 0 {
		t.Fatalf("loop CFG evidence is missing or incomplete: %#v", looping)
	}
	conditionLocationAvailable := false
	for _, branch := range looping.Branches {
		if branch.Location != nil {
			if branch.Location.File == "" || branch.Location.Line < 1 {
				t.Fatalf("invalid branch condition location: %#v", branch.Location)
			}
			conditionLocationAvailable = true
		}
	}
	if !conditionLocationAvailable {
		t.Fatalf("condition instruction location was available but not projected: %#v", looping.Branches)
	}
	spinning := byFunction["spinning"]
	if spinning.ID == "" || spinning.Status != "unknown_postdominators" || spinning.Complete {
		t.Fatalf("non-exiting CFG did not expose postdominator uncertainty: %#v", spinning)
	}
}

func TestControlFlowMarkersStayWithinTheirSSAFunction(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/controlmarkers\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source := `package p
import "os"
func markers(path string) {
 _ = os.Remove("parent")
 defer func() { _ = recover(); _ = os.Remove(path) }()
 if path == "panic" { panic("x") }
 return
}
`
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, EffectView: true, Operation: "os.Remove", ControlFlow: true})
	if err != nil {
		t.Fatal(err)
	}
	var parent, closure *ControlFlowRow
	for i := range report.ControlFlow.Rows {
		row := &report.ControlFlow.Rows[i]
		if row.API != "os.Remove" {
			continue
		}
		if row.Function.Function == "markers" {
			parent = row
		} else {
			closure = row
		}
	}
	if parent == nil || closure == nil {
		t.Fatalf("expected parent and closure operation rows: %#v", report.ControlFlow.Rows)
	}
	hasMarker := func(row *ControlFlowRow, kind string) bool {
		for _, marker := range row.MarkerEvidence {
			if marker.Kind == kind {
				return true
			}
		}
		return false
	}
	for _, kind := range []string{"defer", "run_defers", "panic", "function_recover"} {
		if !hasMarker(parent, kind) {
			t.Errorf("parent function lacks local %s marker: %#v", kind, parent.MarkerEvidence)
		}
	}
	for _, kind := range []string{"recover"} {
		if !hasMarker(closure, kind) {
			t.Errorf("closure lacks local %s marker: %#v", kind, closure.MarkerEvidence)
		}
	}
	if hasMarker(closure, "defer") || hasMarker(closure, "panic") || hasMarker(closure, "function_recover") {
		t.Fatalf("parent-function markers leaked into closure CFG: %#v", closure.MarkerEvidence)
	}
}

func TestControlFlowUnavailableSiteMappingsAreExplicit(t *testing.T) {
	g := &graph{siteNodes: map[string]map[*node]bool{"no-body": {&node{}: true}}}
	sites := []Site{
		{ID: "empty-api"},
		{ID: "no-nodes", Operations: []OperationEvidence{{API: "os.Remove"}}},
		{ID: "no-body", Operations: []OperationEvidence{{API: "os.Remove"}}},
	}
	projection, err := g.analyzeControlFlow(sites, ControlFlowOptions{}, false)
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]bool{}
	ids := map[string]bool{}
	for _, row := range projection.Rows {
		statuses[row.Status] = true
		if ids[row.ID] {
			t.Fatalf("duplicate unavailable row ID %q", row.ID)
		}
		ids[row.ID] = true
		if row.Complete {
			t.Fatalf("unavailable row claims completeness: %#v", row)
		}
	}
	for _, status := range []string{"unavailable_api_identity", "unavailable_site_nodes", "unavailable_function_body"} {
		if !statuses[status] {
			t.Fatalf("missing %s: %#v", status, projection.Rows)
		}
	}
}

func TestControlFlowCandidateAPIAlternativesKeepResolutionClasses(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/controlcandidates\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(`package p
import "os"
func conditional(b bool) { f := os.Chdir; if b { f = os.Remove }; _ = f("never") }
`), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, EffectView: true, Operation: "os.Remove", ControlFlow: true})
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]ControlFlowRow{}
	site := ""
	for _, row := range report.ControlFlow.Rows {
		if row.Function.Function != "conditional" {
			continue
		}
		if site == "" {
			site = row.Site
		} else if row.Site != site {
			t.Fatalf("candidate APIs did not retain their merged site: %q != %q", row.Site, site)
		}
		rows[row.API] = row
	}
	for _, api := range []string{"os.Chdir", "os.Remove"} {
		row, ok := rows[api]
		if !ok {
			t.Fatalf("missing candidate API %s at conditional: candidates=%#v all_rows=%#v", api, rows, report.ControlFlow.Rows)
		}
		if len(row.Resolutions) == 0 || row.Resolution == "established" {
			t.Fatalf("candidate resolution was lost or presented as established: %#v", row)
		}
		for _, resolution := range row.Resolutions {
			if resolution != "candidate_function_value" && resolution != "candidate_vta" {
				t.Fatalf("unexpected candidate resolution %q", resolution)
			}
		}
	}
}

func TestControlFlowPreservesSameSiteSSAFunctionVariants(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/controlvariants\n\ngo 1.26.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(`package p
import "os"
type T struct{}
func (T) Read() { _, _ = os.ReadFile("never") }
func Bound() { fn := (T{}).Read; fn() }
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "p_test.go"), []byte("package p\nfunc OnlyInTestFile() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Tests: true, Exact: true, EffectView: true, Operation: "os.ReadFile", ControlFlow: true})
	if err != nil {
		t.Fatal(err)
	}
	rows := []ControlFlowRow{}
	for _, row := range report.ControlFlow.Rows {
		if row.API == "os.ReadFile" && row.Function.Function == "(T).Read" {
			rows = append(rows, row)
		}
	}
	if len(rows) != 2 {
		t.Fatalf("expected two rows for one source site across SSA variants, got %#v", rows)
	}
	if rows[0].Site != rows[1].Site || rows[0].Variant == rows[1].Variant || rows[0].ID == rows[1].ID {
		t.Fatalf("source site variants were merged or IDs collided: %#v", rows)
	}
}

func TestControlFlowSiteOperationsAreSortedAndDeduplicated(t *testing.T) {
	site := Site{APIs: []string{"fallback"}, Operations: []OperationEvidence{{API: "z"}, {API: "a"}, {API: "z"}}}
	got := siteOperationAPIs(site)
	if len(got) != 2 || got[0] != "a" || got[1] != "z" {
		t.Fatalf("site operation alternatives = %#v", got)
	}
	fallback := siteOperationAPIs(Site{APIs: []string{"os.Remove"}, Operations: []OperationEvidence{{API: ""}}})
	if len(fallback) != 1 || fallback[0] != "os.Remove" {
		t.Fatalf("empty operation identity prevented explicit API fallback: %#v", fallback)
	}
	if empty := siteOperationAPIs(Site{Operations: []OperationEvidence{{API: ""}}}); len(empty) != 0 {
		t.Fatalf("empty API identity was not treated as unavailable: %#v", empty)
	}
}

func TestControlFlowGlobalBranchAndRelationshipBounds(t *testing.T) {
	dir := argumentFixture(t)
	run := func(limits ControlFlowOptions) *ControlFlow {
		t.Helper()
		r, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, EffectView: true, ControlFlow: true, ControlFlowLimits: limits})
		if err != nil {
			t.Fatal(err)
		}
		return r.ControlFlow
	}
	base := ControlFlowOptions{MaxFunctions: 64, MaxBlocksPerFunction: 128, MaxBlockVisits: 4096, MaxBranches: 256, MaxRelationships: 2048, MaxDepth: 8, MaxJSONBytes: 1048576}
	branchLimits := base
	branchLimits.MaxBranches = 1
	branches := run(branchLimits)
	branchCount := 0
	for _, row := range branches.Rows {
		branchCount += len(row.Branches)
	}
	if branchCount > branchLimits.MaxBranches || !branches.Truncated["branches"] {
		t.Fatalf("global branch cap not enforced across rows: count=%d truncated=%#v", branchCount, branches.Truncated)
	}
	relationshipLimits := base
	relationshipLimits.MaxRelationships = 1
	relationships := run(relationshipLimits)
	count := 0
	for _, row := range relationships.Rows {
		count += len(row.Dominators) + len(row.ReturnEvidence) + len(row.MarkerEvidence)
		for _, branch := range row.Branches {
			count += len(branch.Controls)
		}
	}
	if count > relationshipLimits.MaxRelationships || !relationships.Truncated["relationships"] {
		t.Fatalf("global relationship cap not enforced across rows: count=%d truncated=%#v", count, relationships.Truncated)
	}
}

func TestControlFlowDepthAndJSONBoundsPreserveIdentity(t *testing.T) {
	dir := argumentFixture(t)
	depthLimits := ControlFlowOptions{MaxFunctions: 64, MaxBlocksPerFunction: 128, MaxBlockVisits: 4096, MaxBranches: 256, MaxRelationships: 2048, MaxDepth: 1, MaxJSONBytes: 1048576}
	depthReport, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, EffectView: true, ControlFlow: true, ControlFlowLimits: depthLimits})
	if err != nil {
		t.Fatal(err)
	}
	if !depthReport.ControlFlow.Truncated["depth"] {
		t.Fatal("fixture did not exercise a postdominator chain deeper than the depth bound")
	}
	for _, row := range depthReport.ControlFlow.Rows {
		for _, branch := range row.Branches {
			for _, control := range branch.Controls {
				if control.Depth < 1 || control.Depth > depthLimits.MaxDepth {
					t.Fatalf("invalid control depth: %#v", control)
				}
			}
		}
	}
	jsonLimits := depthLimits
	jsonLimits.MaxDepth = 8
	jsonLimits.MaxJSONBytes = 15000
	jsonReport, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, EffectView: true, ControlFlow: true, ControlFlowLimits: jsonLimits})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(jsonReport.ControlFlow)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > jsonLimits.MaxJSONBytes {
		t.Fatalf("JSON cap exceeded: %d > %d", len(encoded), jsonLimits.MaxJSONBytes)
	}
	if !jsonReport.ControlFlow.Truncated["json_bytes"] {
		t.Fatal("fixture did not exercise JSON byte fallback")
	}
	if jsonReport.ControlFlow.Truncated["json_bytes"] {
		for _, row := range jsonReport.ControlFlow.Rows {
			if row.Status != "omitted_json_bytes" || row.Site == "" || row.API == "" || row.Complete {
				t.Fatalf("JSON omission lost matched identity/status: %#v", row)
			}
		}
	}
}

func TestControlFlowVisitBudgetIsSharedAcrossSitesInOneFunction(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.test/control\n\ngo 1.23\n"), 0600); err != nil {
		t.Fatal(err)
	}
	source := `package control
import "os"
func twoSites() { _ = os.Remove("one"); _ = os.Remove("two") }
`
	if err := os.WriteFile(filepath.Join(dir, "control.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	limits := ControlFlowOptions{MaxFunctions: 1, MaxBlocksPerFunction: 8, MaxBlockVisits: 5, MaxBranches: 8, MaxRelationships: 32, MaxDepth: 4, MaxJSONBytes: 1 << 20}
	report, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, EffectView: true, Operation: "os.Remove", ControlFlow: true, ControlFlowLimits: limits})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.ControlFlow.Rows) != 2 {
		t.Fatalf("expected two operation rows: %#v", report.ControlFlow.Rows)
	}
	if report.ControlFlow.Truncated["block_visits"] {
		t.Fatalf("same function CFG was charged once per site: %#v", report.ControlFlow.Truncated)
	}
	for _, row := range report.ControlFlow.Rows {
		if !row.Complete || row.Status != "complete" {
			t.Fatalf("shared cached CFG should complete both rows: %#v", row)
		}
	}
}

func TestControlFlowFunctionAndBlockBoundsIndependently(t *testing.T) {
	dir := argumentFixture(t)
	base := ControlFlowOptions{MaxFunctions: 64, MaxBlocksPerFunction: 128, MaxBlockVisits: 4096, MaxBranches: 256, MaxRelationships: 2048, MaxDepth: 8, MaxJSONBytes: 1048576}
	cases := []struct {
		name  string
		field string
		limit func(*ControlFlowOptions)
	}{
		{"functions", "functions", func(o *ControlFlowOptions) { o.MaxFunctions = 1 }},
		{"blocks_per_function", "blocks_per_function", func(o *ControlFlowOptions) { o.MaxBlocksPerFunction = 1 }},
		{"block_visits", "block_visits", func(o *ControlFlowOptions) { o.MaxBlockVisits = 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			limits := base
			tc.limit(&limits)
			r, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, EffectView: true, ControlFlow: true, ControlFlowLimits: limits})
			if err != nil {
				t.Fatal(err)
			}
			if !r.ControlFlow.Truncated[tc.field] {
				t.Fatalf("%s cap did not truncate: %#v", tc.field, r.ControlFlow.Truncated)
			}
			for _, row := range r.ControlFlow.Rows {
				if tc.name == "block_visits" && row.Status == "truncated_block_visits" && (len(row.Dominators) > 0 || len(row.Branches) > 0) {
					t.Fatal("incomplete CFG emitted dominator or branch conclusions")
				}
				if row.Status == "truncated_blocks" && (len(row.Dominators) > 0 || len(row.Branches) > 0) {
					t.Fatal("bounded CFG emitted conclusions")
				}
				if tc.name == "block_visits" && row.Function.Function != "" && row.Status != "truncated_functions" && row.Status != "truncated_blocks" && row.Status != "truncated_block_visits" {
					t.Fatalf("visit exhaustion should be explicit, got %#v", row)
				}
				if len(r.ControlFlow.Truncated) > 0 && row.Complete {
					t.Fatalf("row implies completeness after truncation: %#v", row)
				}
			}
		})
	}
}
