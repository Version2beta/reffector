package main

import (
	"path/filepath"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestCallEdgesExposeGoroutineAndDeferredInvocationKinds(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/asyncedges\n\ngo 1.27\n")
	mustWrite(t, filepath.Join(dir, "async.go"), `package asyncedges
import "os"

func remove() { _ = os.Remove("deferred") }
func write() { _ = os.WriteFile("spawned", nil, 0600) }

func Deferred() { defer remove() }
func Spawned() { go write() }
func Called() { remove(); write() }

type Runner interface { Run() }
type candidate struct{}
func (candidate) Run() { _ = os.Mkdir("created", 0700) }
func Candidate(r Runner) { go r.Run() }
func CandidateRoot() { Candidate(candidate{}) }
`)

	checkWitnessKinds := func(effect, owner string, want map[string]string) {
		t.Helper()
		report := detailed(t, dir, "-all", "-effect", effect)
		if len(report.Details.Sites) != 1 {
			t.Fatalf("%s operation-site count = %d, want one origin: %+v", effect, len(report.Details.Sites), report.Details.Sites)
		}
		var site *analyzer.Site
		for i := range report.Details.Sites {
			if report.Details.Sites[i].Function == owner {
				site = &report.Details.Sites[i]
				break
			}
		}
		if site == nil {
			t.Fatalf("missing %s operation owned by %s: %+v", effect, owner, report.Details.Sites)
		}
		edges := make(map[string]analyzer.CallEdge, len(report.Details.CallEdges))
		for _, edge := range report.Details.CallEdges {
			edges[edge.ID] = edge
		}
		found := make(map[string]bool)
		for _, witness := range site.Witnesses {
			if kind, selected := want[witness.Root.Function]; selected {
				if len(witness.Edges) != 1 {
					t.Errorf("%s witness path for %s = %+v, want one edge", witness.Root.Function, effect, witness)
					continue
				}
				edge, ok := edges[witness.Edges[0]]
				if !ok || edge.Caller.Function != witness.Root.Function || edge.Callee.Function != owner || edge.Kind != kind || edge.Resolution != "established" {
					t.Errorf("%s witness edge = %+v, want kind=%s established", witness.Root.Function, edge, kind)
				}
				found[witness.Root.Function] = true
			} else if len(witness.Edges) == 0 && witness.Root.Function != owner {
				t.Errorf("unexpected direct witness: %+v", witness)
			}
		}
		records := make(map[string]analyzer.Record, len(report.Records))
		for _, record := range report.Records {
			records[record.Function] = record
		}
		for function := range want {
			if !found[function] {
				t.Errorf("missing %s witness for %s", function, effect)
			}
			record, ok := records[function]
			inherited := false
			for _, candidate := range record.Effects {
				if candidate.Identifier == effect {
					inherited = !candidate.Direct
				}
			}
			if !ok || !inherited {
				t.Errorf("%s summary for %s = %+v, want inherited effect", function, effect, record)
			}
		}
	}

	checkWitnessKinds("fs.write", "write", map[string]string{"Spawned": "go", "Called": "call"})
	checkWitnessKinds("fs.remove", "remove", map[string]string{"Deferred": "defer", "Called": "call"})

	// The default reverse graph uses the same source-edge evidence as -details.
	report, _ := effectResult(t, dir, "-all", "-effect", "fs.write")
	seen := map[string]string{}
	for _, edge := range report.Edges {
		if edge.Source != nil {
			seen[edge.Source.Caller.Function] = edge.Source.Kind
		}
	}
	if seen["Spawned"] != "go" || seen["Called"] != "call" {
		t.Fatalf("effect-view source edge kinds = %v", seen)
	}

	// A VTA candidate edge on a go statement retains both its invocation kind
	// and candidate-only resolution. Its launch operation stays uncertain.
	candidateReport, _ := effectResult(t, dir, "-all", "-function", "=Candidate", "-effect", "fs.create", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100")
	foundCandidateGo := false
	for _, edge := range candidateReport.Edges {
		if edge.Source != nil && edge.Source.Caller.Function == "Candidate" && edge.Source.Callee.Function == "(candidate).Run" {
			foundCandidateGo = edge.Source.Kind == "go" && edge.Source.Resolution == "candidate_vta" && edge.Candidate
		}
	}
	if !foundCandidateGo {
		t.Fatalf("VTA go edge lost invocation kind or candidate-only status: %+v", candidateReport.Edges)
	}
	var candidateRecord *analyzer.Record
	for i := range candidateReport.Inventory {
		if candidateReport.Inventory[i].Function == "Candidate" {
			candidateRecord = &candidateReport.Inventory[i]
			break
		}
	}
	if candidateRecord == nil || !candidateRecord.Uncertain || len(candidateRecord.PossibleEffects) != 1 || candidateRecord.PossibleEffects[0] != "fs.create" {
		t.Fatalf("VTA candidate summary should retain possible effect and uncertainty: %+v", candidateRecord)
	}
	launchReport, _ := effectResult(t, dir, "-all", "-function", "=Candidate", "-effect", "concurrency.goroutine")
	if len(launchReport.Sites) != 1 || launchReport.Sites[0].Function != "Candidate" || !launchReport.Sites[0].Uncertain || len(launchReport.Sites[0].Reasons) != 1 || launchReport.Sites[0].Reasons[0] != "interface_dispatch" {
		t.Fatalf("candidate go launch must retain dispatch uncertainty: %+v", launchReport.Sites)
	}
}
