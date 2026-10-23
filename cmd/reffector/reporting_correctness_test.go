package main

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

// Phase 1 A & B: Effect-filtered function summaries and direct-only queries.
func TestEffectFilteredFunctionSummariesAndDirectQueries(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/filtertest\ngo 1.26\n")
	mustWrite(t, filepath.Join(dir, "filter.go"), `package filtertest
import "os"

type Backend interface { Save() error }
type writer struct{}
func (writer) Save() error { return os.WriteFile("x", nil, 0600) }

func getBackend() Backend { return writer{} }
func EstablishedOnly() error { _, err := os.ReadFile("direct"); return err }
func CandidateOnly() error { return getBackend().Save() }
func BothKinds() error {
	_, _ = os.ReadFile("direct")
	return getBackend().Save()
}
func UnrelatedEffect() error { return os.Setenv("FOO", "BAR") }
`)

	getSummaries := func(args ...string) map[string]analyzer.Record {
		text := success(t, dir, append([]string{"-view", "functions", "-json"}, args...)...)
		var list []analyzer.Record
		if err := json.Unmarshal([]byte(text), &list); err != nil {
			t.Fatalf("json decode failed: %v", err)
		}
		res := make(map[string]analyzer.Record, len(list))
		for _, r := range list {
			res[r.Function] = r
		}
		return res
	}

	// 1. Query for -effect fs.write: CandidateOnly and BothKinds must match and be emitted.
	// EstablishedOnly (fs.read only) and UnrelatedEffect (env.write only) must not appear.
	writeFiltered := getSummaries("-effect", "fs.write")
	if _, ok := writeFiltered["EstablishedOnly"]; ok {
		t.Fatalf("EstablishedOnly (fs.read) should not match -effect fs.write: %+v", writeFiltered)
	}
	if _, ok := writeFiltered["UnrelatedEffect"]; ok {
		t.Fatalf("UnrelatedEffect should not match -effect fs.write: %+v", writeFiltered)
	}
	cand, ok := writeFiltered["CandidateOnly"]
	if !ok {
		t.Fatalf("CandidateOnly (possible fs.write) disappeared from -effect fs.write query: %+v", writeFiltered)
	}
	if len(cand.Effects) != 0 || !reflect.DeepEqual(cand.PossibleEffects, []string{"fs.write"}) {
		t.Fatalf("CandidateOnly unexpected effects: %+v", cand)
	}
	both, ok := writeFiltered["BothKinds"]
	if !ok {
		t.Fatalf("BothKinds disappeared from -effect fs.write query: %+v", writeFiltered)
	}
	// Established fs.read should be filtered out by -effect fs.write; possible fs.write remains.
	if len(both.Effects) != 0 || !reflect.DeepEqual(both.PossibleEffects, []string{"fs.write"}) {
		t.Fatalf("BothKinds unexpected effects under -effect fs.write: %+v", both)
	}

	// 2. Query for -effect fs.read: EstablishedOnly and BothKinds must match; CandidateOnly must not.
	readFiltered := getSummaries("-effect", "fs.read")
	if _, ok := readFiltered["CandidateOnly"]; ok {
		t.Fatalf("CandidateOnly should not match -effect fs.read: %+v", readFiltered)
	}
	est, ok := readFiltered["EstablishedOnly"]
	if !ok || len(est.Effects) != 1 || est.Effects[0].Identifier != "fs.read" || len(est.PossibleEffects) != 0 {
		t.Fatalf("EstablishedOnly unexpected under -effect fs.read: %+v", est)
	}
	bothRead, ok := readFiltered["BothKinds"]
	if !ok || len(bothRead.Effects) != 1 || bothRead.Effects[0].Identifier != "fs.read" || len(bothRead.PossibleEffects) != 0 {
		t.Fatalf("BothKinds unexpected under -effect fs.read: %+v", bothRead)
	}

	unfiltered := getSummaries()["BothKinds"]
	if !reflect.DeepEqual(unfiltered.Effects, []analyzer.Effect{{Identifier: "fs.read", Direct: true}}) || !reflect.DeepEqual(unfiltered.PossibleEffects, []string{"fs.create", "fs.write"}) {
		t.Fatalf("mixed unfiltered effects lost labels: %+v", unfiltered)
	}

	// 3. Direct-only queries: -direct must never report indirect VTA candidate effects as direct.
	directAll := getSummaries("-direct")
	candDirect := directAll["CandidateOnly"]
	if len(candDirect.PossibleEffects) != 0 || len(candDirect.Effects) != 0 {
		t.Fatalf("CandidateOnly had effects under -direct: %+v", candDirect)
	}
	bothDirect := directAll["BothKinds"]
	if len(bothDirect.PossibleEffects) != 0 || len(bothDirect.Effects) != 1 || bothDirect.Effects[0].Identifier != "fs.read" {
		t.Fatalf("BothKinds should only have direct fs.read under -direct: %+v", bothDirect)
	}

	// Direct + effect filter: CandidateOnly has no direct fs.write, so it must not be emitted.
	directWrite := getSummaries("-direct", "-effect", "fs.write")
	if _, ok := directWrite["CandidateOnly"]; ok {
		t.Fatalf("CandidateOnly must not be emitted under -direct -effect fs.write: %+v", directWrite)
	}
	if _, ok := directWrite["BothKinds"]; ok {
		t.Fatalf("BothKinds must not be emitted under -direct -effect fs.write: %+v", directWrite)
	}

	for _, function := range []string{"CandidateOnly", "BothKinds"} {
		r, _ := effectResult(t, dir, "-direct", "-function", "="+function, "-effect", "fs.write")
		if len(r.Groups) != 0 || len(r.Sites) != 0 {
			t.Fatalf("%s acquired indirect operations under -direct: %+v", function, r)
		}
	}
	r, _ := effectResult(t, dir, "-direct", "-function", "=BothKinds")
	if len(r.Groups) != 1 || r.Groups[0].Effect != "fs.read" || len(r.Sites) != 1 {
		t.Fatalf("direct effect view should retain only the read: %+v", r)
	}
	r, _ = effectResult(t, dir, "-direct", "-function", "=(writer).Save", "-effect", "fs.write")
	if len(r.Groups) != 1 || r.Groups[0].Effect != "fs.write" {
		t.Fatalf("direct writer operation lost: %+v", r)
	}
}

// Phase 1 C: Interface-contract aggregation across multiple SSA variants sharing a source call site.
func TestInterfaceContractAggregationAcrossVariants(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/variants\ngo 1.26\n")
	mustWrite(t, filepath.Join(dir, "variants.go"), `package variants
import "os"

type Backend interface { Save() error }
type DiskBackend struct{}
func (DiskBackend) Save() error { return os.WriteFile("disk", nil, 0600) }
type MemBackend struct{}
func (MemBackend) Save() error { return nil }

type Runner[T any] struct{}
func (Runner[T]) Run(b Backend) error {
	return b.Save() // Single source call site in generic method Runner[T].Run
}

func CallA() error { return Runner[int]{}.Run(DiskBackend{}) }
func CallB() error { return Runner[string]{}.Run(MemBackend{}) }
`)

	r, _ := effectResult(t, dir, "-contracts", "-unresolved", "-reason", "interface_dispatch", "-function", "Run")
	if len(r.Contracts) != 1 {
		t.Fatalf("expected 1 aggregated contract, got %d: %#v", len(r.Contracts), r.Contracts)
	}
	contract := r.Contracts[0]
	byFunc := map[string]string{}
	for _, impl := range contract.Implementations {
		byFunc[impl.Symbol.Function] = impl.Relationship
	}
	// Both DiskBackend and MemBackend were passed across different generic variants of the same call.
	// Both must be recognized as candidate_vta, not dropped due to single-site preparation stopping early.
	if byFunc["(DiskBackend).Save"] != "candidate_vta" {
		t.Fatalf("DiskBackend.Save was not candidate_vta: got %q in %#v", byFunc["(DiskBackend).Save"], byFunc)
	}
	if byFunc["(MemBackend).Save"] != "candidate_vta" {
		t.Fatalf("MemBackend.Save was not candidate_vta: got %q in %#v", byFunc["(MemBackend).Save"], byFunc)
	}
	if !contract.Open {
		t.Fatal("generic aggregation closed an open receiver boundary")
	}
	_, baseline := effectResult(t, dir, "-contracts", "-unresolved", "-reason", "interface_dispatch", "-function", "Run")
	for i := 0; i < 3; i++ {
		_, next := effectResult(t, dir, "-contracts", "-unresolved", "-reason", "interface_dispatch", "-function", "Run")
		if next != baseline {
			t.Fatal("generic report is not deterministic in the same source directory")
		}
	}
}

// Phase 1 D: Candidate effects within contract implementations.
func TestCandidateEffectsWithinContractImplementations(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/candcontract\ngo 1.26\n")
	mustWrite(t, filepath.Join(dir, "cand.go"), `package candcontract
import "os"

type Target interface { Do() error }
type ConcreteTarget struct{}
func (ConcreteTarget) Do() error { return os.WriteFile("deep", nil, 0600) }

type Service interface { Run() error }

// IndirectService has no direct or established effects, but its Do() call
// reaches ConcreteTarget via VTA candidate.
type IndirectService struct{ target Target }
func (s IndirectService) Run() error {
	return s.target.Do()
}

type DirectService struct{}
func (DirectService) Run() error {
	return os.Remove("direct")
}

func RunService(s Service) error {
	return s.Run()
}

func UseIndirect() error {
	return RunService(IndirectService{target: ConcreteTarget{}})
}

func UseDirect() error {
	return RunService(DirectService{})
}
`)

	r, _ := effectResult(t, dir, "-contracts", "-unresolved", "-reason", "interface_dispatch", "-function", "RunService")
	var contract *analyzer.InterfaceContract
	for i := range r.Contracts {
		if r.Contracts[i].Method == "example.com/candcontract.(Service).Run" {
			contract = &r.Contracts[i]
			break
		}
	}
	if contract == nil {
		t.Fatalf("expected contract for Service.Run, got %#v", r.Contracts)
	}
	var indirect, direct *analyzer.ContractImplementation
	for i := range contract.Implementations {
		impl := &contract.Implementations[i]
		if impl.Symbol.Function == "(IndirectService).Run" {
			indirect = impl
		}
		if impl.Symbol.Function == "(DirectService).Run" {
			direct = impl
		}
	}
	if indirect == nil || direct == nil {
		t.Fatalf("missing implementations: indirect=%v direct=%v", indirect, direct)
	}

	// DirectService has established fs.remove.
	if !reflect.DeepEqual(direct.Effects, []string{"fs.remove"}) || len(direct.PossibleEffects) != 0 {
		t.Fatalf("DirectService unexpected effects: established=%v possible=%v", direct.Effects, direct.PossibleEffects)
	}

	// IndirectService must not be described as effect-free: its candidate effects must be populated
	// and distinguished from established effects.
	if len(indirect.Effects) != 0 {
		t.Fatalf("IndirectService should have no established effects: got %v", indirect.Effects)
	}
	if !reflect.DeepEqual(indirect.PossibleEffects, []string{"fs.create", "fs.write"}) {
		t.Fatalf("IndirectService must report candidate possible_effects [fs.create fs.write]: got %v", indirect.PossibleEffects)
	}
	text := success(t, dir, "-view", "effects", "-contracts", "-unresolved", "-reason", "interface_dispatch", "-function", "RunService")
	if !strings.Contains(text, "[candidate_vta; effects=; possible=fs.create,fs.write]") {
		t.Fatalf("text lost candidate effect distinction: %s", text)
	}
	without, _ := effectResult(t, dir, "-unresolved", "-reason", "interface_dispatch", "-function", "RunService")
	if !reflect.DeepEqual(r.Edges, without.Edges) || !reflect.DeepEqual(r.Nodes, without.Nodes) {
		t.Fatal("contract inventory fabricated execution graph evidence")
	}
}
