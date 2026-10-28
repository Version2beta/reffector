package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestSignalArgumentMutationAndBounds(t *testing.T) {
	source, err := filepath.Abs("../../testdata/signal-arguments")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "signal-arguments")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	report, _ := effectResult(t, dir, "-all")
	inventory := inventoryByFunction(report)
	for _, api := range []string{"Notify", "Ignore", "Reset"} {
		effect := map[string]string{"Notify": "process.signal.register", "Ignore": "process.signal.ignore", "Reset": "process.signal.reset"}[api]
		for _, tc := range []struct {
			name              string
			effect, uncertain bool
		}{
			{"Mutated", true, true},
			{"PointerEmptyCustom", true, false},
			{"DirectUnsupported", false, false},
			{"DirectNil", false, false},
			{"DirectSupported", true, true},
			{"DirectMixed", true, true},
			{"DirectUnknown", true, true},
			{"NoArgs", true, false},
			{"NilSlice", true, false},
			{"EmptyLiteral", true, false},
			{"PointerPrefix", false, false},
			{"PointerSuffix", false, false},
			{"PointerFull", true, true},
			{"PointerEmptyRange", true, false},
			{"PointerMaxEmpty", true, false},
			{"PointerMaxUnsupported", false, false},
			{"PointerMaxSupported", true, true},
			{"DynamicLow", true, true},
			{"DynamicHigh", true, true},
			{"DynamicMax", true, true},
			{"Alias", false, false},
			{"AliasMutation", true, true},
			{"ReslicedAlias", true, true},
			{"Escape", true, true},
			{"UnknownMutator", true, true},
			{"CompetingRead", true, true},
			{"AfterCallMutation", true, true},
			{"ArrayCopy", true, true},
			{"Boundary", true, true},
			{"ConditionalMutation", true, true},
		} {
			t.Run(api+tc.name, func(t *testing.T) {
				record, ok := inventory[api+tc.name]
				if !ok {
					t.Fatal("missing inventory function")
				}
				want := []analyzer.Effect{}
				if tc.effect {
					want = append(want, analyzer.Effect{Identifier: effect, Direct: true})
				}
				if !reflect.DeepEqual(record.Effects, want) || record.Uncertain != tc.uncertain {
					t.Errorf("signal argument semantics: got %+v, want effects=%v uncertain=%t", record, want, tc.uncertain)
				}
				var sites []analyzer.Site
				for _, site := range report.Sites {
					if site.Function == record.Function && site.API == "os/signal."+api {
						sites = append(sites, site)
					}
				}
				if !tc.effect {
					if len(sites) != 0 {
						t.Errorf("no-effect control has operation sites: %+v", sites)
					}
					return
				}
				if len(sites) != 1 {
					t.Fatalf("want one signal origin, got %+v", sites)
				}
				site := sites[0]
				if !reflect.DeepEqual(site.Effects, []string{effect}) || site.Uncertain != tc.uncertain || len(site.Operations) != 1 || site.Operations[0].Uncertain != tc.uncertain {
					t.Errorf("signal site semantics: %+v", site)
				}
				if tc.uncertain && (!reflect.DeepEqual(site.Reasons, []string{"argument_condition"}) || !reflect.DeepEqual(site.Operations[0].Reasons, []string{"argument_condition"})) {
					t.Errorf("unsupported argument shape lost argument_condition: %+v", site)
				}
			})
		}
	}
	for _, api := range []string{"Notify", "Ignore", "Reset"} {
		got := inventory[api+"BoundaryCaller"]
		want := inventory[api+"Boundary"].Effects
		if len(want) != 1 {
			t.Fatalf("missing callee evidence: %+v", want)
		}
		want = []analyzer.Effect{{Identifier: want[0].Identifier, Direct: false}}
		if !reflect.DeepEqual(got.Effects, want) || !got.Uncertain {
			t.Errorf("fresh caller values must not erase the open callee boundary in %s: %+v", api, got)
		}
	}
	for _, tc := range []struct {
		name              string
		effect, uncertain bool
	}{{"NotifyNilChannel", false, false}, {"NotifyUnknownChannel", true, true}} {
		got := inventory[tc.name]
		if (len(got.Effects) > 0) != tc.effect || got.Uncertain != tc.uncertain {
			t.Errorf("channel precondition %s: %+v", tc.name, got)
		}
	}
}
