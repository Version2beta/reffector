package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

// Readonly module mode prevents go list from updating go.mod/go.sum. Workspace
// mode can still write go.work.sum even with -mod=readonly, so give the Go tool
// a private workspace file with absolute paths and a private checksum file.
func loadConfig(opts Options) (*packages.Config, func(), error) {
	cfg := &packages.Config{
		Mode: packages.LoadAllSyntax | packages.NeedModule, Dir: opts.Dir, Tests: opts.Tests,
		BuildFlags: []string{"-mod=readonly"},
	}
	cmd := exec.Command("go", "env", "-json", "GOWORK")
	cmd.Dir = opts.Dir
	data, err := cmd.Output()
	if err != nil {
		return nil, nil, fmt.Errorf("locating Go workspace: %w", err)
	}
	var env struct{ GOWORK string }
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, nil, err
	}
	noop := func() {}
	if env.GOWORK == "" || env.GOWORK == "off" {
		return cfg, noop, nil
	}
	data, err = os.ReadFile(env.GOWORK)
	if err != nil {
		return nil, nil, err
	}
	work, err := modfile.ParseWork(env.GOWORK, data, nil)
	if err != nil {
		return nil, nil, err
	}
	base := filepath.Dir(env.GOWORK)
	absolute := func(path string) string {
		if !filepath.IsAbs(path) {
			path = filepath.Join(base, path)
		}
		// Go compares workspace paths with its physical working directory.
		// Canonicalize aliases such as macOS /var -> /private/var.
		if physical, err := filepath.EvalSymlinks(path); err == nil {
			path = physical
		}
		return path
	}
	for _, use := range work.Use {
		path := absolute(use.Path)
		if path == use.Path {
			continue
		}
		if err := work.AddUse(path, use.ModulePath); err != nil {
			return nil, nil, err
		}
		_ = work.DropUse(use.Path)
	}
	for _, r := range work.Replace {
		if r.New.Version == "" {
			if err := work.AddReplace(r.Old.Path, r.Old.Version, absolute(r.New.Path), ""); err != nil {
				return nil, nil, err
			}
		}
	}
	work.Cleanup()
	tmp, err := os.MkdirTemp("", "reffector-work-*")
	if err != nil {
		return nil, nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }
	path := filepath.Join(tmp, "go.work")
	if err := os.WriteFile(path, modfile.Format(work.Syntax), 0600); err != nil {
		cleanup()
		return nil, nil, err
	}
	if sum, err := os.ReadFile(env.GOWORK + ".sum"); err == nil {
		if err := os.WriteFile(path+".sum", sum, 0600); err != nil {
			cleanup()
			return nil, nil, err
		}
	} else if !os.IsNotExist(err) {
		cleanup()
		return nil, nil, err
	}
	cfg.Env = append(os.Environ(), "GOWORK="+path)
	return cfg, cleanup, nil
}
