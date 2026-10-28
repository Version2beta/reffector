package main

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestQualifiedX509CertPoolOperationContracts(t *testing.T) {
	source, err := filepath.Abs("../../testdata/x509-certpool")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "x509-certpool")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}

	report, output := effectResult(t, dir, "-all")
	repeated, repeatedOutput := effectResult(t, dir, "-all")
	if output != repeatedOutput || !reflect.DeepEqual(report.Sites, repeated.Sites) {
		t.Fatal("crypto/x509 CertPool report is not deterministic")
	}
	if report.Coverage.OperationSites != 7 || report.Coverage.SelectedOperationSites != 7 || report.Coverage.UnresolvedSites != 2 {
		t.Fatalf("unique operation-site coverage: %+v", report.Coverage)
	}

	want := map[string]struct {
		effects   []string
		reasons   []string
		uncertain bool
	}{
		"crypto/x509.(*CertPool).AddCert":               {effects: []string{"crypto.x509.certpool.add"}, reasons: []string{}},
		"crypto/x509.(*CertPool).AppendCertsFromPEM":    {effects: []string{"crypto.x509.certpool.add"}, reasons: []string{"argument_condition"}, uncertain: true},
		"crypto/x509.(*CertPool).AddCertWithConstraint": {effects: []string{"crypto.x509.certpool.add"}, reasons: []string{"callback"}, uncertain: true},
		"crypto/x509.SystemCertPool":                    {effects: []string{"crypto.x509.certpool.system"}, reasons: []string{}},
	}
	counts := map[string]int{}
	var callbackWriteSite string
	for _, site := range report.Sites {
		if site.API == "os.WriteFile" {
			callbackWriteSite = site.ID
			continue
		}
		if _, ok := want[site.API]; !ok {
			t.Errorf("unexpected recognized operation site: %+v", site)
			continue
		}
		counts[site.API]++
		if site.Kind != "call" || len(site.Operations) != 1 {
			t.Errorf("CertPool operation provenance = %+v", site)
			continue
		}
		operation := site.Operations[0]
		wantAPI := want[site.API]
		if operation.API != site.API || operation.Recognition != "summary" ||
			!reflect.DeepEqual(operation.Effects, wantAPI.effects) ||
			!reflect.DeepEqual(operation.Reasons, wantAPI.reasons) ||
			operation.Uncertain != wantAPI.uncertain || site.Uncertain != wantAPI.uncertain {
			t.Errorf("CertPool evidence for %s = %+v, want effects=%v reasons=%v uncertain=%t", site.API, operation, wantAPI.effects, wantAPI.reasons, wantAPI.uncertain)
		}
	}
	if counts["crypto/x509.(*CertPool).AddCert"] != 3 ||
		counts["crypto/x509.(*CertPool).AppendCertsFromPEM"] != 1 ||
		counts["crypto/x509.(*CertPool).AddCertWithConstraint"] != 1 ||
		counts["crypto/x509.SystemCertPool"] != 1 {
		t.Errorf("qualified CertPool APIs = %v", counts)
	}
	if callbackWriteSite == "" {
		t.Fatal("opaque constraint callback's filesystem operation was not retained at its own source site")
	}
	for _, site := range report.Sites {
		if site.ID == callbackWriteSite && (site.Function != "PoolOperations$1" ||
			!reflect.DeepEqual(site.Effects, []string{"fs.create", "fs.write"})) {
			t.Errorf("constraint callback operation provenance = %+v", site)
		}
	}

	filtered, _ := effectResult(t, dir, "-effect", "crypto.x509.certpool.add", "-max-depth", "0")
	if len(filtered.Sites) != 5 {
		t.Errorf("CertPool addition filter returned %d sites, want 5", len(filtered.Sites))
	}
	systemRoots, _ := effectResult(t, dir, "-effect", "crypto.x509.certpool.system", "-max-depth", "0")
	if len(systemRoots.Sites) != 1 || systemRoots.Sites[0].API != "crypto/x509.SystemCertPool" {
		t.Errorf("system roots effect filter returned %+v", systemRoots.Sites)
	}

	inventory := make(map[string]analyzer.Record)
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	for _, function := range []string{"PoolOperations", "Caller"} {
		record := inventory[function]
		if !hasEffect(record, "crypto.x509.certpool.add") {
			t.Errorf("%s lacks CertPool addition evidence: %+v", function, record)
		}
	}
	poolOperations := inventory["PoolOperations"]
	if len(poolOperations.Effects) != 1 || poolOperations.Effects[0].Identifier != "crypto.x509.certpool.add" ||
		!poolOperations.Effects[0].Direct || !poolOperations.Uncertain {
		t.Errorf("direct CertPool summary lost possible callback/argument uncertainty: %+v", poolOperations)
	}
	caller := inventory["Caller"]
	if len(caller.Effects) != 1 || caller.Effects[0].Identifier != "crypto.x509.certpool.add" ||
		caller.Effects[0].Direct || caller.Uncertain {
		t.Errorf("inherited AddCert evidence changed certainty: %+v", caller)
	}
	if roots := inventory["SystemRoots"]; len(roots.Effects) != 1 || roots.Effects[0].Identifier != "crypto.x509.certpool.system" || !roots.Effects[0].Direct || roots.Uncertain {
		t.Errorf("SystemCertPool evidence = %+v", roots)
	}
	for _, function := range []string{"NewPool", "SameNamedMethods"} {
		record := inventory[function]
		if len(record.Effects) != 0 || record.Uncertain {
			t.Errorf("negative control %s has CertPool operation evidence: %+v", function, record)
		}
	}
	if provider := inventory["UseProvider"]; len(provider.Effects) != 0 || !provider.Uncertain {
		t.Errorf("application-defined CertPool interface was summarized by method name: %+v", provider)
	}
	if adder := inventory["UseAdder"]; hasEffect(adder, "crypto.x509.certpool.add") ||
		!containsString(adder.PossibleEffects, "crypto.x509.certpool.add") || !adder.Uncertain {
		t.Errorf("default interface candidate was promoted or uncertainty lost: %+v", adder)
	}
	unresolved, _ := effectResult(t, dir, "-unresolved", "-max-depth", "0")
	unresolvedAPIs := make(map[string][]string)
	for _, site := range unresolved.Sites {
		unresolvedAPIs[site.API] = site.Reasons
	}
	if len(unresolvedAPIs) != 2 ||
		!reflect.DeepEqual(unresolvedAPIs["x509certpoolfixture.(Provider).CertPool"], []string{"interface_dispatch"}) ||
		!reflect.DeepEqual(unresolvedAPIs["x509certpoolfixture.(CertPoolAdder).AddTo"], []string{"interface_dispatch"}) {
		t.Errorf("unmodeled standard and application interface contracts were not kept unresolved: %+v", unresolvedAPIs)
	}

	vtaArgs := []string{"-all", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100"}
	vta, _ := effectResult(t, dir, vtaArgs...)
	if stats := vta.Coverage.DependencyVTA; stats == nil || stats.Truncated || stats.ContextTruncated {
		t.Fatalf("bounded VTA unexpectedly truncated: %+v", stats)
	}
	if vta.Coverage.OperationSites != report.Coverage.OperationSites ||
		vta.Coverage.SelectedOperationSites != report.Coverage.SelectedOperationSites ||
		vta.Coverage.UnresolvedSites != report.Coverage.UnresolvedSites {
		t.Errorf("VTA changed source operation coverage: default=%+v opt-in=%+v", report.Coverage, vta.Coverage)
	}
	siteIDs := func(sites []analyzer.Site) []string {
		ids := make([]string, 0, len(sites))
		for _, site := range sites {
			ids = append(ids, site.ID)
		}
		sort.Strings(ids)
		return ids
	}
	if !reflect.DeepEqual(siteIDs(report.Sites), siteIDs(vta.Sites)) {
		t.Errorf("VTA changed recognized operation-site IDs: default=%v opt-in=%v", siteIDs(report.Sites), siteIDs(vta.Sites))
	}
	vtaRecords := inventoryByFunction(vta)
	for _, function := range []string{"UseAdder", "CandidatePath"} {
		record := vtaRecords[function]
		if hasEffect(record, "crypto.x509.certpool.add") || !containsString(record.PossibleEffects, "crypto.x509.certpool.add") || !record.Uncertain {
			t.Errorf("candidate CertPool effect was promoted or lost uncertainty in %s: %+v", function, record)
		}
	}
	candidateEdge := false
	for _, edge := range vta.Edges {
		if edge.Source != nil && edge.Source.Caller.Function == "UseAdder" &&
			strings.Contains(edge.Source.Callee.Function, "ExplicitAdder") && strings.Contains(edge.Source.Callee.Function, "AddTo") &&
			edge.Source.Resolution == "candidate_vta" && edge.Candidate {
			candidateEdge = true
		}
	}
	if !candidateEdge {
		t.Errorf("VTA candidate implementation edge was not kept candidate-only")
	}
	defaultUnknown, _ := effectResult(t, dir, "-all", "-unresolved")
	vtaUnknown, _ := effectResult(t, dir, "-all", "-unresolved", "-dependency-vta-functions", "100", "-dependency-vta-edges", "100")
	if !reflect.DeepEqual(defaultUnknown.Unresolved, vtaUnknown.Unresolved) || len(vtaUnknown.Unresolved) != 2 {
		t.Errorf("VTA changed unresolved operation-site IDs: default=%v opt-in=%v", defaultUnknown.Unresolved, vtaUnknown.Unresolved)
	}
}
