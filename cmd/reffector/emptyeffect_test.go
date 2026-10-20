package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestEmptySourceInventoryIsExplicitWhenRequested(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/emptyinventory\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "p.go"), "package p\n")
	r, text := effectResult(t, dir, "-all", "-effect", "fs.write")
	if !strings.Contains(text, `"inventory": []`) || r.Inventory == nil || r.Coverage.SourceFunctions != 0 || r.Coverage.Packages != 1 || len(r.Coverage.SelectedPackages) != 1 {
		t.Fatalf("empty inventory hidden: %s", text)
	}
	readable, diagnostic, status := invoke(t, dir, nil, "-effect", "fs.write")
	if status != 0 || diagnostic != "" || !strings.Contains(readable, "Selected packages:\n  example.com/emptyinventory") {
		t.Fatalf("empty package coverage hidden: %s %s", readable, diagnostic)
	}
	_, text = effectResult(t, dir)
	if strings.Contains(text, `"inventory"`) {
		t.Fatal("inventory is not opt-in")
	}
}
