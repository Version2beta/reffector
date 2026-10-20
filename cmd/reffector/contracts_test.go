package main

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestContractsSeparateCandidateEffectsAndCompatibleImplementations(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/contracts\ngo 1.26\n")
	mustWrite(t, filepath.Join(dir, "contracts.go"), `package contracts
import "os"
type Backend interface { Save() error }
type disk struct{}
func (disk) Save() error { return os.WriteFile("x", nil, 0600) }
type unused struct{}
func (unused) Save() error { return os.Remove("x") }
func Factory() Backend { return disk{} }
func Field() error { b := Factory(); return b.Save() }
func Generic[T Backend](b T) error { return b.Save() }
func GenericUse() error { return Generic(disk{}) }
func External(b Backend) error { return b.Save() }
`)
	r, _ := effectResult(t, dir, "-contracts", "-unresolved", "-reason", "interface_dispatch")
	if len(r.Contracts) < 2 {
		t.Fatalf("contracts = %#v", r.Contracts)
	}
	seenExternal, seenCandidate, seenCompatible := false, false, false
	for _, c := range r.Contracts {
		if !c.Open || len(c.Implementations) != 2 {
			t.Fatalf("contract = %#v", c)
		}
		for _, implementation := range c.Implementations {
			switch implementation.Symbol.Function {
			case "(disk).Save":
				if implementation.Relationship == "candidate_vta" {
					seenCandidate = true
				}
				if len(implementation.Effects) != 2 {
					t.Fatalf("disk effects = %#v", implementation)
				}
			case "(unused).Save":
				if implementation.Relationship == "contract_compatible" {
					seenCompatible = true
				}
			}
		}
		for _, site := range r.Sites {
			if site.ID == c.Site && site.Function == "External" {
				seenExternal = true
			}
		}
	}
	if !seenExternal || !seenCandidate || !seenCompatible {
		t.Fatalf("missing distinction: %#v", r.Contracts)
	}
	var summaries []analyzer.Record
	if err := json.Unmarshal([]byte(success(t, dir, "-view", "functions", "-all", "-json")), &summaries); err != nil {
		t.Fatal(err)
	}
	seenField, seenGeneric := false, false
	for _, summary := range summaries {
		switch summary.Function {
		case "Field":
			seenField = true
			if len(summary.Effects) != 0 || len(summary.PossibleEffects) != 2 || !summary.Uncertain {
				t.Fatalf("candidate effects leaked into established summary: %#v", summary)
			}
		case "GenericUse":
			seenGeneric = true
			if len(summary.Effects) != 2 || len(summary.PossibleEffects) != 0 || summary.Uncertain {
				t.Fatalf("generic established effects: %#v", summary)
			}
		}
	}
	if !seenField || !seenGeneric {
		t.Fatalf("missing Field/GenericUse summaries: %#v", summaries)
	}
}
