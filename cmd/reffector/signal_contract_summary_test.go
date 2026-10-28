package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestQualifiedSignalRegistrationContract(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "signal-contracts")
	if err := os.CopyFS(dir, os.DirFS(signalContractSource)); err != nil {
		t.Fatal(err)
	}
	report, output := effectResult(t, dir, "-all")
	repeated, repeatedOutput := effectResult(t, dir, "-all")
	if output != repeatedOutput || !reflect.DeepEqual(report.Sites, repeated.Sites) {
		t.Fatal("signal registration report is not deterministic")
	}
	if report.Coverage.OperationSites != 32 || report.Coverage.SelectedOperationSites != 32 {
		t.Fatalf("recognized operation-site coverage: %+v", report.Coverage)
	}
	if report.Coverage.UnresolvedSites != 4 {
		t.Fatalf("unresolved related signal APIs: coverage=%+v", report.Coverage)
	}
	unresolved, _ := effectResult(t, dir, "-all", "-unresolved")
	if len(unresolved.Sites) != 4 || len(unresolved.Unresolved) != 4 {
		t.Fatalf("unresolved related signal sites = %+v", unresolved.Sites)
	}
	unresolvedFunctions := map[string]bool{}
	for _, site := range unresolved.Sites {
		unresolvedFunctions[site.Function] = true
	}
	for _, function := range []string{"NilContext", "CallReturnedContextStop", "MixedContextStop", "InvokeArbitraryStop"} {
		if !unresolvedFunctions[function] {
			t.Errorf("expected unresolved signal/function-value origin in %s: %v", function, unresolvedFunctions)
		}
	}

	inventory := map[string]analyzer.Record{}
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	hasEffect := func(record analyzer.Record, identifier string) bool {
		for _, effect := range record.Effects {
			if effect.Identifier == identifier {
				return true
			}
		}
		return false
	}
	registerEffect := []analyzer.Effect{{Identifier: "process.signal.register", Direct: true}}
	if got := inventory["Register"]; !reflect.DeepEqual(got.Effects, registerEffect) || !got.Uncertain {
		t.Errorf("Notify with a possibly nil channel lost uncertainty: %+v", got)
	}
	if got := inventory["RegisterAll"]; !reflect.DeepEqual(got.Effects, registerEffect) || got.Uncertain {
		t.Errorf("proven all-signal registration = %+v", got)
	}
	for _, function := range []string{"RegisterFresh", "RegisterSyscall", "RegisterUnknown", "MixedSignal", "OverrideInterrupt"} {
		if got := inventory[function]; !reflect.DeepEqual(got.Effects, registerEffect) || !got.Uncertain {
			t.Errorf("possible registration %s lost uncertainty: %+v", function, got)
		}
	}
	if got := inventory["Caller"]; !reflect.DeepEqual(got.Effects, []analyzer.Effect{{Identifier: "process.signal.register", Direct: false}}) || !got.Uncertain {
		t.Errorf("inherited possible registration evidence = %+v", got)
	}
	for _, function := range []string{"NilChannel", "UnsupportedSignal", "FakeNotify"} {
		if got := inventory[function]; len(got.Effects) != 0 || got.Uncertain {
			t.Errorf("non-registration %s was reported as registered: %+v", function, got)
		}
	}
	policyEffects := map[string]string{
		"IgnoreSignal": "process.signal.ignore", "ResetSignals": "process.signal.reset",
		"IgnoreAll": "process.signal.ignore", "ResetAll": "process.signal.reset",
		"IgnoreNilSlice": "process.signal.ignore", "ResetNilSlice": "process.signal.reset",
		"IgnoreDynamic": "process.signal.ignore", "ResetDynamic": "process.signal.reset",
		"IgnoreSignalSlice": "process.signal.ignore", "ResetSignalSlice": "process.signal.reset",
		"IgnoreMixedSignals": "process.signal.ignore", "ResetMixedSignals": "process.signal.reset",
	}
	for function, effect := range policyEffects {
		got := inventory[function]
		if len(got.Effects) != 1 || got.Effects[0].Identifier != effect || !got.Effects[0].Direct {
			t.Errorf("qualified signal policy effect in %s = %+v, want %s", function, got, effect)
		}
		wantsUncertain := function != "IgnoreAll" && function != "ResetAll" && function != "IgnoreNilSlice" && function != "ResetNilSlice"
		if got.Uncertain != wantsUncertain {
			t.Errorf("signal policy precondition certainty in %s = %t, want %t: %+v", function, got.Uncertain, wantsUncertain, got)
		}
	}
	for _, function := range []string{"IgnoreUnsupported", "ResetUnsupported", "IgnoreNilSignal", "ResetNilSignal", "FakeIgnore", "FakeReset"} {
		if got := inventory[function]; len(got.Effects) != 0 || got.Uncertain {
			t.Errorf("unsupported signal or unrelated policy method %s was characterized: %+v", function, got)
		}
		for _, site := range report.Sites {
			if site.Function == function {
				t.Errorf("unsupported signal or unrelated policy method %s became an operation site: %+v", function, site)
			}
		}
	}
	for _, function := range []string{"Unregister", "UnregisterFresh", "UnregisterAfterNotify"} {
		got := inventory[function]
		if !hasEffect(got, "process.signal.unregister") || !got.Uncertain {
			t.Errorf("Stop call overstated prior registration or lost possible unregister evidence in %s: %+v", function, got)
		}
	}
	if got := inventory["UnregisterAfterNotify"]; !hasEffect(got, "process.signal.register") {
		t.Errorf("same-function Notify registration evidence missing: %+v", got)
	}
	for _, function := range []string{"UnregisterNil", "FakeStop"} {
		if got := inventory[function]; len(got.Effects) != 0 || got.Uncertain {
			t.Errorf("no-op or unrelated Stop call was summarized in %s: %+v", function, got)
		}
		for _, site := range report.Sites {
			if site.Function == function {
				t.Errorf("no-op or unrelated Stop origin became an operation site: %+v", site)
			}
		}
	}

	var notifySites int
	for _, site := range report.Sites {
		for _, operation := range site.Operations {
			if operation.API != "os/signal.Notify" {
				continue
			}
			notifySites++
			if site.Kind != "call" || site.Location.Line < 1 || site.Location.Column < 1 ||
				operation.Recognition != "summary" || !reflect.DeepEqual(operation.Effects, []string{"process.signal.register"}) {
				t.Errorf("qualified Notify site evidence = %+v", site)
			}
		}
	}
	if notifySites != 8 {
		t.Fatalf("recognized Notify sites = %d, want 8", notifySites)
	}
	policySites := map[string]string{}
	for _, site := range report.Sites {
		for _, operation := range site.Operations {
			if operation.API != "os/signal.Ignore" && operation.API != "os/signal.Reset" {
				continue
			}
			policySites[site.Function] = operation.API
			wantEffect := "process.signal.ignore"
			if operation.API == "os/signal.Reset" {
				wantEffect = "process.signal.reset"
			}
			if site.API != operation.API || site.Kind != "call" || site.Location.Line < 1 || site.Location.Column < 1 ||
				operation.Recognition != "summary" || !reflect.DeepEqual(operation.Effects, []string{wantEffect}) {
				t.Errorf("qualified signal policy site evidence = %+v", site)
			}
			wantsUncertain := site.Function != "IgnoreAll" && site.Function != "ResetAll" && site.Function != "IgnoreNilSlice" && site.Function != "ResetNilSlice"
			if site.Uncertain != wantsUncertain || operation.Uncertain != wantsUncertain {
				t.Errorf("signal policy site uncertainty in %s = site:%t operation:%t, want %t", site.Function, site.Uncertain, operation.Uncertain, wantsUncertain)
			}
			if wantsUncertain && (!reflect.DeepEqual(site.Reasons, []string{"argument_condition"}) || !reflect.DeepEqual(operation.Reasons, []string{"argument_condition"})) {
				t.Errorf("signal policy uncertainty reason in %s = site:%v operation:%v", site.Function, site.Reasons, operation.Reasons)
			}
		}
	}
	if len(policySites) != len(policyEffects) {
		t.Errorf("recognized Ignore/Reset sites = %v, want %d", policySites, len(policyEffects))
	}
	stopEffects := []string{"context.cancel", "process.signal.unregister"}
	stopSites := map[string]string{}
	directStopSites := map[string]analyzer.Site{}
	contextSites := map[string]analyzer.Site{}
	for _, site := range report.Sites {
		for _, operation := range site.Operations {
			if operation.API == "os/signal.Stop" {
				directStopSites[site.Function] = site
				if site.API != "os/signal.Stop" || operation.Recognition != "summary" ||
					!site.Uncertain || !operation.Uncertain || site.Kind != "call" ||
					site.Location.File == "" || site.Location.Line < 1 || site.Location.Column < 1 ||
					!reflect.DeepEqual(site.Reasons, []string{"state_condition"}) ||
					!reflect.DeepEqual(operation.Reasons, []string{"state_condition"}) ||
					!reflect.DeepEqual(operation.Effects, []string{"process.signal.unregister"}) {
					t.Errorf("direct Stop evidence claims unknown registration state as certain: %+v", site)
				}
			}
			if operation.API == "os/signal.NotifyContext.stop" {
				stopSites[site.Function] = site.Kind
				if site.Uncertain || operation.Uncertain || site.Location.Line < 1 || site.Location.Column < 1 ||
					!reflect.DeepEqual(operation.Effects, stopEffects) {
					t.Errorf("proven NotifyContext stop site = %+v", site)
				}
			}
			if operation.API == "os/signal.NotifyContext" {
				contextSites[site.Function] = site
			}
		}
	}
	if !reflect.DeepEqual(stopSites, map[string]string{"RegisterContext": "defer", "RegisterContextFresh": "call", "UnsupportedContext": "call"}) {
		t.Errorf("recognized NotifyContext stop invocations = %v", stopSites)
	}
	if len(directStopSites) != 3 {
		t.Errorf("exact direct Stop call sites = %v", directStopSites)
	}
	for _, function := range []string{"Unregister", "UnregisterFresh", "UnregisterAfterNotify"} {
		if _, ok := directStopSites[function]; !ok {
			t.Errorf("direct Stop site missing in %s: %v", function, directStopSites)
		}
	}
	for _, function := range []string{"RegisterContext", "RegisterContextFresh", "RegisterContextAll", "UnsupportedContext", "ReturnContextStop", "MixedContextStop"} {
		site, ok := contextSites[function]
		if !ok || !site.Uncertain || !reflect.DeepEqual(site.Reasons, []string{"argument_condition"}) {
			t.Errorf("NotifyContext constructor evidence for %s = %+v", function, site)
		}
		if ok {
			wantsRegistration := function != "UnsupportedContext"
			var effects []string
			for _, operation := range site.Operations {
				effects = append(effects, operation.Effects...)
			}
			hasRegistration := false
			hasGoroutine := false
			for _, effect := range effects {
				hasRegistration = hasRegistration || effect == "process.signal.register"
				hasGoroutine = hasGoroutine || effect == "concurrency.goroutine"
			}
			if hasRegistration != wantsRegistration || !hasGoroutine {
				t.Errorf("NotifyContext constructor effects for %s = %v", function, effects)
			}
		}
	}
	for _, function := range []string{"ReturnContextStop", "CallReturnedContextStop", "MixedContextStop", "InvokeArbitraryStop", "NilContext"} {
		if record := inventory[function]; !record.Uncertain || hasEffect(record, "process.signal.unregister") || hasEffect(record, "context.cancel") {
			t.Errorf("unproven NotifyContext stop gained lifecycle effects in %s: %+v", function, record)
		}
	}
	if record := inventory["CallReturnedArbitraryStop"]; len(record.Effects) != 0 || record.Uncertain {
		t.Errorf("known arbitrary factory must resolve without cancellation effects: %+v", record)
	}
	if record := inventory["LocalArbitraryStop"]; len(record.Effects) != 0 || record.Uncertain {
		t.Errorf("known local arbitrary CancelFunc was misclassified: %+v", record)
	}
	for _, site := range report.Sites {
		if site.Function == "Register" || site.Function == "RegisterFresh" || site.Function == "RegisterSyscall" ||
			site.Function == "RegisterUnknown" || site.Function == "MixedSignal" || site.Function == "OverrideInterrupt" {
			if !site.Uncertain || len(site.Operations) != 1 || !site.Operations[0].Uncertain ||
				!reflect.DeepEqual(site.Reasons, []string{"argument_condition"}) {
				t.Errorf("unknown registration preconditions not retained accurately: %+v", site)
			}
		}
	}
}
