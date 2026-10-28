package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestControlFlowCLIOptInAndStandaloneOutput(t *testing.T) {
	dir := provenanceFixture(t)
	defaultOutput, stderr, status := invoke(t, dir, nil, "-view", "effects", "-json")
	if status != 0 || strings.Contains(defaultOutput, `"control_flow"`) {
		t.Fatalf("default effect report changed: status=%d stderr=%s output=%s", status, stderr, defaultOutput)
	}
	stdout, stderr, status := invoke(t, dir, nil, "-control-flow")
	if status != 0 {
		t.Fatalf("status=%d stderr=%s", status, stderr)
	}
	var projection analyzer.ControlFlow
	if err := json.Unmarshal([]byte(stdout), &projection); err != nil {
		t.Fatalf("invalid control-flow output: %v: %s", err, stdout)
	}
	if projection.Schema != "reffector.control-flow.v1" {
		t.Fatalf("wrong schema: %q", projection.Schema)
	}
	boundedOutput, stderr, status := invoke(t, dir, nil, "-control-flow", "-max-sites", "1")
	if status != 0 {
		t.Fatalf("site-limited status=%d stderr=%s", status, stderr)
	}
	var bounded analyzer.ControlFlow
	if err := json.Unmarshal([]byte(boundedOutput), &bounded); err != nil {
		t.Fatalf("invalid site-limited output: %v", err)
	}
	if !bounded.Truncated["effect_report_sites"] || len(bounded.Limitations) < 2 {
		t.Fatalf("inherited EffectReport site truncation was hidden: %#v", bounded)
	}
	_, stderr, status = invoke(t, dir, nil, "-control-flow", "-argument-provenance")
	if status != 2 || !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("conflicting projections status=%d stderr=%s", status, stderr)
	}
	_, stderr, status = invoke(t, dir, nil, "-control-flow", "-control-flow-max-functions", "0")
	if status != 2 || !strings.Contains(stderr, "positive limits") {
		t.Fatalf("invalid bound status=%d stderr=%s", status, stderr)
	}
}
