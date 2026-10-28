package main

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Version2beta/reffector/internal/analyzer"
)

func TestContextualWitnessRootFilters(t *testing.T) {
	dir := copyWaitGroupVTAFixture(t)
	for _, tc := range []struct {
		name, function, pkg, effect, operation, paths, pattern string
		roots                                                  []analyzer.Symbol
		sites                                                  int
		unresolved                                             bool
	}{
		{name: "exact-root-one-path", function: "=Root", effect: "concurrency.wait", paths: "1", pattern: ".", roots: []analyzer.Symbol{{Package: "example.com/waitgroupfixture", Function: "Root"}}, sites: 1},
		{name: "multiple-roots", function: "Root", effect: "concurrency.wait", paths: "100", pattern: "./...", roots: []analyzer.Symbol{{Package: "example.com/waitgroupfixture", Function: "Root"}, {Package: "example.com/waitgroupfixture/other", Function: "WaiterRoot"}}, sites: 1},
		{name: "package-filter-one-path", function: "Root", pkg: "example.com/waitgroupfixture/other", effect: "concurrency.wait", paths: "1", pattern: "./...", roots: []analyzer.Symbol{{Package: "example.com/waitgroupfixture/other", Function: "WaiterRoot"}}, sites: 1},
		{name: "task-effect-filter", function: "=Root", effect: "concurrency.task", paths: "100", pattern: ".", roots: []analyzer.Symbol{{Package: "example.com/waitgroupfixture", Function: "Root"}}, sites: 2},
		{name: "unresolved-path", function: "=Root", operation: "(Handler).Handle", paths: "1", pattern: ".", roots: []analyzer.Symbol{{Package: "example.com/waitgroupfixture", Function: "Root"}}, sites: 1, unresolved: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			common := []string{"-all", "-exact", "-function", tc.function, "-package", tc.pkg, "-effect", tc.effect, "-operation", tc.operation, "-dependency-vta-functions", "100", "-dependency-vta-edges", "100", "-max-depth", "12"}
			args := append(append([]string{"-details", "-max-paths", tc.paths}, common...), tc.pattern)
			text, diagnostic, status := invoke(t, dir, nil, args...)
			if status != 0 {
				t.Fatalf("details exit=%d stderr=%s", status, diagnostic)
			}
			again, _, status := invoke(t, dir, nil, args...)
			if status != 0 || text != again {
				t.Fatal("details JSON is not deterministic")
			}
			var details analyzer.Report
			if err := json.Unmarshal([]byte(text), &details); err != nil {
				t.Fatal(err)
			}
			if details.Details == nil || len(details.Details.Sites) != tc.sites {
				t.Fatalf("details site selection: %+v", details.Details)
			}
			if stats := details.Stats.DependencyVTA; stats == nil || stats.ContextTruncated || stats.Truncated {
				t.Fatalf("unexpected context truncation: %+v", stats)
			}
			allowed := map[analyzer.Symbol]bool{}
			for _, root := range tc.roots {
				allowed[root] = true
			}
			edges := map[string]analyzer.CallEdge{}
			for _, edge := range details.Details.CallEdges {
				edges[edge.ID] = edge
			}
			for _, site := range details.Details.Sites {
				seen := map[analyzer.Symbol]bool{}
				for _, witness := range site.Witnesses {
					if !allowed[witness.Root] {
						t.Errorf("unrequested root consumed witness budget: %+v", witness)
					}
					if witness.Direct || len(witness.Edges) == 0 {
						t.Errorf("caller explanation became direct: %+v", witness)
					}
					seen[witness.Root] = true
					current := witness.Root
					candidate := false
					for _, id := range witness.Edges {
						edge, ok := edges[id]
						if !ok || edge.Caller != current {
							t.Fatalf("disconnected witness edge %s: %+v", id, edge)
						}
						current = edge.Callee
						candidate = candidate || edge.Resolution == "candidate_vta"
					}
					if current != site.Symbol || candidate == tc.unresolved {
						t.Errorf("wrong witness endpoint or candidate status: %+v site=%+v", witness, site)
					}
				}
				if !reflect.DeepEqual(seen, allowed) {
					t.Errorf("selected roots lost their explanation: got %v, want %v", seen, allowed)
				}
			}
			effectArgs := append([]string{}, common...)
			if tc.unresolved {
				effectArgs = append(effectArgs, "-unresolved")
			}
			effectArgs = append(effectArgs, tc.pattern)
			effect, output := effectResult(t, dir, effectArgs...)
			_, repeat := effectResult(t, dir, effectArgs...)
			if output != repeat {
				t.Fatal("effect JSON is not deterministic")
			}
			if len(effect.Sites) != tc.sites {
				t.Fatalf("effect site selection: %+v", effect.Sites)
			}
			for _, site := range details.Details.Sites {
				found := false
				for _, other := range effect.Sites {
					found = found || site.ID == other.ID
				}
				if !found {
					t.Errorf("projections disagree on source site identity %s", site.ID)
				}
			}
			seenRoots := map[analyzer.Symbol]bool{}
			for _, node := range effect.Nodes {
				if allowed[node.Symbol] {
					seenRoots[node.Symbol] = true
				}
				if node.Function == "ClosedBoundary" || node.Function == "EmptyRoot" {
					t.Errorf("incompatible root leaked into filtered graph: %+v", node)
				}
			}
			if !reflect.DeepEqual(seenRoots, allowed) {
				t.Errorf("effect graph lost selected roots: got %v want %v", seenRoots, allowed)
			}
			candidate := false
			for _, edge := range effect.Edges {
				if edge.Source != nil && edge.Source.Resolution == "candidate_vta" {
					candidate = true
					if !edge.Candidate {
						t.Errorf("candidate promoted to established: %+v", edge)
					}
				}
			}
			if candidate == tc.unresolved {
				t.Errorf("effect graph candidate status=%t for unresolved=%t", candidate, tc.unresolved)
			}
		})
	}
}
