// Package skillinstall installs bundled agent skills without replacing
// unrelated or locally modified files.
package skillinstall

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Home    string
	Name    string
	Content []byte
}

type Result struct {
	Path    string
	Updated bool
}

func Install(cfg Config) (Result, error) {
	if cfg.Home == "" || cfg.Name == "" || len(cfg.Content) == 0 {
		return Result{}, errors.New("skill installation requires home, name, and content")
	}
	dir := filepath.Join(cfg.Home, ".agents", "skills", cfg.Name)
	path := filepath.Join(dir, "SKILL.md")
	statePath := filepath.Join(dir, ".reffector-sha256")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return Result{Path: path}, fmt.Errorf("create skill directory: %w", err)
	}

	current, err := os.ReadFile(path)
	if err == nil && string(current) != string(cfg.Content) {
		managed, stateErr := managedCopy(statePath, current)
		if stateErr != nil {
			return Result{Path: path}, stateErr
		}
		if !managed {
			return Result{Path: path}, fmt.Errorf("refusing to overwrite existing unmanaged or modified skill: %s", path)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{Path: path}, fmt.Errorf("read existing skill: %w", err)
	}

	if string(current) != string(cfg.Content) {
		if err := os.WriteFile(path, cfg.Content, 0644); err != nil {
			return Result{Path: path}, fmt.Errorf("write skill: %w", err)
		}
	}
	if err := os.WriteFile(statePath, []byte(digest(cfg.Content)+"\n"), 0600); err != nil {
		return Result{Path: path}, fmt.Errorf("record installed skill: %w", err)
	}
	return Result{Path: path, Updated: string(current) != string(cfg.Content)}, nil
}

func managedCopy(statePath string, current []byte) (bool, error) {
	state, err := os.ReadFile(statePath)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read skill installation state: %w", err)
	}
	return strings.TrimSpace(string(state)) == digest(current), nil
}

func digest(content []byte) string {
	sum := sha256.Sum256(content)
	return fmt.Sprintf("%x", sum)
}
