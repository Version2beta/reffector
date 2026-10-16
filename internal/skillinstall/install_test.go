package skillinstall

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInstallCreatesAndUpdatesManagedSkill(t *testing.T) {
	home := t.TempDir()
	first, err := Install(Config{Home: home, Name: "reffector", Content: []byte("first\n")})
	if err != nil || !first.Updated {
		t.Fatalf("first install: %+v, %v", first, err)
	}
	data, err := os.ReadFile(first.Path)
	if err != nil || string(data) != "first\n" {
		t.Fatalf("installed content: %q, %v", data, err)
	}
	if mode, err := os.Stat(first.Path); err != nil || mode.Mode().Perm() != 0644 {
		t.Fatalf("skill permissions: %v, %v", mode, err)
	}
	second, err := Install(Config{Home: home, Name: "reffector", Content: []byte("second\n")})
	if err != nil || !second.Updated {
		t.Fatalf("update: %+v, %v", second, err)
	}
	data, _ = os.ReadFile(second.Path)
	if string(data) != "second\n" {
		t.Fatalf("updated content: %q", data)
	}
	third, err := Install(Config{Home: home, Name: "reffector", Content: []byte("second\n")})
	if err != nil || third.Updated {
		t.Fatalf("repeat install: %+v, %v", third, err)
	}
}

func TestInstallRefusesUnmanagedOrModifiedSkill(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".agents", "skills", "reffector", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("custom\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Config{Home: home, Name: "reffector", Content: []byte("canonical\n")}); err == nil {
		t.Fatal("unmanaged skill was overwritten")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "custom\n" {
		t.Fatalf("custom skill changed: %q", data)
	}
}

func TestInstallReportsWriteFailure(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".agents", "skills", "reffector")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("not a directory"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(Config{Home: home, Name: "reffector", Content: []byte("skill\n")}); err == nil {
		t.Fatal("expected installation failure")
	}
}
