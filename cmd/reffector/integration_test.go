package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Version2beta/reffector/internal/analyzer"
)

type record struct {
	Package, Function, Effect string
	Direct                    bool
}

var executable, fixtureSource string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "reffector-cli-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	executable = filepath.Join(tmp, "reffector")
	fixtureSource, _ = filepath.Abs("../../testdata/fixture")
	cmd := exec.Command("go", "build", "-o", executable, ".")
	if data, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "building CLI: %v\n%s", err, data)
		_ = os.RemoveAll(tmp)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(tmp)
	os.Exit(code)
}

func invoke(t *testing.T, dir string, env []string, args ...string) (string, string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), "GOWORK=off"), env...)
	var out, diagnostic bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &diagnostic
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("CLI timed out: %v", ctx.Err())
	}
	if err == nil {
		return out.String(), diagnostic.String(), 0
	}
	if e, ok := err.(*exec.ExitError); ok {
		return out.String(), diagnostic.String(), e.ExitCode()
	}
	t.Fatal(err)
	return "", "", -1
}

func success(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, diagnostic, status := invoke(t, dir, nil, args...)
	if status != 0 || diagnostic != "" {
		t.Fatalf("%v: exit %d, stderr %s", args, status, diagnostic)
	}
	return out
}

func records(t *testing.T, text string) []record {
	t.Helper()
	var summaries []analyzer.Record
	if err := json.Unmarshal([]byte(text), &summaries); err != nil || summaries == nil {
		t.Fatalf("invalid JSON array: %v: %s", err, text)
	}
	var result []record
	for _, summary := range summaries {
		for _, effect := range summary.Effects {
			result = append(result, record{summary.Package, summary.Function, effect.Identifier, effect.Direct})
		}
	}
	return result
}

func copyFixture(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "fixture")
	if err := os.CopyFS(dir, os.DirFS(fixtureSource)); err != nil {
		t.Fatal(err)
	}
	return dir
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		key, _ := filepath.Rel(root, path)
		if d.IsDir() {
			result[key] = info.Mode().String()
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[key] = fmt.Sprintf("%s:%x", info.Mode(), sha256.Sum256(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestAnalysisCLI(t *testing.T) {
	dir := copyFixture(t)
	before := snapshot(t, dir)
	t.Cleanup(func() {
		if after := snapshot(t, dir); !reflect.DeepEqual(before, after) {
			t.Errorf("analyzed repository changed: before %v, after %v", before, after)
		}
	})
	base := success(t, dir, "-json", ".")
	all := records(t, base)
	actual := make(map[string]string)
	for i, r := range all {
		if r.Package != "example.com/effects" {
			t.Fatalf("displayed dependency: %+v", r)
		}
		if i > 0 {
			a, b := all[i-1], r
			if a.Package+"\x00"+a.Function+"\x00"+a.Effect >= b.Package+"\x00"+b.Function+"\x00"+b.Effect {
				t.Fatalf("unsorted or duplicate records: %+v %+v", a, b)
			}
		}
		if r.Function == "init" { // Dependency initialization varies by Go release.
			continue
		}
		if actual[r.Function] != "" {
			actual[r.Function] += " "
		}
		actual[r.Function] += r.Effect
	}
	want := map[string]string{
		"Read": "fs.read", "Write": "fs.create fs.write", "Remove": "fs.remove",
		"Multiple": "fs.create fs.read fs.write", "Middle": "fs.create fs.read fs.write", "Top": "fs.create fs.read fs.write",
		"Cross": "fs.read", "(*Pointer).Save": "fs.create fs.write", "(Value).Load": "fs.read", "(Other).Load": "env.write", "(private).unused": "fs.read",
		"KnownInterface": "fs.read",
		"Recursive":      "fs.read", "MutualA": "fs.create fs.write", "MutualB": "fs.create fs.write",

		"KnownValue": "fs.read", "PhiValue": "fs.create fs.read fs.write", "BoundMethod": "fs.create fs.write", "BoundSink": "fs.read",
		"Closure": "fs.read", "Closure$1": "fs.read", "Spawn": "fs.create fs.read fs.write",
		"Generic": "env.read", "(Box[T]).Load": "fs.read", "GenericUse": "env.read fs.read",
		"HTTP": "net.connect", "Connect": "net.connect", "Listen": "net.listen",
		"Exec": "env.read fs.read process.exec", "Environment": "env.read env.write", "Database": "db.exec db.query",
		"FileMethods": "fs.read fs.write", "OpenFile": "fs.read", "OpenFileUnknown": "fs.create fs.read fs.write", "OpenFileWrite": "fs.create fs.write", "Root": "fs.create fs.read fs.write", "Exit": "process.exit", "(fileWriter).Write": "fs.write", "WriteFile": "fs.write", "(stringer).String": "fs.read", "Fatal": "process.exit", "LoggerFatal": "process.exit",
	}
	if !reflect.DeepEqual(actual, want) {
		t.Errorf("effects mismatch\ngot:  %v\nwant: %v", actual, want)
	}
	var summaries []analyzer.Record
	if err := json.Unmarshal([]byte(base), &summaries); err != nil {
		t.Fatal(err)
	}
	uncertain := map[string]bool{}
	for _, summary := range summaries {
		uncertain[summary.Function] = summary.Uncertain
	}
	for _, name := range []string{"Unknown", "ForwardUnknown", "ReturnedValue", "Reflect", "External", "OpenInterface", "MissingInterface", "HTTP", "WriteOpen", "FormatStringer"} {
		if !uncertain[name] {
			t.Errorf("%s should be marked uncertain", name)
		}
	}
	for _, name := range []string{"Pure", "NotSinks", "KnownInterface", "Read", "OpenFile", "Format", "FormatInteger", "FormatMultiple", "TypeName", "WriteMemory"} {
		if uncertain[name] {
			t.Errorf("%s should not be marked uncertain", name)
		}
	}

	t.Run("direct", func(t *testing.T) {
		direct := records(t, success(t, dir, "-direct", "-json", "."))
		want := make([]record, 0)
		for _, r := range all {
			if r.Direct {
				want = append(want, r)
			}
		}
		if !reflect.DeepEqual(direct, want) {
			t.Fatalf("direct output differs from direct JSON metadata\ngot %v\nwant %v", direct, want)
		}
		for _, r := range direct {
			if r.Function == "Top" || r.Function == "Cross" || !r.Direct {
				t.Fatalf("inherited result in direct output: %+v", r)
			}
		}
	})
	t.Run("text and deterministic defaults", func(t *testing.T) {
		var want strings.Builder
		for _, r := range all {
			fmt.Fprintf(&want, "%s.%s %s\n", r.Package, r.Function, r.Effect)
		}
		if got := success(t, dir); got != want.String() {
			t.Fatalf("default text mismatch: %s", got)
		}
		for i := 0; i < 2; i++ {
			if got := success(t, dir, "-json", "."); got != base {
				t.Fatal("nondeterministic JSON")
			}
		}
		if got := success(t, dir, "-json", "example.com/effects"); got != base {
			t.Fatal("import path differs from .")
		}
	})
	t.Run("patterns", func(t *testing.T) {
		text := success(t, dir, "-json", "./...")
		if got := success(t, dir, "-json", ".", "./dep", "./opaque", "."); got != text {
			t.Fatal("overlapping patterns not deduplicated")
		}
		dep := records(t, success(t, dir, "-json", "./dep/..."))
		found := false
		for _, r := range dep {
			if r.Package != "example.com/effects/dep" {
				t.Fatalf("unexpected package %+v", r)
			}
			found = found || r.Function == "Load" && r.Effect == "fs.read"
		}
		if !found || !strings.Contains(text, `"package": "example.com/effects/dep"`) {
			t.Fatal("dependency pattern missing Load")
		}
	})
	t.Run("tests", func(t *testing.T) {
		if strings.Contains(base, "TestOnly") || strings.Contains(base, "TestExternal") {
			t.Fatal("tests included by default")
		}
		testRecords := records(t, success(t, dir, "-json", "-tests", "."))
		found := make(map[string]bool)
		seen := make(map[string]bool)
		for _, r := range testRecords {
			key := r.Package + "." + r.Function + " " + r.Effect
			if seen[key] || strings.HasSuffix(r.Package, ".test") {
				t.Fatalf("duplicate or generated test function: %+v", r)
			}
			seen[key] = true
			if r.Function == "TestOnly" && r.Package == "example.com/effects" && r.Effect == "env.write" && r.Direct {
				found["internal"] = true
			}
			if r.Function == "TestExternal" && r.Package == "example.com/effects_test" && r.Effect == "fs.remove" && r.Direct {
				found["external"] = true
			}
		}
		if len(found) != 2 {
			t.Fatalf("test functions missing: %v", found)
		}
	})
}

func TestSkillInstallCLI(t *testing.T) {
	home := t.TempDir()
	out, diagnostic, status := invoke(t, fixtureSource, []string{"HOME=" + home}, "skill", "install")
	if status != 0 || diagnostic != "" || !strings.Contains(out, "Installed skill:") {
		t.Fatalf("install: stdout %q stderr %q exit %d", out, diagnostic, status)
	}
	installed, err := os.ReadFile(filepath.Join(home, ".agents", "skills", "reffector", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := os.ReadFile("../../skills/reffector/SKILL.md")
	if err != nil || !bytes.Equal(installed, canonical) {
		t.Fatalf("skill content mismatch: %v", err)
	}
	out, diagnostic, status = invoke(t, fixtureSource, []string{"HOME=" + home}, "skill", "install")
	if status != 0 || diagnostic != "" || !strings.Contains(out, "Skill is current:") {
		t.Fatalf("repeat install: stdout %q stderr %q exit %d", out, diagnostic, status)
	}
	_, diagnostic, status = invoke(t, fixtureSource, nil, "skill", "remove")
	if status != 2 || !strings.Contains(diagnostic, "Usage: reffector skill install") {
		t.Fatalf("bad subcommand: stderr %q exit %d", diagnostic, status)
	}
}

func TestCanonicalModulePath(t *testing.T) {
	data, err := os.ReadFile("../../go.mod")
	if err != nil || !strings.HasPrefix(string(data), "module github.com/Version2beta/reffector\n") {
		t.Fatalf("module path: %q, %v", data, err)
	}
}

func TestCLIUsageAndErrors(t *testing.T) {
	for _, flag := range []string{"-h", "-help"} {
		out, diagnostic, status := invoke(t, fixtureSource, nil, flag)
		if out != "" || status != 0 || !strings.Contains(diagnostic, "Usage: reffector") {
			t.Fatalf("%s: stdout %q stderr %q exit %d", flag, out, diagnostic, status)
		}
	}
	for _, tc := range []struct {
		arg    string
		status int
	}{{"-invalid", 2}, {"./nonexistent", 1}, {"./missing/...", 1}} {
		out, diagnostic, status := invoke(t, fixtureSource, nil, tc.arg)
		if out != "" || status != tc.status || diagnostic == "" {
			t.Fatalf("%s: stdout %q stderr %q exit %d", tc.arg, out, diagnostic, status)
		}
	}
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/pure\n\ngo 1.24.0\n")
	mustWrite(t, filepath.Join(dir, "pure.go"), "package pure\nfunc Pure() int { return 1 }\n")
	if got := strings.TrimSpace(success(t, dir, "-json")); got != "[]" {
		t.Fatalf("pure JSON: %q", got)
	}
	if got := success(t, dir); got != "" {
		t.Fatalf("pure text: %q", got)
	}
	mustWrite(t, filepath.Join(dir, "broken.go"), "package pure\nfunc Broken() { missing() }\n")
	out, diagnostic, status := invoke(t, dir, nil)
	if status != 1 || out != "" || !strings.Contains(diagnostic, "undefined: missing") {
		t.Fatalf("type errors: %q %q %d", out, diagnostic, status)
	}
}

func mustWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceAndReadonlyModules(t *testing.T) {
	dir := copyFixture(t)
	root := filepath.Dir(dir)
	work := filepath.Join(root, "go.work")
	mustWrite(t, work, "go 1.26.0\nuse ./fixture\n")
	before := snapshot(t, root)
	out, diagnostic, status := invoke(t, dir, []string{"GOWORK=" + work}, "-direct", "./...")
	if status != 0 || diagnostic != "" || !strings.Contains(out, "example.com/effects.Read fs.read") {
		t.Fatalf("workspace: %q %q %d", out, diagnostic, status)
	}
	if !reflect.DeepEqual(before, snapshot(t, root)) {
		t.Fatal("workspace changed")
	}
	// Absolute use paths must also survive workspace rebasing.
	mustWrite(t, work, "go 1.26.0\nuse "+fmt.Sprintf("%q", dir)+"\n")
	_, diagnostic, status = invoke(t, dir, []string{"GOWORK=" + work})
	if status != 0 || diagnostic != "" {
		t.Fatalf("absolute workspace: %s (%d)", diagnostic, status)
	}

	// A caller's -mod=mod must not let go list repair the analyzed module.
	module := t.TempDir()
	mod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(module, "go.mod"), strings.Replace(string(mod), "module github.com/Version2beta/reffector", "module example.com/missing-sums", 1))
	mustWrite(t, filepath.Join(module, "main.go"), "package main\nimport _ \"golang.org/x/tools/go/packages\"\nfunc main() {}\n")
	before = snapshot(t, module)
	out, diagnostic, status = invoke(t, module, []string{"GOFLAGS=-mod=mod"})
	if status != 1 || out != "" || !strings.Contains(diagnostic, "go.sum") {
		t.Fatalf("missing sums: %q %q (%d)", out, diagnostic, status)
	}
	if !reflect.DeepEqual(before, snapshot(t, module)) {
		t.Fatal("missing-sum module was modified")
	}
	// Workspace loading can generate checksums even in readonly mode. Those
	// writes must go to the private workspace, not the analyzed repository.
	work = filepath.Join(module, "go.work")
	mustWrite(t, work, "go 1.26.0\nuse .\n")
	before = snapshot(t, module)
	_, diagnostic, status = invoke(t, module, []string{"GOWORK=" + work})
	if status != 0 || diagnostic != "" {
		t.Fatalf("workspace missing sums: %s (%d)", diagnostic, status)
	}
	if !reflect.DeepEqual(before, snapshot(t, module)) {
		t.Fatal("workspace checksums modified the analyzed repository")
	}
}

func TestDotlessModuleCrossPackagePropagation(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "dep", "dep.go"), `package dep
import "os"
func Load() { _, _ = os.ReadFile("never") }
`)
	mustWrite(t, filepath.Join(dir, "internal", "leaf", "leaf.go"), `package leaf
import "os"
func Load() { _, _ = os.ReadFile("never") }
`)
	mustWrite(t, filepath.Join(dir, "app", "app.go"), `package app
import (
 "example/dep"
 "example/internal/leaf"
)
func Process() { dep.Load() }
func InternalProcess() { leaf.Load() }
`)
	result := records(t, success(t, dir, "-json", "./..."))
	found := map[string]record{}
	for _, r := range result {
		found[r.Package+"."+r.Function] = r
	}
	for _, name := range []string{"example/app.Process", "example/app.InternalProcess"} {
		r, ok := found[name]
		if !ok || r.Effect != "fs.read" || r.Direct {
			t.Fatalf("dotless module did not inherit fs.read for %s: %+v", name, r)
		}
	}
	var summaries []analyzer.Record
	if err := json.Unmarshal([]byte(success(t, dir, "-json", "./...")), &summaries); err != nil {
		t.Fatal(err)
	}
	for _, summary := range summaries {
		if summary.Package == "example/app" && (summary.Function == "Process" || summary.Function == "InternalProcess") && summary.Uncertain {
			t.Fatalf("dotless application package marked uncertain: %+v", summary)
		}
	}
}

func TestRealPackageEndingInTestIsNotHidden(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/legitimate.test\n\ngo 1.26.0\n")
	mustWrite(t, filepath.Join(dir, "main.go"), "package main\nimport \"os\"\nfunc Load() { _, _ = os.ReadFile(\"never\") }\nfunc main() {}\n")
	for _, args := range [][]string{{}, {"-tests"}} {
		if out := success(t, dir, args...); !strings.Contains(out, "example.com/legitimate.test.Load fs.read\n") {
			t.Fatalf("real main package omitted with %v: %s", args, out)
		}
	}
}

func TestJSONDirectPrecedence(t *testing.T) {
	// The same effect reached both directly and indirectly gets one direct=true
	// record. This exercises the real executable, not output/table mocks.
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/precedence\n\ngo 1.24.0\n")
	mustWrite(t, filepath.Join(dir, "p.go"), `package p
import "os"
func Leaf() { _, _ = os.ReadFile("never") }
func Both() { Leaf(); _, _ = os.ReadFile("never") }
`)
	result := records(t, success(t, dir, "-json"))
	var got []record
	for _, r := range result {
		if r.Function == "Both" {
			got = append(got, r)
		}
	}
	sort.Slice(got, func(i, j int) bool { return got[i].Effect < got[j].Effect })
	want := []record{{Package: "example.com/precedence", Function: "Both", Effect: "fs.read", Direct: true}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("direct precedence: %v", got)
	}
}
