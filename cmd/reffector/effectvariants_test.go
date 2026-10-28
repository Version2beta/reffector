package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEffectGraphPreservesBoundMethodTestVariants(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/variants\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "p.go"), `package p
import "os"
type T struct{}
func (T) Read() { _, _ = os.ReadFile("never") }
func Bound() { fn := (T{}).Read; fn() }
`)
	mustWrite(t, filepath.Join(dir, "p_test.go"), "package p\nfunc OnlyInTestFile() {}\n")
	r, first := effectResult(t, dir, "-tests", "-max-depth", "3")
	if r.Coverage.SourceFunctions != 3 {
		t.Fatalf("test variants inflate inventory: %+v", r.Coverage)
	}
	_, g, nodes, edges := reverseFor(t, r, "(T).Read")
	if len(g.Origins) != 2 {
		t.Fatalf("expected two SSA variants at one site: %+v", g)
	}
	bounds, synthetic := 0, 0
	for _, ref := range g.Nodes {
		n := nodes[ref.ID]
		if n.Function == "Bound" {
			bounds++
		}
		if n.Synthetic {
			synthetic++
		}
	}
	if bounds != 2 || synthetic != 2 {
		t.Fatalf("wrappers/variants merged: %+v", g)
	}
	for _, id := range g.Edges {
		e := edges[id]
		caller, callee := nodes[e.Caller], nodes[e.Callee]
		// Same package source and wrappers must retain the original/test package
		// distinction, even though wrapper.Function and source symbols coincide.
		if strings.SplitN(caller.Variant, "|", 2)[0] != strings.SplitN(callee.Variant, "|", 2)[0] {
			t.Fatalf("cross-variant edge: %+v -> %+v", caller, callee)
		}
	}
	text, diagnostic, status := invoke(t, dir, nil, "-tests", "-max-depth", "3")
	if status != 0 || diagnostic != "" || !strings.Contains(text, "SSA variant:") || !strings.Contains(text, "synthetic SSA forwarding") {
		t.Fatalf("variant evidence missing from readable view: %s %s", text, diagnostic)
	}
	if _, text := effectResult(t, dir, "-tests", "-max-depth", "3"); text != first {
		t.Fatal("nondeterministic test-variant graph")
	}
}

func TestExactFunctionQueryDoesNotReachUncalledClosure(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/exactroot\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "p.go"), `package p
import "os"
func Parent() { f := func() { _ = os.Remove("never") }; _ = f }
func Called() { f := func() { _ = os.Remove("never") }; f() }
`)
	r, _ := effectResult(t, dir, "-function", "=Parent", "-all")
	if len(r.Sites) != 0 || len(r.Inventory) != 4 {
		t.Fatal("uncalled closure treated as reachable from exact parent")
	}
	r, _ = effectResult(t, dir, "-function", "Parent")
	if len(r.Sites) != 1 || r.Sites[0].Function != "Parent$1" {
		t.Fatal("documented substring query lost closure root")
	}
	r, _ = effectResult(t, dir, "-function", "=Called")
	_, g, nodes, _ := reverseFor(t, r, "Called$1")
	found := false
	for _, ref := range g.Nodes {
		found = found || nodes[ref.ID].Function == "Called" && ref.Depth == 1
	}
	if !found {
		t.Fatal("known closure invocation lost")
	}
	compatible := detailed(t, dir, "-function", "=Parent")
	if len(compatible.Records) != 0 || len(compatible.Details.Sites) != 0 {
		t.Fatal("function projection disagrees with exact-root analysis")
	}
}

func TestEffectAPIAlternativesRetainTheirOwnUncertainty(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/alternatives\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "p.go"), `package p
import ("os"; "strings")
func Pick(b bool) { f := os.Getenv; if b { f = strings.TrimSpace }; _ = f("never") }
`)
	r, _ := effectResult(t, dir)
	if len(r.Sites) != 1 || len(r.Sites[0].Operations) != 2 {
		t.Fatalf("alternatives: %+v", r.Sites)
	}
	s := r.Sites[0]
	if s.API != "" || !s.Uncertain || len(s.Effects) != 0 || len(s.PossibleEffects) != 1 {
		t.Fatalf("site union: %+v", s)
	}
	for _, op := range s.Operations {
		switch op.API {
		case "os.Getenv":
			if op.Uncertain || len(op.Effects) != 0 || len(op.PossibleEffects) != 1 || op.PossibleEffects[0] != "env.read" || op.Recognition != "candidate_function_value" {
				t.Fatalf("misattributed uncertainty: %+v", op)
			}
		case "strings.TrimSpace":
			if !op.Uncertain || len(op.Effects) != 0 || op.Reasons[0] != "unrecognized_stdlib" {
				t.Fatalf("lost boundary: %+v", op)
			}
		default:
			t.Fatalf("unexpected alternative: %+v", op)
		}
	}
}
