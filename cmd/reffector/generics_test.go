package main

import (
	"path/filepath"
	"testing"
)

func TestGenericInventoryMergesWithoutCrossingWitnessVariants(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/generic\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "p.go"), `package p
import "os"
type readAction struct{}
func (readAction) Run() { _, _ = os.ReadFile("never") }
type envAction struct{}
func (envAction) Run() { _ = os.Setenv("never","never") }
func Apply[T interface{ Run() }](v T) { v.Run() }
func CallRead() { Apply(readAction{}) }
func CallEnv() { Apply(envAction{}) }
`)
	report := detailed(t, dir, "-all")
	if report.Stats.SourceFunctions != 5 {
		t.Fatalf("generic variants inflate inventory: %+v", report.Stats)
	}
	apply := map[string]bool{}
	for _, r := range report.Records {
		if r.Function == "Apply" {
			for _, e := range r.Effects {
				apply[e.Identifier] = true
			}
		}
	}
	if !apply["fs.read"] || !apply["env.write"] {
		t.Fatalf("instantiation effects were not merged into source summary: %v", apply)
	}
	seen := map[string]bool{}
	for _, s := range report.Details.Sites {
		for _, w := range s.Witnesses {
			if w.Root.Function == "CallRead" && s.Function == "(envAction).Run" || w.Root.Function == "CallEnv" && s.Function == "(readAction).Run" {
				t.Fatalf("path crosses incompatible generic variants: %+v -> %+v", w, s)
			}
			if w.Root.Function == "CallRead" && s.Function == "(readAction).Run" {
				seen["read"] = true
			}
			if w.Root.Function == "CallEnv" && s.Function == "(envAction).Run" {
				seen["env"] = true
			}
		}
	}
	if !seen["read"] || !seen["env"] {
		t.Fatalf("missing concrete generic witness: %v", seen)
	}
	if got := success(t, dir, "-all", "-details", "-tests"); got != success(t, dir, "-all", "-details", "-tests") {
		t.Fatal("nondeterministic variants")
	}
}
