package main

import (
	"path/filepath"
	"testing"
)

func TestInventoryExcludesLoweredIteratorHelpers(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/iterator\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "p.go"), `package p
func Seq(yield func(int) bool) { yield(1) }
func Pure() {}
func Range() { for range Seq { fn := func() { Pure() }; fn() } }
`)
	report := detailed(t, dir, "-all")
	if report.Stats.SourceFunctions != 4 {
		t.Fatalf("expected 3 declarations and 1 source literal, not lowered yield helper: %+v", report.Records)
	}
	literals := 0
	for _, r := range report.Records {
		if r.Function != "Seq" && r.Function != "Pure" && r.Function != "Range" {
			literals++
		}
	}
	if literals != 1 {
		t.Fatalf("source closure inventory: %+v", report.Records)
	}
}
