package analyzer

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type auditFailWriter struct{}

func (auditFailWriter) Write([]byte) (int, error) { return 0, errors.New("audit write failed") }

func TestAuditTestVariantsMergeOnlySourceEvidenceWithoutReverseEdges(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"go.mod":    "module example.com/auditvariants\ngo 1.26\n",
		"p.go":      "package p\nfunc Unknown(fn func()) { fn() }\n",
		"p_test.go": "package p\nfunc Extra(fn func()) { Unknown(fn) }\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	r, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, Tests: true, AuditUnresolved: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Audit.sites) != 1 {
		t.Fatalf("test variants created %d source sites", len(r.Audit.sites))
	}
	id := r.Audit.sites[0]
	if len(r.Audit.g.siteNodes[id]) < 2 {
		t.Fatal("SSA test variants lost")
	}
	if len(r.Audit.g.edges) != 0 {
		t.Fatal("audit built source reverse edges")
	}
	for _, n := range r.Audit.g.nodes {
		if len(n.incoming) != 0 {
			t.Fatal("audit built reverse traversal links")
		}
	}
	var out bytes.Buffer
	if err := r.Audit.WriteJSONL(&out); err != nil {
		t.Fatal(err)
	}
	if bytes.Count(out.Bytes(), []byte("\n")) != 1 {
		t.Fatal("source site duplicated")
	}
	if err := r.Audit.WriteJSONL(auditFailWriter{}); err == nil {
		t.Fatal("write failure ignored")
	}
}
