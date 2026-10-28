package analyzer

import (
	"strings"
	"testing"
)

func TestArgumentProvenanceRolesAreExplicit(t *testing.T) {
	for _, tc := range []struct {
		api  string
		want []string
	}{
		{"os.WriteFile", []string{"path", "data"}},
		{"os.Rename", []string{"old_path", "new_path"}},
		{"os.StartProcess", []string{"path", "argv"}},
		{"os.Remove", []string{"path"}},
		{"io/ioutil.WriteFile", []string{"path", "data"}},
		{"io/ioutil.TempFile", []string{"dir", "pattern"}},
		{"io/ioutil.TempDir", []string{"dir", "pattern"}},
		{"os.CreateTemp", []string{"dir", "pattern"}},
		{"os.MkdirTemp", []string{"dir", "pattern"}},
		{"os.(*Root).Rename", []string{"old_path", "new_path"}},
		{"os.Link", []string{"old_path", "new_path"}},
		{"os.Symlink", []string{"old_path", "new_path"}},
		{"os.(*Root).Link", []string{"old_path", "new_path"}},
		{"os.(*Root).Symlink", []string{"old_path", "new_path"}},
		{"os/exec.(*Cmd).Start", []string{"receiver"}},
		{"os/exec.(*Cmd).Output", []string{"receiver"}},
		{"os/exec.(*Cmd).CombinedOutput", []string{"receiver"}},
	} {
		got := argumentRoles(tc.api)
		if len(got) != len(tc.want) {
			t.Fatalf("%s roles=%v, want %v", tc.api, got, tc.want)
		}
		for i, role := range tc.want {
			if got[i].Role != role {
				t.Fatalf("%s role %d=%q want %q", tc.api, i, got[i].Role, role)
			}
		}
	}
}
func TestArgumentProvenanceSemanticOriginsAndOperationSites(t *testing.T) {
	dir := argumentFixture(t)
	report, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, ArgumentProvenance: true, EffectView: true})
	if err != nil {
		t.Fatal(err)
	}
	p := report.ArgumentProvenance
	calls, nodes := map[string]ArgumentCall{}, map[string]ArgumentValue{}
	for _, c := range p.Calls {
		calls[c.ID] = c
	}
	for _, n := range p.Nodes {
		nodes[n.ID] = n
	}
	// Distinct static callers must remain distinct through a shared formal and
	// reach an operation role after a local return transformation.
	origins := map[string]string{}
	for _, n := range p.Nodes {
		if n.Kind == "constant" && (n.Label == `"first"` || n.Label == `"second"`) {
			origins[n.Label] = n.ID
		}
	}
	formal := ""
	callerRefs := map[string]string{}
	for _, n := range p.Nodes {
		if n.Kind != "parameter" || n.Label != "p" {
			continue
		}
		refs := map[string]string{}
		for _, e := range p.Edges {
			if e.To == n.ID && e.Resolution == "established" && e.CallRef != "" {
				if label := nodes[e.From].Label; label == `"first"` || label == `"second"` {
					refs[label] = e.CallRef
				}
			}
		}
		if len(refs) == 2 {
			formal, callerRefs = n.ID, refs
			break
		}
	}
	if formal == "" || len(origins) != 2 || callerRefs[`"first"`] == callerRefs[`"second"`] {
		t.Fatalf("caller actuals were merged or lost: origins=%v formal=%s call_refs=%v", origins, formal, callerRefs)
	}
	adj := map[string][]string{}
	for _, e := range p.Edges {
		adj[e.From] = append(adj[e.From], e.To)
	}
	reaches := func(from, target string) bool {
		queue, seen := []string{from}, map[string]bool{}
		for len(queue) > 0 {
			at := queue[0]
			queue = queue[1:]
			if at == target {
				return true
			}
			if seen[at] {
				continue
			}
			seen[at] = true
			queue = append(queue, adj[at]...)
		}
		return false
	}
	transformedRoleReached := false
	for _, s := range p.Sites {
		if s.API == "os.WriteFile" && len(s.Roles) > 0 &&
			reaches(origins[`"first"`], s.Roles[0].ValueRef) && reaches(origins[`"second"`], s.Roles[0].ValueRef) {
			transformedRoleReached = true
			break
		}
	}
	if !transformedRoleReached {
		t.Fatal("both caller origins do not reach a transformed operation role")
	}
	for _, s := range p.Sites {
		if s.API == "os.WriteFile" && s.CallRef != "" && calls[s.CallRef].ID == "" {
			t.Fatalf("unresolvable site call ref: %+v", s)
		}
	}
	// Tuple Extract preserves its selected result only.
	tupleRole := ""
	for _, s := range p.Sites {
		if s.API == "os.WriteFile" && nodes[s.Roles[0].ValueRef].Kind == "extract" {
			tupleRole = s.Roles[0].ValueRef
		}
	}
	if tupleRole == "" {
		t.Fatal("tuple result Extract missing")
	}
	for _, e := range p.Edges {
		if e.To == tupleRole && nodes[e.From].Label == `"excluded"` {
			t.Fatal("unselected tuple result leaked into role")
		}
	}
	// SSA Phi/cycles and explicit outside-analysis boundaries are visible.
	kinds, boundaries := map[string]bool{}, map[string]bool{}
	for _, n := range p.Nodes {
		kinds[n.Kind] = true
		boundaries[n.Boundary] = true
	}
	if !kinds["phi"] {
		t.Fatal("SSA Phi was not retained")
	}
	cycle := false
	for _, n := range p.Nodes {
		if n.Kind == "cycle" || n.Boundary == "recursive_ssa_value" {
			cycle = true
		}
	}
	if !cycle {
		t.Fatal("recursive SSA/Phi cycle was not retained")
	}
	for _, boundary := range []string{"global_state", "mutable_heap_state", "external_or_dynamic_call_return"} {
		if !boundaries[boundary] {
			t.Errorf("missing explicit boundary %q (%v)", boundary, boundaries)
		}
	}
	cmd := false
	for _, s := range p.Sites {
		if strings.HasPrefix(s.API, "os/exec.(*Cmd).") {
			cmd = true
			if len(s.Roles) != 1 || s.Roles[0].Role != "receiver" || s.Roles[0].Boundary != "mutable_cmd_receiver_fields_not_inferred" {
				t.Fatalf("Cmd arguments inferred beyond receiver boundary: %+v", s)
			}
		}
	}
	if !cmd {
		t.Fatal("Cmd receiver boundary missing")
	}
	wantRoles := map[string][]ArgumentRole{
		"os.WriteFile":         {{Role: "path", Index: 0}, {Role: "data", Index: 1}},
		"io/ioutil.WriteFile":  {{Role: "path", Index: 0}, {Role: "data", Index: 1}},
		"os.Remove":            {{Role: "path", Index: 0}},
		"os.Rename":            {{Role: "old_path", Index: 0}, {Role: "new_path", Index: 1}},
		"os.StartProcess":      {{Role: "path", Index: 0}, {Role: "argv", Index: 1}},
		"os.OpenInRoot":        {{Role: "dir", Index: 0}, {Role: "path", Index: 1}},
		"os.(*Root).Rename":    {{Role: "old_path", Index: 1}, {Role: "new_path", Index: 2}},
		"os.(*Root).WriteFile": {{Role: "path", Index: 1}, {Role: "data", Index: 2}},
	}
	foundRoles := map[string]bool{}
	for _, s := range p.Sites {
		want, ok := wantRoles[s.API]
		if !ok {
			continue
		}
		if len(s.Roles) != len(want) {
			t.Errorf("actual %s roles=%+v want=%+v", s.API, s.Roles, want)
			continue
		}
		for i := range want {
			if s.Roles[i].Role != want[i].Role || s.Roles[i].Index != want[i].Index || s.Roles[i].ValueRef == "" {
				t.Errorf("actual %s role %d=%+v want %+v", s.API, i, s.Roles[i], want[i])
			}
		}
		foundRoles[s.API] = true
	}
	for api := range wantRoles {
		if !foundRoles[api] {
			t.Errorf("fixture did not emit actual roles for %s", api)
		}
	}
}

func TestArgumentProvenanceCandidateAndEffectSiteJoin(t *testing.T) {
	dir := argumentFixture(t)
	report, err := AnalyzeReport([]string{"."}, Options{Dir: dir, Exact: true, ArgumentProvenance: true, EffectView: true})
	if err != nil {
		t.Fatal(err)
	}
	p := report.ArgumentProvenance
	calls := map[string]ArgumentCall{}
	for _, c := range p.Calls {
		calls[c.ID] = c
	}
	effectSites := map[string]Site{}
	for _, s := range report.Effects.Sites {
		effectSites[s.ID] = s
	}
	localWrapperCandidate := false
	for _, c := range p.Calls {
		if strings.Contains(c.Caller, "candidateLocalWrapper") && c.Resolution == "candidate_function_value" {
			localWrapperCandidate = true
		}
	}
	if !localWrapperCandidate {
		t.Fatal("local effect-wrapper function-value call was not retained as a candidate")
	}
	found := false
	for _, s := range p.Sites {
		if s.API != "os.Remove" || s.Location.Line == 0 {
			continue
		}
		c, ok := calls[s.CallRef]
		if s.Resolution == "candidate_function_value" {
			found = true
			if !ok || c.Resolution != "candidate_function_value" {
				t.Fatalf("candidate operation call ref does not resolve to candidate call: site=%+v call=%+v", s, c)
			}
			effectSite, ok := effectSites[s.SiteRef]
			if s.SiteRef == "" || !ok {
				t.Fatalf("candidate site ref does not join EffectReport site: %+v effect sites=%v", s, effectSites)
			}
			apiJoined := effectSite.API == s.API
			for _, api := range effectSite.APIs {
				apiJoined = apiJoined || api == s.API
			}
			for _, operation := range effectSite.Operations {
				apiJoined = apiJoined || operation.API == s.API
			}
			if !apiJoined {
				t.Fatalf("candidate API %q does not match joined EffectReport site: provenance=%+v effect=%+v", s.API, s, effectSite)
			}
		}
	}
	if !found {
		t.Fatalf("supported candidate function-value operation absent; sites=%+v effect=%+v", p.Sites, report.Effects.Sites)
	}
}

func TestArgumentProvenanceUnsupportedAPIReturnsNoRoles(t *testing.T) {
	if got := argumentRoles("example.com/app.WriteFile"); len(got) != 0 {
		t.Fatalf("application lookalike recognized: %v", got)
	}
}
