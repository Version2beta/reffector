package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestContractAggregationAcrossTestPackages(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/testvariants\ngo 1.26\n")
	mustWrite(t, filepath.Join(dir, "variants.go"), `package testvariants
import "os"
type Target interface { Do() error }
type Reader struct{}
func (Reader) Do() error { _, err := os.ReadFile("read"); return err }
var target Target
func Configure() { target = Reader{} }
type Backend interface { Save() error }
type Disk struct{}
func (Disk) Save() error { return target.Do() }
func External(b Backend) error { return b.Save() }
func Use() error { return External(Disk{}) }
`)
	mustWrite(t, filepath.Join(dir, "variants_test.go"), `package testvariants
import "os"
type Writer struct{}
func (Writer) Do() error { return os.WriteFile("write", nil, 0600) }
type TestDisk struct{}
func (TestDisk) Save() error { return os.Remove("remove") }
func UseFromTest() error { target = Writer{}; return External(TestDisk{}) }
`)
	mustWrite(t, filepath.Join(dir, "external_test.go"), `package testvariants_test
import v "example.com/testvariants"
func ExternalUse() error { return v.External(v.Disk{}) }
`)
	args := []string{"-tests", "-contracts", "-unresolved", "-function", "=External", "-operation", "(Backend).Save"}
	report, baseline := effectResult(t, dir, args...)
	if len(report.Contracts) != 1 || !report.Contracts[0].Open {
		t.Fatalf("expected one open source contract: %+v", report.Contracts)
	}
	seenDisk, seenTestDisk := false, false
	for _, impl := range report.Contracts[0].Implementations {
		switch impl.Symbol.Function {
		case "(Disk).Save":
			seenDisk = true
			if impl.Relationship != "candidate_vta" || len(impl.Effects) != 0 || !reflect.DeepEqual(impl.PossibleEffects, []string{"fs.create", "fs.read", "fs.write"}) {
				t.Fatalf("lost variant effects or relationship: %+v", impl)
			}
		case "(TestDisk).Save":
			seenTestDisk = true
			if impl.Relationship != "candidate_vta" || !reflect.DeepEqual(impl.Effects, []string{"fs.remove"}) {
				t.Fatalf("test-only candidate lost: %+v", impl)
			}
		}
	}
	if !seenDisk || !seenTestDisk {
		t.Fatalf("missing implementations: %+v", report.Contracts)
	}
	for i := 0; i < 3; i++ {
		_, next := effectResult(t, dir, args...)
		if next != baseline {
			t.Fatal("test-variant report is not deterministic in the same source directory")
		}
	}
	without, _ := effectResult(t, dir, "-tests", "-unresolved", "-function", "=External", "-operation", "(Backend).Save")
	if !reflect.DeepEqual(report.Nodes, without.Nodes) || !reflect.DeepEqual(report.Edges, without.Edges) {
		t.Fatal("contract aggregation changed execution node identity or edges")
	}
}
