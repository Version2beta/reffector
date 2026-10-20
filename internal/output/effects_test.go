package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestCategoryReferencesIdentifyOperationsWithoutFollowingIDs(t *testing.T) {
	const id = "site:shared"
	r := &analyzer.EffectReport{
		Groups: []analyzer.EffectGroup{
			{Effect: "fs.read", Sites: []string{id}},
			{Effect: "resource.acquire", Sites: []string{id}},
		},
		Sites: []analyzer.Site{{
			ID: id, Symbol: analyzer.Symbol{Package: "example.com/store", Function: "Load"},
			APIs: []string{"os.Open"}, Location: analyzer.Location{File: "store.go", Line: 12, Column: 8},
		}},
	}
	var out bytes.Buffer
	if err := WriteEffects(&out, r, false); err != nil {
		t.Fatal(err)
	}
	header, _, found := strings.Cut(out.String(), "\n"+id+"\n")
	if !found {
		t.Fatal("missing canonical site definition")
	}
	for _, effect := range []string{"fs.read", "resource.acquire"} {
		want := effect + " (1 sites)\n  os.Open — store.go:12:8 in example.com/store.Load\n    " + id
		if !strings.Contains(header, want) {
			t.Fatalf("category lacks operation evidence %q:\n%s", want, header)
		}
	}
	if strings.Count(out.String(), "\n"+id+"\n") != 1 {
		t.Fatal("multi-category references duplicated the canonical definition")
	}
}
