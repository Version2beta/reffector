package main

import (
	"os"
	"path/filepath"
	"testing"
)

// VTA edges are candidates: they expose value-flow leads but must not turn an
// open interface boundary into a fabricated established call.
func TestEffectViewVTACandidatesFollowIndirectValues(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/candidates\ngo 1.26\n")
	mustWrite(t, filepath.Join(dir, "candidates.go"), `package candidates
import "os"
type Backend interface { Save() error }
type local struct{}
func (local) Save() error { return os.Rename("old", "new") }
type other struct{}
func (other) Save() error { return os.Rename("other-old", "other-new") }
type unused struct{}
func (unused) Save() error { return os.WriteFile("unused", nil, 0600) }
func Direct() error { var b Backend = local{}; return b.Save() }
func Multiple(ok bool) error { var b Backend = local{}; if ok { b = other{} }; return b.Save() }
type holder struct { backend Backend; callback func() error }
func Returned() Backend { return local{} }
func Field() error { h := holder{backend: Returned()}; return h.backend.Save() }
func Callback() error { h := holder{callback: func() error { return os.Rename("callback-old", "callback-new") }}; return h.callback() }
func Unknown(b Backend) error { return b.Save() }
func Generic[T Backend](v T) error { return v.Save() }
`)
	r, _ := effectResult(t, dir, "-effect", "fs.rename", "-max-depth", "4", "-max-edges", "100")
	if len(r.Sites) != 3 {
		t.Fatalf("rename sites = %#v", r.Sites)
	}
	byCaller := map[string]map[string]string{}
	for _, edge := range r.Edges {
		if edge.Source == nil {
			continue
		}
		if edge.Source.Resolution != "candidate_vta" && edge.Source.Resolution != "established" {
			t.Fatalf("unknown resolution: %#v", edge)
		}
		if edge.Source.Resolution == "candidate_vta" && !edge.Candidate {
			t.Fatalf("candidate edge not marked: %#v", edge)
		}
		if byCaller[edge.Source.Caller.Function] == nil {
			byCaller[edge.Source.Caller.Function] = map[string]string{}
		}
		byCaller[edge.Source.Caller.Function][edge.Source.Callee.Function] = edge.Source.Resolution
	}
	for _, caller := range []string{"Direct", "Multiple", "Field", "Callback"} {
		if len(byCaller[caller]) == 0 {
			t.Fatalf("missing reachable callback/implementation from %s: %#v", caller, byCaller)
		}
	}
	if byCaller["Direct"]["(local).Save"] != "established" || byCaller["Multiple"]["(local).Save"] != "established" || byCaller["Multiple"]["(other).Save"] != "established" || byCaller["Field"]["(local).Save"] != "candidate_vta" {
		t.Fatalf("local proof and VTA candidates must remain distinct: %#v", byCaller)
	}
	if byCaller["Unknown"] != nil {
		t.Fatalf("parameter interface fabricated a concrete target: %#v", byCaller["Unknown"])
	}
	for _, callees := range byCaller {
		if callees["(unused).Save"] != "" {
			t.Fatalf("uninstantiated implementation reached call: %#v", byCaller)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "candidates.go")); err != nil {
		t.Fatal(err)
	}
}
