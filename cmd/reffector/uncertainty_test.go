package main

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestUncertaintyOriginsAndInheritance(t *testing.T) {
	dir := copyFixture(t)
	mustWrite(t, filepath.Join(dir, "reasons.go"), `package fixture
import "strings"
func Unrecognized() { _ = strings.TrimSpace("never") }
func Unsupported() { println("never executed") }
func Mixed(fn func()) { Unknown(fn); fn() }
`)
	report := detailed(t, dir, "-exact", "-all", "-max-paths", "10000")
	states := map[string]analyzer.FunctionUncertainty{}
	for _, s := range report.Details.Uncertainty {
		states[s.Function] = s
	}
	for name, want := range map[string][2]bool{
		"Unknown": {true, false}, "ForwardUnknown": {false, true}, "Mixed": {true, true},
		"Read": {false, false}, "KnownInterface": {false, false}, "OpenInterface": {true, false},
	} {
		got, ok := states[name]
		if !ok || got.Direct != want[0] || got.Inherited != want[1] {
			t.Fatalf("%s: %+v want %v", name, got, want)
		}
	}
	reasons := map[string]map[string]bool{}
	unknownSites := 0
	inherited := false
	for _, site := range report.Details.Sites {
		if site.Uncertain && len(site.Reasons) == 0 {
			t.Fatalf("unexplained uncertainty %+v", site)
		}
		if site.Location.Line < 1 || site.Location.Column < 1 {
			t.Fatalf("missing location %+v", site)
		}
		if reasons[site.Function] == nil {
			reasons[site.Function] = map[string]bool{}
		}
		for _, reason := range site.Reasons {
			reasons[site.Function][reason] = true
		}
		if site.Function == "ForwardUnknown" {
			t.Fatalf("inherited origin duplicated into caller: %+v", site)
		}
		if site.Function == "Unknown" {
			unknownSites++
			for _, w := range site.Witnesses {
				if w.Root.Function == "ForwardUnknown" && !w.Direct && len(w.Edges) == 1 {
					inherited = true
				}
			}
		}
	}
	for name, reason := range map[string]string{
		"Unknown": "function_value", "OpenInterface": "interface_dispatch", "External": "missing_body",
		"Reflect": "reflection_opaque", "Unrecognized": "unrecognized_stdlib", "Unsupported": "unsupported_resolution", "FormatStringer": "callback",
	} {
		if !reasons[name][reason] {
			t.Fatalf("%s missing %s: %v", name, reason, reasons[name])
		}
	}
	if unknownSites != 1 || !inherited {
		t.Fatalf("origin grouping/witness: sites=%d inherited=%v", unknownSites, inherited)
	}
	opaque := detailed(t, dir, "-exact", "-all", "./opaque")
	if len(opaque.Details.Sites) != 1 || opaque.Details.Sites[0].Kind != "declaration" || opaque.Details.Sites[0].Reasons[0] != "missing_body" {
		t.Fatalf("bodyless source declaration origin: %+v", opaque.Details)
	}
}

func TestHTTPAndResourceSemanticsWithoutInventedTargets(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/semantics\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "p.go"), `package p
import ("net/http"; "net"; "os"; "io"; "bytes")
func Respond(w http.ResponseWriter) { w.WriteHeader(200); _, _ = w.Write(nil) }
func Forward(w http.ResponseWriter) { Respond(w) }
func BoundHTTP(w http.ResponseWriter) { fn := w.WriteHeader; fn(200) }
func CloseFile(f *os.File) { _ = f.Close() }
func Acquire() { f, _ := os.Open("never"); defer f.Close() }
func TCP(c *net.TCPConn) { _ = c.Close(); _ = c.CloseWrite() }
func Conn(c net.Conn) { _ = c.Close() }
func Writer(w io.Writer) { _, _ = w.Write(nil) }
func Fake(w interface { Write([]byte) (int,error); WriteHeader(int) }) { w.WriteHeader(200); _, _ = w.Write(nil) }
func Memory() { var b bytes.Buffer; _, _ = b.Write(nil) }
func Phi(b bool) { fn := os.Getenv; if b { fn = os.ExpandEnv }; _ = fn("never") }
`)
	before := snapshot(t, dir)
	report := detailed(t, dir, "-all")
	summaries := map[string]analyzer.Record{}
	for _, r := range report.Records {
		summaries[r.Function] = r
	}
	for name, want := range map[string][]string{
		"Respond": {"http.response"}, "Forward": {"http.response"}, "BoundHTTP": {"http.response"},
		"CloseFile": {"resource.release"}, "Acquire": {"fs.read", "resource.acquire", "resource.release"},
		"TCP": {"net.close", "resource.release"}, "Conn": {"net.close", "resource.release"},
		"Writer": {}, "Fake": {}, "Memory": {}, "Phi": {"env.read"},
	} {
		r, ok := summaries[name]
		got := []string{}
		for _, e := range r.Effects {
			got = append(got, e.Identifier)
		}
		if !ok || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s got %+v want %v", name, r, want)
		}
	}
	if !summaries["Respond"].Uncertain || !summaries["Forward"].Uncertain || summaries["TCP"].Uncertain || summaries["CloseFile"].Uncertain || summaries["Memory"].Uncertain {
		t.Fatal("uncertainty lost or invented")
	}
	count, phiCount := 0, 0
	for _, s := range report.Details.Sites {
		if s.Function == "Phi" {
			phiCount++
			if s.API != "" || !reflect.DeepEqual(s.APIs, []string{"os.ExpandEnv", "os.Getenv"}) {
				t.Fatalf("phi alternatives lost or split: %+v", s)
			}
		}
		if s.Function == "Respond" {
			count++
			if s.Recognition != "interface_summary" || !s.Uncertain || len(s.Reasons) != 1 || s.Reasons[0] != "interface_dispatch" {
				t.Fatalf("semantic summary must retain open dispatch: %+v", s)
			}
			if s.API != "net/http.(ResponseWriter).Write" && s.API != "net/http.(ResponseWriter).WriteHeader" {
				t.Fatalf("identity: %+v", s)
			}
		}
	}
	if count != 2 || phiCount != 1 {
		t.Fatalf("response sites %d, phi sites %d", count, phiCount)
	}
	for _, edge := range report.Details.CallEdges {
		if edge.Caller.Function == "Respond" || edge.Caller.Function == "Writer" || edge.Caller.Function == "Conn" || edge.Caller.Function == "Fake" {
			t.Fatalf("invented open-interface target %+v", edge)
		}
	}
	if !reflect.DeepEqual(before, snapshot(t, dir)) {
		t.Fatal("source changed")
	}
}
