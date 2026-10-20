package analyzer

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSelectionPatterns(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0700); err != nil {
		t.Fatal(err)
	}
	input := []string{".", "sub", "./sub", "./sub/...", "example.com/p", "std", "file=p.go"}
	want := []string{"./...", "./sub/...", "./sub/...", "./sub/...", "example.com/p", "std", "file=p.go"}
	if got := SelectionPatterns(input, Options{Dir: dir}); !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
	want = []string{".", "./sub", "./sub", "./sub/...", "example.com/p", "std", "file=p.go"}
	if got := SelectionPatterns(input, Options{Dir: dir, Exact: true}); !reflect.DeepEqual(got, want) {
		t.Fatalf("%v", got)
	}
}
