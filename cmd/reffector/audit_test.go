package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func auditLines(t *testing.T, text string) ([]analyzer.AuditSite, []analyzer.AuditFunction) {
	t.Helper()
	sites := []analyzer.AuditSite{}
	functions := []analyzer.AuditFunction{}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(text), "\n") {
		var kind struct {
			RecordType string `json:"record_type"`
		}
		if err := json.Unmarshal([]byte(line), &kind); err != nil {
			t.Fatal(err)
		}
		switch kind.RecordType {
		case "site":
			var s analyzer.AuditSite
			if err := json.Unmarshal([]byte(line), &s); err != nil {
				t.Fatal(err)
			}
			if seen[s.ID] || s.ID == "" || s.Location.Line < 1 || s.Location.Column < 1 {
				t.Fatalf("duplicate/missing source identity: %+v", s)
			}
			seen[s.ID] = true
			sites = append(sites, s)
		case "function":
			var f analyzer.AuditFunction
			if err := json.Unmarshal([]byte(line), &f); err != nil {
				t.Fatal(err)
			}
			functions = append(functions, f)
		default:
			t.Fatalf("unknown record %s", line)
		}
	}
	return sites, functions
}

func TestAuditJSONLAndSameGraphEvidence(t *testing.T) {
	dir := copyFixture(t)
	// The CLI's physical working directory and API Dir must name the same path.
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "audit.go"), `package fixture
import ("os"; "net/http")
func AuditKnown(w http.ResponseWriter) { w.WriteHeader(200) }
type auditHolder struct { fn func() }
func AuditCandidate() { h:=auditHolder{fn:Read}; h.fn(); _=os.Setenv("never","never") }
func AuditGeneric[T any](fn func(), v T) { fn() }
func AuditUse(fn func()) { AuditGeneric(fn,1); AuditGeneric(fn,"") }
`)
	mustWrite(t, filepath.Join(dir, "dep", "dep.go"), "package dep\nfunc Load() { println(\"never\") }\n")
	before := snapshot(t, dir)
	out, diag, code := invoke(t, dir, nil, "-audit-unresolved", "-exact", ".")
	if code != 0 || diag != "" {
		t.Fatalf("exit %d: %s", code, diag)
	}
	sites, _ := auditLines(t, out)
	ids := map[string]bool{}
	unknown, dep, candidate, generic := false, false, false, false
	for _, s := range sites {
		ids[s.ID] = true
		if !s.Uncertain || len(s.Effects) != 0 || len(s.Reasons) == 0 || s.PrimaryReason != s.Reasons[0] {
			t.Fatalf("invalid unresolved record %+v", s)
		}
		if s.Function == "Unknown" {
			unknown = s.UnknownAPI && s.API == "" && reflect.DeepEqual(s.Dispatch, []string{"function_value"}) && s.Selected
		}
		if s.Function == "Load" && s.Package == "example.com/effects/dep" {
			dep = s.Origin == "dependency" && s.Kind == "call"
		}
		if s.Function == "AuditGeneric" {
			generic = len(s.Nodes) > 1
		}
		if s.Function == "AuditCandidate" {
			candidate = s.CandidateCount > 0 && reflect.DeepEqual(s.PossibleEffects, []string{"fs.read"})
			if containsString(s.PossibleEffects, "env.write") {
				t.Fatal("unrelated owner effect leaked")
			}
		}
	}
	if !unknown || !dep || !candidate || !generic {
		t.Fatalf("missing evidence unknown=%t dep=%t candidate=%t generic=%t", unknown, dep, candidate, generic)
	}
	declarations, diagnostic, status := invoke(t, dir, nil, "-audit-unresolved", "-exact", "./opaque")
	if status != 0 || diagnostic != "" {
		t.Fatal(diagnostic)
	}
	boundary, _ := auditLines(t, declarations)
	if len(boundary) != 1 || boundary[0].Kind != "declaration" || !boundary[0].Selected {
		t.Fatal("declaration boundary lost")
	}
	// Compare both function summaries and canonical sites to the normal projection.
	normal, err := analyzer.AnalyzeReport([]string{"."}, analyzer.Options{Dir: dir, Exact: true, All: true, EffectView: true, Unresolved: true, MaxSites: 10000, MaxEdges: 10000})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range normal.Effects.Sites {
		if !ids[s.ID] {
			t.Fatalf("normal site missing %s", s.ID)
		}
	}
	out, diag, code = invoke(t, dir, nil, "-audit-unresolved", "-audit-include-known", "-all", "-stats", "-exact", ".")
	if code != 0 {
		t.Fatal(diag)
	}
	all, functions := auditLines(t, out)
	var coverage analyzer.AuditCoverage
	if err := json.Unmarshal([]byte(diag), &coverage); err != nil {
		t.Fatalf("non-stats diagnostic: %s", diag)
	}
	if coverage.EmittedSites != len(all) || coverage.FunctionRecords != len(functions) {
		t.Fatal("coverage mismatch")
	}
	known := false
	for _, s := range all {
		if s.Function == "AuditKnown" {
			known = len(s.Effects) > 0 && s.Uncertain && !s.UnknownAPI && reflect.DeepEqual(s.Dispatch, []string{"interface"})
		}
	}
	if !known || len(all) <= len(sites) {
		t.Fatal("known uncertain not included")
	}
	records := []analyzer.Record{}
	for _, f := range functions {
		if len(f.Locations) == 0 {
			t.Fatal("function lost location")
		}
		records = append(records, f.Record)
	}
	normalJSON, _ := json.Marshal(normal.Records)
	auditJSON, _ := json.Marshal(records)
	if !bytes.Equal(normalJSON, auditJSON) {
		t.Fatal("audit changed function summaries")
	}
	if !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Fatal("target changed")
	}
	// Stable IDs and bytes across independent SSA builds, including instantiations.
	again, diag, code := invoke(t, dir, nil, "-audit-unresolved", "-exact", ".")
	if code != 0 || diag != "" {
		t.Fatal(diag)
	}
	var base bytes.Buffer
	r, err := analyzer.AnalyzeReport([]string{"."}, analyzer.Options{Dir: dir, Exact: true, AuditUnresolved: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = r.Audit.WriteJSONL(&base); err != nil {
		t.Fatal(err)
	}
	if again != base.String() {
		a, b := strings.Split(again, "\n"), strings.Split(base.String(), "\n")
		for i := range a {
			if i < len(b) && a[i] != b[i] {
				t.Fatalf("audit output not deterministic at line %d:\nCLI: %s\nAPI: %s", i+1, a[i], b[i])
			}
		}
		t.Fatal("audit output record counts differ")
	}
}

func containsString(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func TestAuditDoesNotTruncateAndRejectsMisleadingFlags(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/auditlimit\ngo 1.26\n")
	var src strings.Builder
	src.WriteString("package p\nfunc Many(fn func()) {\n")
	for i := 0; i < 1007; i++ {
		fmt.Fprintln(&src, "fn()")
	}
	src.WriteString("}\n")
	mustWrite(t, filepath.Join(dir, "p.go"), src.String())
	out, diag, code := invoke(t, dir, nil, "-audit-unresolved", ".")
	if code != 0 || diag != "" {
		t.Fatalf("%d %s", code, diag)
	}
	sites, _ := auditLines(t, out)
	if len(sites) != 1007 {
		t.Fatalf("truncated: %d", len(sites))
	}
	for _, args := range [][]string{{"-audit-include-known"}, {"-audit-unresolved", "-max-sites", "2000"}, {"-audit-unresolved", "-json"}, {"-audit-unresolved", "-view", "functions"}, {"-audit-unresolved", "-reason", "function_value"}, {"-audit-unresolved", "-direct"}, {"-audit-unresolved", "-effect", "fs.read"}} {
		out, diag, code := invoke(t, dir, nil, args...)
		if code != 2 || out != "" || diag == "" {
			t.Fatalf("%v: %d %q %s", args, code, out, diag)
		}
	}
	mustWrite(t, filepath.Join(dir, "p.go"), "package p\nfunc Broken(\n")
	out, diag, code = invoke(t, dir, nil, "-audit-unresolved", ".")
	if code != 1 || out != "" || diag == "" {
		t.Fatalf("load error: %d %q %s", code, out, diag)
	}
}
