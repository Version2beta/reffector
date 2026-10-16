package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

func TestPrivateWorkspaceRebasesPaths(t *testing.T) {
	dir := t.TempDir()
	module := filepath.Join(dir, "module")
	if err := os.Mkdir(module, 0700); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(dir, "go.work")
	if err := os.WriteFile(work, []byte("go 1.24.0\nuse ./module\nreplace example.com/dep => ./dep\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWORK", work)
	cfg, cleanup, err := loadConfig(Options{Dir: module})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	var path string
	for _, entry := range cfg.Env {
		if strings.HasPrefix(entry, "GOWORK=") {
			path = strings.TrimPrefix(entry, "GOWORK=")
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := modfile.ParseWork(path, data, nil)
	if err != nil {
		t.Fatal(err)
	}
	physicalModule, err := filepath.EvalSymlinks(module)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Use) != 1 || parsed.Use[0].Path != physicalModule || len(parsed.Replace) != 1 || parsed.Replace[0].New.Path != filepath.Join(dir, "dep") {
		t.Fatalf("incorrect rebasing:\n%s", data)
	}
}
