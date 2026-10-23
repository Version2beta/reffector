package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestContractGenericImplementationEffects(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/genericimpl\ngo 1.26\n")
	mustWrite(t, filepath.Join(dir, "generic.go"), `package genericimpl
import "os"
type Target interface { Do() error }
type Reader struct{}
func (Reader) Do() error { _, err := os.ReadFile("read"); return err }
type Writer struct{}
func (Writer) Do() error { return os.WriteFile("write", nil, 0600) }
type Backend interface { Save() error }
type Worker[T Target] struct { Target T }
func (w Worker[T]) Save() error { return w.Target.Do() }
func External(b Backend) error { return b.Save() }
func UseReader() error { return External(Worker[Reader]{}) }
func UseWriter() error { return External(Worker[Writer]{}) }
`)
	r, _ := effectResult(t, dir, "-contracts", "-unresolved", "-function", "=External", "-operation", "(Backend).Save")
	if len(r.Contracts) != 1 || len(r.Contracts[0].Implementations) != 1 {
		t.Fatalf("contracts: %+v", r.Contracts)
	}
	impl := r.Contracts[0].Implementations[0]
	if impl.Relationship != "candidate_vta" || !reflect.DeepEqual(impl.Effects, []string{"fs.create", "fs.read", "fs.write"}) || len(impl.PossibleEffects) != 0 {
		t.Fatalf("generic implementation evidence lost: %+v", impl)
	}
}
