package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestQualifiedTimeTimerContracts(t *testing.T) {
	source, err := filepath.Abs("../../testdata/time-timer")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "time-timer")
	if err := os.CopyFS(dir, os.DirFS(source)); err != nil {
		t.Fatal(err)
	}
	report, output := effectResult(t, dir, "-all")
	repeated, repeatedOutput := effectResult(t, dir, "-all")
	if output != repeatedOutput || !reflect.DeepEqual(report.Sites, repeated.Sites) {
		t.Fatal("time.Timer report is not deterministic")
	}
	if report.Coverage.OperationSites != 20 || report.Coverage.SelectedOperationSites != 20 || report.Coverage.UnresolvedSites != 2 {
		t.Fatalf("unique recognized/unresolved operation-site coverage: %+v", report.Coverage)
	}

	inventory := map[string]analyzer.Record{}
	for _, record := range report.Inventory {
		inventory[record.Function] = record
	}
	contains := func(record analyzer.Record, effect string) bool {
		for _, got := range record.Effects {
			if got.Identifier == effect {
				return true
			}
		}
		return false
	}
	for _, function := range []string{"Schedule", "ScheduleInline", "ResetAfterFunc"} {
		record := inventory[function]
		if !contains(record, "concurrency.callback.register") || !record.Uncertain {
			t.Errorf("AfterFunc scheduling evidence in %s = %+v", function, record)
		}
	}
	if record := inventory["TimerCaller"]; !contains(record, "concurrency.timer.schedule") || len(record.Effects) != 1 || record.Effects[0].Direct {
		t.Errorf("timer scheduling was not inherited through the established helper call: %+v", record)
	}
	if record := inventory["ScheduleInline"]; contains(record, "channel.close") {
		t.Errorf("asynchronous callback body effect was inherited by scheduler: %+v", record)
	}
	for _, function := range []string{"StopNow", "StopNewTimer", "StopDeferred"} {
		record := inventory[function]
		if !contains(record, "concurrency.timer.stop") || !record.Uncertain {
			t.Errorf("Timer.Stop evidence in %s = %+v", function, record)
		}
	}
	for _, function := range []string{"ResetNow", "ResetAfterFunc"} {
		record := inventory[function]
		if !contains(record, "concurrency.timer.reset") || !record.Uncertain {
			t.Errorf("Timer.Reset evidence in %s = %+v", function, record)
		}
	}
	if record := inventory["ResetAfterFunc"]; !contains(record, "concurrency.callback.register") {
		t.Errorf("ResetAfterFunc lost its separate callback-registration evidence: %+v", record)
	}
	if record := inventory["NewTickerAndStop"]; !contains(record, "concurrency.timer.schedule") || !contains(record, "concurrency.timer.stop") || record.Uncertain {
		t.Errorf("ticker creation/stop evidence = %+v", record)
	}
	if record := inventory["NewTickerDynamic"]; !contains(record, "concurrency.timer.schedule") || !record.Uncertain {
		t.Errorf("dynamic NewTicker argument lost possible scheduling or uncertainty: %+v", record)
	}
	if record := inventory["TickerResetKnown"]; !contains(record, "concurrency.timer.reset") || record.Uncertain {
		t.Errorf("known-positive Ticker.Reset evidence = %+v", record)
	}
	if record := inventory["TickerResetDynamic"]; !contains(record, "concurrency.timer.reset") || !record.Uncertain {
		t.Errorf("dynamic Ticker.Reset argument lost possible reset or uncertainty: %+v", record)
	}
	if record := inventory["TickerResetInvalid"]; len(record.Effects) != 0 || !record.Uncertain {
		t.Errorf("non-positive Ticker.Reset interval was reported as a reset: %+v", record)
	}
	if record := inventory["NewTickerInvalid"]; len(record.Effects) != 0 || !record.Uncertain {
		t.Errorf("non-positive NewTicker interval was reported as scheduling: %+v", record)
	}
	if record := inventory["TickInvalid"]; len(record.Effects) != 0 || record.Uncertain {
		t.Errorf("non-positive Tick interval should return nil without scheduling: %+v", record)
	}
	if record := inventory["SleepKnown"]; !contains(record, "concurrency.sleep") || record.Uncertain {
		t.Errorf("known-positive Sleep evidence = %+v", record)
	}
	if record := inventory["SleepDynamic"]; !contains(record, "concurrency.sleep") || !record.Uncertain {
		t.Errorf("dynamic Sleep lost possible delay or argument uncertainty: %+v", record)
	}
	if record := inventory["SleepCaller"]; !contains(record, "concurrency.sleep") || len(record.Effects) != 1 || record.Effects[0].Direct {
		t.Errorf("Sleep effect was not inherited from the helper: %+v", record)
	}
	for _, function := range []string{"SleepZero", "SleepNegative", "FakeSleep"} {
		if record := inventory[function]; len(record.Effects) != 0 || record.Uncertain {
			t.Errorf("non-sleeping or same-named control gained evidence in %s: %+v", function, record)
		}
	}
	if record := inventory["FakeTimerOperations"]; len(record.Effects) != 0 || record.Uncertain {
		t.Errorf("same-named application timer methods were summarized: %+v", record)
	}
	if record := inventory["FakeTickerStop"]; len(record.Effects) != 0 || record.Uncertain {
		t.Errorf("same-named application Ticker.Stop method was summarized: %+v", record)
	}
	if record := inventory["FakeTickerReset"]; len(record.Effects) != 0 || record.Uncertain {
		t.Errorf("same-named application Ticker.Reset method was summarized: %+v", record)
	}

	want := map[string]struct {
		function, api, effect, reason, kind string
	}{
		"Schedule\x00time.AfterFunc":                 {"Schedule", "time.AfterFunc", "concurrency.callback.register", "callback", "call"},
		"ScheduleInline\x00time.AfterFunc":           {"ScheduleInline", "time.AfterFunc", "concurrency.callback.register", "callback", "call"},
		"ResetAfterFunc\x00time.AfterFunc":           {"ResetAfterFunc", "time.AfterFunc", "concurrency.callback.register", "callback", "call"},
		"StopNow\x00time.(*Timer).Stop":              {"StopNow", "time.(*Timer).Stop", "concurrency.timer.stop", "state_condition", "call"},
		"StopNewTimer\x00time.NewTimer":              {"StopNewTimer", "time.NewTimer", "concurrency.timer.schedule", "", "call"},
		"StopNewTimer\x00time.(*Timer).Stop":         {"StopNewTimer", "time.(*Timer).Stop", "concurrency.timer.stop", "state_condition", "call"},
		"StopDeferred\x00time.(*Timer).Stop":         {"StopDeferred", "time.(*Timer).Stop", "concurrency.timer.stop", "state_condition", "defer"},
		"ResetNow\x00time.(*Timer).Reset":            {"ResetNow", "time.(*Timer).Reset", "concurrency.timer.reset", "state_condition", "call"},
		"ResetAfterFunc\x00time.(*Timer).Reset":      {"ResetAfterFunc", "time.(*Timer).Reset", "concurrency.timer.reset", "state_condition", "call"},
		"AfterChannel\x00time.After":                 {"AfterChannel", "time.After", "concurrency.timer.schedule", "", "call"},
		"NewTimerOnly\x00time.NewTimer":              {"NewTimerOnly", "time.NewTimer", "concurrency.timer.schedule", "", "call"},
		"NewTickerAndStop\x00time.NewTicker":         {"NewTickerAndStop", "time.NewTicker", "concurrency.timer.schedule", "", "call"},
		"NewTickerAndStop\x00time.(*Ticker).Stop":    {"NewTickerAndStop", "time.(*Ticker).Stop", "concurrency.timer.stop", "", "call"},
		"TickChannel\x00time.Tick":                   {"TickChannel", "time.Tick", "concurrency.timer.schedule", "", "call"},
		"SleepKnown\x00time.Sleep":                   {"SleepKnown", "time.Sleep", "concurrency.sleep", "", "call"},
		"SleepDynamic\x00time.Sleep":                 {"SleepDynamic", "time.Sleep", "concurrency.sleep", "argument_condition", "call"},
		"NewTickerDynamic\x00time.NewTicker":         {"NewTickerDynamic", "time.NewTicker", "concurrency.timer.schedule", "argument_condition", "call"},
		"TickerResetKnown\x00time.(*Ticker).Reset":   {"TickerResetKnown", "time.(*Ticker).Reset", "concurrency.timer.reset", "", "call"},
		"TickerResetDynamic\x00time.(*Ticker).Reset": {"TickerResetDynamic", "time.(*Ticker).Reset", "concurrency.timer.reset", "argument_condition", "call"},
	}
	counts := map[string]int{}
	for _, site := range report.Sites {
		for _, operation := range site.Operations {
			counts[operation.API]++
			if site.Function == "FakeTimerOperations" {
				t.Errorf("fake method generated operation evidence: %+v", site)
			}
			if expected, ok := want[site.Function+"\x00"+operation.API]; ok {
				wantUncertain := expected.reason != ""
				wantReasons := []string{}
				if expected.reason != "" {
					wantReasons = append(wantReasons, expected.reason)
				}
				if site.Kind != expected.kind || site.API != expected.api ||
					operation.Recognition != "summary" || operation.Uncertain != wantUncertain ||
					!reflect.DeepEqual(operation.Effects, []string{expected.effect}) ||
					!reflect.DeepEqual(operation.Reasons, wantReasons) ||
					!reflect.DeepEqual(site.Reasons, wantReasons) ||
					site.Location.Line < 1 || site.Location.Column < 1 {
					t.Errorf("timer operation evidence = %+v", site)
				}
			}
		}
	}
	if counts["time.AfterFunc"] != 3 || counts["time.(*Timer).Stop"] != 3 ||
		counts["time.(*Timer).Reset"] != 2 || counts["time.After"] != 1 ||
		counts["time.NewTimer"] != 2 || counts["time.NewTicker"] != 2 ||
		counts["time.(*Ticker).Stop"] != 1 || counts["time.(*Ticker).Reset"] != 2 ||
		counts["time.Tick"] != 1 || counts["time.Sleep"] != 2 || counts["builtin.close"] != 1 {
		t.Errorf("operation-site counts by API = %v", counts)
	}

	schedules, _ := effectResult(t, dir, "-effect", "concurrency.timer.schedule")
	if len(schedules.Sites) != 6 {
		t.Fatalf("timer-schedule effect filter selected %d sites, want 6", len(schedules.Sites))
	}
	resets, _ := effectResult(t, dir, "-effect", "concurrency.timer.reset")
	if len(resets.Sites) != 4 {
		t.Fatalf("timer-reset effect filter selected %d sites, want 4", len(resets.Sites))
	}
	sleeps, _ := effectResult(t, dir, "-effect", "concurrency.sleep")
	if len(sleeps.Sites) != 2 {
		t.Fatalf("sleep effect filter selected %d sites, want 2", len(sleeps.Sites))
	}
	stops, _ := effectResult(t, dir, "-effect", "concurrency.timer.stop")
	if len(stops.Sites) != 4 {
		t.Fatalf("timer-stop effect filter selected %d sites, want 4", len(stops.Sites))
	}
	for _, site := range stops.Sites {
		if site.API == "time.(*Timer).Stop" {
			if !site.Uncertain {
				t.Errorf("Timer.Stop lost state uncertainty: %+v", site)
			}
		} else if site.API != "time.(*Ticker).Stop" || site.Uncertain {
			t.Errorf("ticker-stop filter returned unexpected evidence: %+v", site)
		}
	}
	unresolved, _ := effectResult(t, dir, "-all", "-unresolved")
	if len(unresolved.Sites) != 2 {
		t.Fatalf("invalid ticker arguments produced %d unresolved sites, want 2: %+v", len(unresolved.Sites), unresolved.Sites)
	}
	unresolvedByFunction := map[string]analyzer.Site{}
	for _, site := range unresolved.Sites {
		unresolvedByFunction[site.Function] = site
	}
	for function, api := range map[string]string{"NewTickerInvalid": "time.NewTicker", "TickerResetInvalid": "time.(*Ticker).Reset"} {
		if site := unresolvedByFunction[function]; site.API != api || !reflect.DeepEqual(site.Reasons, []string{"argument_condition"}) {
			t.Errorf("invalid ticker precondition was lost or reported as a known operation: %+v", site)
		}
	}
	for _, site := range report.Sites {
		if site.Function == "TickInvalid" || site.Function == "FakeTickerStop" || site.Function == "SleepZero" || site.Function == "SleepNegative" || site.Function == "FakeSleep" {
			t.Errorf("non-scheduling or same-named call gained an operation site: %+v", site)
		}
	}
}
