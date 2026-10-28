package analyzer

import (
	"encoding/json"
	"fmt"
	"go/token"
	"sort"

	"golang.org/x/tools/go/ssa"
)

// ControlFlowOptions bounds the exported control-flow projection. MaxFunctions
// selects unique function variants in g.variant order. MaxBlocksPerFunction
// suppresses all CFG conclusions for an oversized function; MaxBlockVisits
// counts block-processing iterations across reachability, dominator, and postdominator scans. MaxBranches is
// global across exported branch rows. MaxRelationships is global and counts
// each dominator relation, control-dependency relation, bypass-return record,
// and local marker-evidence record (including recover). MaxDepth bounds each
// branch successor's postdominator-chain traversal. MaxJSONBytes counts compact
// UTF-8 json.Marshal output bytes.
type ControlFlowOptions struct {
	MaxFunctions         int `json:"max_functions"`
	MaxBlocksPerFunction int `json:"max_blocks_per_function"`
	MaxBlockVisits       int `json:"max_block_visits"`
	MaxBranches          int `json:"max_branches"`
	MaxRelationships     int `json:"max_relationships"`
	MaxDepth             int `json:"max_depth"`
	MaxJSONBytes         int `json:"max_json_bytes"`
}
type ControlFlow struct {
	SchemaVersion int                `json:"schema_version"`
	Schema        string             `json:"schema"`
	Limits        ControlFlowOptions `json:"limits"`
	Truncated     map[string]bool    `json:"truncated"`
	Limitations   []string           `json:"limitations"`
	Rows          []ControlFlowRow   `json:"rows"`
}
type ControlFlowRow struct {
	ID                    string                            `json:"id"`
	Site                  string                            `json:"site"`
	API                   string                            `json:"api"`
	Resolution            string                            `json:"resolution,omitempty"`
	Resolutions           []string                          `json:"resolutions,omitempty"`
	Function              Symbol                            `json:"function"`
	Block                 int                               `json:"block"`
	Instruction           int                               `json:"instruction"`
	Kind                  string                            `json:"kind"`
	Status                string                            `json:"status"`
	Reachable             bool                              `json:"reachable,omitempty"`
	Dominators            []int                             `json:"dominators,omitempty"`
	Branches              []ControlFlowBranch               `json:"branches,omitempty"`
	Returns               []int                             `json:"bypass_returns,omitempty"`
	ReturnEvidence        []ControlFlowReturn               `json:"bypass_return_evidence,omitempty"`
	MarkerEvidence        []ControlFlowMarker               `json:"marker_evidence,omitempty"`
	Variant               string                            `json:"function_variant,omitempty"`
	Markers               []string                          `json:"markers,omitempty"`
	InstructionCandidates []ControlFlowInstructionReference `json:"instruction_candidates,omitempty"`
	Complete              bool                              `json:"complete"`
}
type ControlFlowInstructionReference struct {
	Block       int      `json:"block"`
	Instruction int      `json:"instruction"`
	Kind        string   `json:"kind"`
	Resolutions []string `json:"resolutions,omitempty"`
}

type ControlFlowBranch struct {
	Block         int                  `json:"block"`
	True          int                  `json:"true"`
	False         int                  `json:"false"`
	Controls      []ControlFlowControl `json:"controls,omitempty"`
	ConditionKind string               `json:"condition_kind"`
	ConditionType string               `json:"condition_type,omitempty"`
	Condition     string               `json:"condition_identity,omitempty"`
	Location      *Location            `json:"location,omitempty"`
}

// ControlFlowControl identifies a control-dependent block and its depth along
// the postdominator chain from a branch successor.
type ControlFlowControl struct {
	Block int `json:"block"`
	Depth int `json:"depth"`
}
type ControlFlowReturn struct {
	Block       int       `json:"block"`
	Instruction int       `json:"instruction"`
	Location    *Location `json:"location,omitempty"`
	ResultTypes []string  `json:"result_types,omitempty"`
}
type ControlFlowMarker struct {
	Kind        string    `json:"kind"`
	Block       int       `json:"block"`
	Instruction int       `json:"instruction"`
	Location    *Location `json:"location,omitempty"`
}

func (g *graph) analyzeControlFlow(sites []Site, opts ControlFlowOptions, effectReportSitesTruncated bool) (ControlFlow, error) {
	defaults := []*int{&opts.MaxFunctions, &opts.MaxBlocksPerFunction, &opts.MaxBlockVisits, &opts.MaxBranches, &opts.MaxRelationships, &opts.MaxDepth, &opts.MaxJSONBytes}
	vals := []int{64, 128, 4096, 256, 2048, 8, 1048576}
	names := []string{"functions", "blocks_per_function", "block_visits", "branches", "relationships", "depth", "json_bytes"}
	for i, p := range defaults {
		if *p == 0 {
			*p = vals[i]
		}
		if *p < 1 {
			return ControlFlow{}, fmt.Errorf("control-flow %s limit must be positive", names[i])
		}
	}
	out := ControlFlow{SchemaVersion: 1, Schema: "reffector.control-flow.v1", Limits: opts, Truncated: map[string]bool{}, Limitations: []string{"SSA control-flow facts are structural evidence only; they do not establish execution, feasible paths, safety, authorization, or cleanup success."}, Rows: []ControlFlowRow{}}
	if effectReportSitesTruncated {
		out.Truncated["effect_report_sites"] = true
		out.Limitations = append(out.Limitations, "Rows cover only EffectReport sites retained by its site limit.")
	}
	type match struct {
		site        Site
		api         string
		fn          *ssa.Function
		b, i        int
		ins         ssa.Instruction
		resolutions []string
	}
	ms := []match{}
	seenfn := map[*ssa.Function]bool{}
	for _, s := range sites {
		apis := siteOperationAPIs(s)
		if len(apis) == 0 {
			out.Rows = append(out.Rows, unavailableControlFlowRow(g, s, "", "unavailable_api_identity", ""))
			continue
		}
		nodes := make([]*node, 0, len(g.siteNodes[s.ID]))
		for n := range g.siteNodes[s.ID] {
			nodes = append(nodes, n)
		}
		sort.Slice(nodes, func(i, j int) bool { return variantForNode(g, nodes[i]) < variantForNode(g, nodes[j]) })
		if len(nodes) == 0 {
			for _, api := range apis {
				out.Rows = append(out.Rows, unavailableControlFlowRow(g, s, api, "unavailable_site_nodes", ""))
			}
			continue
		}
		for _, n := range nodes {
			if n == nil {
				continue
			}
			fn := n.fn
			if fn == nil || len(fn.Blocks) == 0 {
				for _, api := range apis {
					out.Rows = append(out.Rows, unavailableControlFlowRow(g, s, api, "unavailable_function_body", variantForNode(g, n)))
				}
				continue
			}
			for _, api := range apis {
				for bi, b := range fn.Blocks {
					for ii, in := range b.Instrs {
						if !in.Pos().IsValid() || g.location(in.Pos()) != s.Location {
							continue
						}
						resolutions, ok := instructionAPIResolutions(g, in, api)
						if !ok {
							continue
						}
						ms = append(ms, match{site: s, api: api, fn: fn, b: bi, i: ii, ins: in, resolutions: resolutions})
						seenfn[fn] = true
					}
				}
			}
		}
	}
	sort.Slice(ms, func(i, j int) bool {
		a, b := ms[i], ms[j]
		if a.site.ID != b.site.ID {
			return a.site.ID < b.site.ID
		}
		if a.api != b.api {
			return a.api < b.api
		}
		if g.variant(a.fn) != g.variant(b.fn) {
			return g.variant(a.fn) < g.variant(b.fn)
		}
		if symbolKey(g.symbol(a.fn)) != symbolKey(g.symbol(b.fn)) {
			return symbolKey(g.symbol(a.fn)) < symbolKey(g.symbol(b.fn))
		}
		if a.b != b.b {
			return a.b < b.b
		}
		return a.i < b.i
	})
	allowed := map[*ssa.Function]bool{}
	fns := []*ssa.Function{}
	for f := range seenfn {
		fns = append(fns, f)
	}
	sort.Slice(fns, func(i, j int) bool { return g.variant(fns[i]) < g.variant(fns[j]) })
	allowedVariants := map[string]bool{}
	for _, f := range fns {
		v := g.variant(f)
		if _, exists := allowedVariants[v]; exists {
			allowed[f] = allowedVariants[v]
			continue
		}
		if len(allowedVariants) < opts.MaxFunctions {
			allowedVariants[v] = true
			allowed[f] = true
		} else {
			allowedVariants[v] = false
			out.Truncated["functions"] = true
		}
	}
	counts := map[string]int{}
	for _, m := range ms {
		counts[m.site.ID+"\x00"+m.api+"\x00"+g.variant(m.fn)]++
	}
	ambiguous := map[string]bool{}
	facts := map[*ssa.Function]controlFlowFacts{}
	factsByVariant := map[string]controlFlowFacts{}
	blockVisits, branchCount, relationshipCount := 0, 0, 0
	for _, fn := range fns {
		variant := g.variant(fn)
		if _, exists := factsByVariant[variant]; exists || !allowed[fn] || len(fn.Blocks) > opts.MaxBlocksPerFunction {
			continue
		}
		remaining := opts.MaxBlockVisits - blockVisits
		fact, used := computeControlFlowFacts(fn.Blocks, opts.MaxDepth, remaining)
		blockVisits += used
		factsByVariant[variant] = fact
		if !fact.complete {
			out.Truncated["block_visits"] = true
		}
	}
	for _, fn := range fns {
		if fact, exists := factsByVariant[g.variant(fn)]; exists {
			facts[fn] = fact
		}
	}
	for _, m := range ms {
		group := m.site.ID + "\x00" + m.api + "\x00" + g.variant(m.fn)
		resolution := ""
		if len(m.resolutions) == 1 {
			resolution = m.resolutions[0]
		}
		row := ControlFlowRow{Site: m.site.ID, API: m.api, Resolution: resolution, Resolutions: m.resolutions, Function: g.symbol(m.fn), Block: m.b, Instruction: m.i, Kind: fmt.Sprintf("%T", m.ins), Status: "partial", Complete: false, Variant: g.variant(m.fn)}
		row.ID = stableID("control", fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%d\x00%s", row.Site, row.API, symbolKey(row.Function), row.Variant, row.Block, row.Instruction, row.Kind))
		if counts[group] > 1 {
			if !ambiguous[group] {
				row.ID = stableID("control-ambiguous", group)
				row.Block = -1
				row.Instruction = -1
				row.Kind = "ambiguous"
				row.Resolution = ""
				row.Resolutions = []string{}
				row.Status = "ambiguous_instruction"
				candidates := map[string]bool{}
				resolutions := map[string]bool{}
				for _, candidate := range ms {
					if candidate.site.ID != m.site.ID || candidate.api != m.api || g.variant(candidate.fn) != g.variant(m.fn) {
						continue
					}
					kind := fmt.Sprintf("%T", candidate.ins)
					key := fmt.Sprintf("%d\\x00%d\\x00%s", candidate.b, candidate.i, kind)
					if candidates[key] {
						continue
					}
					candidates[key] = true
					row.InstructionCandidates = append(row.InstructionCandidates, ControlFlowInstructionReference{Block: candidate.b, Instruction: candidate.i, Kind: kind, Resolutions: candidate.resolutions})
					for _, resolution := range candidate.resolutions {
						resolutions[resolution] = true
					}
				}
				for resolution := range resolutions {
					row.Resolutions = append(row.Resolutions, resolution)
				}
				sort.Strings(row.Resolutions)
				out.Rows = append(out.Rows, row)
				ambiguous[group] = true
			}
			out.Truncated["mapping"] = true
			continue
		}
		if !allowed[m.fn] {
			row.Status = "truncated_functions"
			row.Complete = false
			out.Rows = append(out.Rows, row)
			continue
		}
		blocks := m.fn.Blocks
		if len(blocks) > opts.MaxBlocksPerFunction {
			row.Status = "truncated_blocks"
			out.Truncated["blocks_per_function"] = true
			out.Rows = append(out.Rows, row)
			continue
		}
		if m.b < 0 || m.b >= len(blocks) {
			row.Status = "unavailable"
			row.Complete = false
			out.Rows = append(out.Rows, row)
			continue
		}
		fact := facts[m.fn]
		if !fact.complete {
			row.Status = "truncated_block_visits"
			row.Complete = false
			out.Rows = append(out.Rows, row)
			continue
		}
		reachable := fact.reachable
		row.Reachable = reachable[m.b]
		if !fact.postdomKnown {
			row.Status = "unknown_postdominators"
		}
		if !row.Reachable {
			row.Status = "unreachable"
			row.Complete = false
			out.Rows = append(out.Rows, row)
			continue
		}
		dom := fact.dominators
		dominatorIndexes := make([]int, 0, len(dom[m.b]))
		for x := range dom[m.b] {
			dominatorIndexes = append(dominatorIndexes, x)
		}
		sort.Ints(dominatorIndexes)
		for _, x := range dominatorIndexes {
			if relationshipCount >= opts.MaxRelationships {
				truncateRelationships(&out, &row)
				break
			}
			row.Dominators = append(row.Dominators, x)
			relationshipCount++
		}
		if m.fn.Recover != nil {
			row.Markers = append(row.Markers, "function_recover")
			if relationshipCount < opts.MaxRelationships {
				row.MarkerEvidence = append(row.MarkerEvidence, ControlFlowMarker{Kind: "function_recover", Block: m.fn.Recover.Index, Instruction: -1, Location: controlFlowLocation(g, m.fn.Pos())})
				relationshipCount++
			} else {
				truncateRelationships(&out, &row)
			}
		}
		for bi, b := range blocks {
			if _, ok := reachable[bi]; !ok {
				continue
			}
			if iff, ok := b.Instrs[len(b.Instrs)-1].(*ssa.If); ok && len(b.Succs) == 2 {
				if branchCount >= opts.MaxBranches {
					out.Truncated["branches"] = true
					row.Complete = false
					row.Status = "truncated_branches"
					continue
				}
				branchCount++
				br := ControlFlowBranch{Block: bi, True: b.Succs[0].Index, False: b.Succs[1].Index, ConditionKind: "ssa.If", ConditionType: iff.Cond.Type().String(), Condition: fmt.Sprintf("%T:%s", iff.Cond, iff.Cond.Name()), Location: controlFlowConditionLocation(g, iff.Cond, iff.Pos())}
				_ = iff
				if fact.postdomKnown {
					br.Controls = fact.dependencies[bi]
					if fact.depthCut[bi] {
						out.Truncated["depth"] = true
						row.Complete = false
						row.Status = "truncated_depth"
					}
					kept := br.Controls[:0]
					for _, control := range br.Controls {
						if relationshipCount >= opts.MaxRelationships {
							truncateRelationships(&out, &row)
							break
						}
						kept = append(kept, control)
						relationshipCount++
					}
					br.Controls = kept
				} else {
					row.Status = "unknown_postdominators"
					row.Complete = false
				}
				row.Branches = append(row.Branches, br)
			}
		}
		for bi, b := range blocks {
			if !reachable[bi] {
				continue
			}
			for ii, in := range b.Instrs {
				if call, ok := in.(ssa.CallInstruction); ok {
					if builtin, ok := call.Common().Value.(*ssa.Builtin); ok && builtin.Name() == "recover" {
						row.Markers = append(row.Markers, "recover")
					}
				}
				switch in.(type) {
				case *ssa.Defer:
					row.Markers = append(row.Markers, "defer")
				case *ssa.RunDefers:
					row.Markers = append(row.Markers, "run_defers")
				case *ssa.Return:
					row.Markers = append(row.Markers, "return")
				case *ssa.Panic:
					row.Markers = append(row.Markers, "panic")
				}
				if ret, ok := in.(*ssa.Return); ok {
					bypass := !dom[bi][m.b] || (bi == m.b && ii < m.i)
					if bypass {
						if relationshipCount >= opts.MaxRelationships {
							truncateRelationships(&out, &row)
						} else {
							relationshipCount++
							row.Returns = append(row.Returns, bi)
							types := []string{}
							for _, v := range ret.Results {
								types = append(types, v.Type().String())
							}
							row.ReturnEvidence = append(row.ReturnEvidence, ControlFlowReturn{Block: bi, Instruction: ii, Location: controlFlowLocation(g, ret.Pos()), ResultTypes: types})
						}
					}
				}
				marker := ""
				switch in.(type) {
				case *ssa.Defer:
					marker = "defer"
				case *ssa.RunDefers:
					marker = "run_defers"
				case *ssa.Panic:
					marker = "panic"
				}
				if _, ok := in.(*ssa.Return); ok {
					marker = "return"
				}
				if call, ok := in.(ssa.CallInstruction); ok {
					if builtin, ok := call.Common().Value.(*ssa.Builtin); ok && builtin.Name() == "recover" {
						marker = "recover"
					}
				}
				if marker != "" {
					if relationshipCount < opts.MaxRelationships {
						row.MarkerEvidence = append(row.MarkerEvidence, ControlFlowMarker{Kind: marker, Block: bi, Instruction: ii, Location: controlFlowLocation(g, in.Pos())})
						relationshipCount++
					} else {
						truncateRelationships(&out, &row)
					}
				}
			}
		}
		sort.Strings(row.Markers)
		row.Markers = uniqueStrings(row.Markers)
		row.Returns = uniqueInts(row.Returns)
		if row.Status == "partial" {
			row.Status = "complete"
			row.Complete = true
		}
		out.Rows = append(out.Rows, row)
	}
	// Sites without any exact SSA instruction remain explicit; never guess from location/API.
	mapped := map[string]bool{}
	for _, r := range out.Rows {
		mapped[r.Site+"\x00"+r.API] = true
	}
	for _, s := range sites {
		for _, api := range siteOperationAPIs(s) {
			if !mapped[s.ID+"\x00"+api] {
				out.Truncated["mapping"] = true
				out.Rows = append(out.Rows, ControlFlowRow{ID: stableID("control-unavailable", s.ID+"\x00"+api), Site: s.ID, API: api, Status: "unavailable_mapping", Complete: false})
			}
		}
	}
	sort.Slice(out.Rows, func(i, j int) bool { return out.Rows[i].ID < out.Rows[j].ID })
	if len(out.Truncated) > 0 {
		for i := range out.Rows {
			out.Rows[i].Complete = false
		}
	}
	if b, e := json.Marshal(out); e != nil {
		return ControlFlow{}, e
	} else if len(b) > opts.MaxJSONBytes {
		out.Truncated["json_bytes"] = true
		out.Rows = []ControlFlowRow{}
		matched := map[string]bool{}
		omittedIDs := map[string]bool{}
		for _, m := range ms {
			kind := fmt.Sprintf("%T", m.ins)
			id := stableID("control", fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%d\x00%d\x00%s", m.site.ID, m.api, symbolKey(g.symbol(m.fn)), g.variant(m.fn), m.b, m.i, kind))
			if !omittedIDs[id] {
				row := ControlFlowRow{ID: id, Site: m.site.ID, API: m.api, Function: g.symbol(m.fn), Block: m.b, Instruction: m.i, Kind: kind, Variant: g.variant(m.fn), Status: "omitted_json_bytes", Complete: false}
				out.Rows = append(out.Rows, row)
				omittedIDs[id] = true
			}
			matched[m.site.ID+"\x00"+m.api] = true
		}
		for _, s := range sites {
			for _, api := range siteOperationAPIs(s) {
				if !matched[s.ID+"\x00"+api] {
					out.Rows = append(out.Rows, ControlFlowRow{ID: stableID("control-omitted", s.ID+"\x00"+api), Site: s.ID, API: api, Status: "omitted_json_bytes", Complete: false})
				}
			}
		}
		sort.Slice(out.Rows, func(i, j int) bool { return out.Rows[i].ID < out.Rows[j].ID })
		b, e = json.Marshal(out)
		if e != nil {
			return ControlFlow{}, e
		}
		if len(b) > opts.MaxJSONBytes {
			return ControlFlow{}, fmt.Errorf("control-flow JSON byte limit below minimum valid schema")
		}
	}
	return out, nil
}

func truncateRelationships(out *ControlFlow, row *ControlFlowRow) {
	out.Truncated["relationships"] = true
	row.Complete = false
	row.Status = "truncated_relationships"
}

type controlFlowFacts struct {
	reachable    map[int]bool
	dominators   map[int]map[int]bool
	dependencies map[int][]ControlFlowControl
	depthCut     map[int]bool
	postdomKnown bool
	complete     bool
}

// computeControlFlowFacts counts one visit per processed block in each fixed-point
// scan, and one visit per block removed from the reachability queue.
func computeControlFlowFacts(blocks []*ssa.BasicBlock, maxDepth, budget int) (controlFlowFacts, int) {
	used := 0
	visit := func() bool {
		if used >= budget {
			return false
		}
		used++
		return true
	}
	fact := controlFlowFacts{reachable: map[int]bool{}, dominators: map[int]map[int]bool{}, complete: false}
	if len(blocks) > 0 {
		q := []int{0}
		for len(q) > 0 {
			x := q[0]
			q = q[1:]
			if x < 0 || x >= len(blocks) || fact.reachable[x] {
				continue
			}
			if !visit() {
				return controlFlowFacts{}, used
			}
			fact.reachable[x] = true
			for _, succ := range blocks[x].Succs {
				q = append(q, succ.Index)
			}
		}
	}
	for i := range blocks {
		fact.dominators[i] = map[int]bool{}
		if i == 0 {
			fact.dominators[i][0] = true
		} else if fact.reachable[i] {
			for j := range fact.reachable {
				fact.dominators[i][j] = true
			}
		}
	}
	changed := true
	for changed {
		changed = false
		for i, b := range blocks {
			if i == 0 || !fact.reachable[i] {
				continue
			}
			if !visit() {
				return controlFlowFacts{}, used
			}
			next := map[int]bool{}
			first := true
			for _, p := range b.Preds {
				if !fact.reachable[p.Index] {
					continue
				}
				if first {
					for x := range fact.dominators[p.Index] {
						next[x] = true
					}
					first = false
				} else {
					for x := range next {
						if !fact.dominators[p.Index][x] {
							delete(next, x)
						}
					}
				}
			}
			next[i] = true
			if !sameInts(fact.dominators[i], next) {
				fact.dominators[i] = next
				changed = true
			}
		}
	}
	deps, cut, known, ok := controlDependencies(blocks, fact.reachable, maxDepth, visit)
	if !ok {
		return controlFlowFacts{}, used
	}
	fact.dependencies, fact.depthCut, fact.postdomKnown, fact.complete = deps, cut, known, true
	return fact, used
}

// controlDependencies computes postdominators after adding one synthetic exit. A
// reachable region that cannot reach a terminal block has unknown postdominators.
func controlDependencies(blocks []*ssa.BasicBlock, reachable map[int]bool, maxDepth int, visit func() bool) (map[int][]ControlFlowControl, map[int]bool, bool, bool) {
	n := len(blocks)
	out := make(map[int][]ControlFlowControl)
	truncated := make(map[int]bool)
	canExit := make(map[int]bool)
	q := []int{}
	for i, b := range blocks {
		if !reachable[i] {
			continue
		}
		if !visit() {
			return nil, nil, false, false
		}
		if len(b.Succs) == 0 {
			canExit[i] = true
			q = append(q, i)
		}
	}
	for len(q) > 0 {
		x := q[0]
		q = q[1:]
		if !visit() {
			return nil, nil, false, false
		}
		for _, p := range blocks[x].Preds {
			if reachable[p.Index] && !canExit[p.Index] {
				canExit[p.Index] = true
				q = append(q, p.Index)
			}
		}
	}
	for i := range reachable {
		if !canExit[i] {
			return nil, nil, false, true
		}
	}
	// Synthetic exit is n. Initialize the classic postdominator fixed point.
	pd := make([]map[int]bool, n+1)
	pd[n] = map[int]bool{n: true}
	for i := 0; i < n; i++ {
		pd[i] = map[int]bool{}
		if !reachable[i] {
			continue
		}
		if !visit() {
			return nil, nil, false, false
		}
		for j := 0; j <= n; j++ {
			if j == n || reachable[j] {
				pd[i][j] = true
			}
		}
	}
	changed := true
	for changed {
		changed = false
		for i, b := range blocks {
			if !reachable[i] {
				continue
			}
			if !visit() {
				return nil, nil, false, false
			}
			succ := []int{}
			for _, s := range b.Succs {
				if reachable[s.Index] {
					succ = append(succ, s.Index)
				}
			}
			if len(succ) == 0 {
				succ = append(succ, n)
			}
			next := map[int]bool{}
			for v := range pd[succ[0]] {
				next[v] = true
			}
			for _, s := range succ[1:] {
				for v := range next {
					if !pd[s][v] {
						delete(next, v)
					}
				}
			}
			next[i] = true
			if !sameInts(pd[i], next) {
				pd[i] = next
				changed = true
			}
		}
	}
	for bi, b := range blocks {
		if !reachable[bi] || len(b.Succs) < 2 {
			continue
		}
		if !visit() {
			return nil, nil, false, false
		}
		strict := map[int]bool{}
		for x := range pd[bi] {
			if x != bi {
				strict[x] = true
			}
		}
		ipdom := -1
		for c := range strict {
			closest := true
			for d := range strict {
				if d != c && pd[d][c] {
					closest = false
					break
				}
			}
			if closest {
				ipdom = c
				break
			}
		}
		controlled := map[int]int{}
		for _, succ := range b.Succs {
			runner := succ.Index
			seen := map[int]bool{}
			depth := 0
			for runner != ipdom && runner >= 0 && runner < n && !seen[runner] && depth < maxDepth {
				if !visit() {
					return nil, nil, false, false
				}
				seen[runner] = true
				depth++
				if prior, exists := controlled[runner]; !exists || depth < prior {
					controlled[runner] = depth
				}
				parent := -1
				for c := range pd[runner] {
					if c != runner && c != n { // immediate postdominator of runner
						ok := true
						for d := range pd[runner] {
							if d != runner && d != c && pd[d][c] {
								ok = false
								break
							}
						}
						if ok {
							parent = c
							break
						}
					}
				}
				runner = parent
			}
			if runner != ipdom && runner >= 0 && runner < n && !seen[runner] {
				truncated[bi] = true
			}
		}
		indexes := make([]int, 0, len(controlled))
		for x := range controlled {
			indexes = append(indexes, x)
		}
		sort.Ints(indexes)
		for _, x := range indexes {
			out[bi] = append(out[bi], ControlFlowControl{Block: x, Depth: controlled[x]})
		}
	}
	return out, truncated, true, true
}

func siteOperationAPIs(s Site) []string {
	apis := make([]string, 0, len(s.Operations)+len(s.APIs))
	for _, op := range s.Operations {
		if op.API != "" {
			apis = append(apis, op.API)
		}
	}
	if len(apis) == 0 {
		for _, api := range s.APIs {
			if api != "" {
				apis = append(apis, api)
			}
		}
	}
	sort.Strings(apis)
	return uniqueStrings(apis)
}

func instructionAPIResolutions(g *graph, in ssa.Instruction, api string) ([]string, bool) {
	if operation, _, _, ok := ssaOperation(in); ok {
		return []string{"operation"}, operation == api
	}
	call, ok := in.(ssa.CallInstruction)
	if !ok {
		return nil, false
	}
	if fn := call.Common().StaticCallee(); fn != nil {
		return []string{"established"}, apiKey(fn) == api
	}
	classes := map[string]bool{}
	for _, fn := range g.callCandidates(call) {
		if apiKey(fn) != api {
			continue
		}
		if containsExactFunction(g.functionCandidates[call], fn) {
			classes["candidate_function_value"] = true
		} else {
			classes["candidate_vta"] = true
		}
	}
	resolutions := make([]string, 0, len(classes))
	for resolution := range classes {
		resolutions = append(resolutions, resolution)
	}
	sort.Strings(resolutions)
	return resolutions, len(resolutions) > 0
}

func instructionAPIResolution(g *graph, in ssa.Instruction, api string) (string, bool) {
	resolutions, ok := instructionAPIResolutions(g, in, api)
	if !ok || len(resolutions) != 1 {
		return "", false
	}
	return resolutions[0], true
}

func controlFlowConditionLocation(g *graph, condition ssa.Value, branchPos token.Pos) *Location {
	if location := controlFlowLocation(g, branchPos); location != nil {
		return location
	}
	if instruction, ok := condition.(ssa.Instruction); ok {
		return controlFlowLocation(g, instruction.Pos())
	}
	return nil
}

func controlFlowLocation(g *graph, pos token.Pos) *Location {
	if !pos.IsValid() {
		return nil
	}
	location := g.location(pos)
	if location.File == "" || location.Line < 1 {
		return nil
	}
	return &location
}

func variantForNode(g *graph, n *node) string {
	if n == nil || n.fn == nil {
		return ""
	}
	return g.variant(n.fn)
}

func unavailableControlFlowRow(g *graph, s Site, api, status, variant string) ControlFlowRow {
	key := s.ID + "\x00" + api + "\x00" + status + "\x00" + variant
	return ControlFlowRow{ID: stableID("control-"+status, key), Site: s.ID, API: api, Function: s.Symbol, Variant: variant, Status: status, Complete: false}
}
func sameInts(a, b map[int]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for x := range a {
		if !b[x] {
			return false
		}
	}
	return true
}
func uniqueInts(xs []int) []int {
	m := map[int]bool{}
	o := []int{}
	for _, x := range xs {
		if !m[x] {
			m[x] = true
			o = append(o, x)
		}
	}
	sort.Ints(o)
	return o
}
func uniqueStrings(xs []string) []string {
	o := []string{}
	for _, x := range xs {
		if len(o) == 0 || o[len(o)-1] != x {
			o = append(o, x)
		}
	}
	return o
}
