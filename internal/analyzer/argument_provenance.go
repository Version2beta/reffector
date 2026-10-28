package analyzer

// Argument provenance is a bounded structural projection, not taint, trust,
// authorization, alias, or path-predicate analysis.
import (
	"encoding/json"
	"fmt"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"github.com/Version2beta/reffector/internal/effects"
	"golang.org/x/tools/go/ssa"
)

// MaxJSONBytes caps the compact JSON payload bytes; CLI framing newlines are not included.
type ArgumentProvenanceOptions struct {
	MaxDepth        int
	MaxNodes        int
	MaxCalls        int
	MaxAlternatives int
	MaxJSONBytes    int
}
type ArgumentProvenance struct {
	SchemaVersion int                  `json:"schema_version"`
	Schema        string               `json:"schema"`
	Nodes         []ArgumentValue      `json:"nodes"`
	Edges         []ArgumentEdge       `json:"edges"`
	Sites         []ArgumentSite       `json:"sites"`
	Calls         []ArgumentCall       `json:"calls"`
	Truncation    []ArgumentTruncation `json:"truncation"`
}
type ArgumentLocation struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}
type ArgumentValue struct {
	ID       string           `json:"id"`
	Kind     string           `json:"kind"`
	Label    string           `json:"label"`
	Location ArgumentLocation `json:"location"`
	Boundary string           `json:"boundary,omitempty"`
}
type ArgumentEdge struct {
	ID         string `json:"id"`
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Resolution string `json:"resolution"`
	CallRef    string `json:"call_ref,omitempty"`
}
type ArgumentSite struct {
	ID             string           `json:"id"`
	SiteRef        string           `json:"site_ref,omitempty"`
	SiteLimitation string           `json:"site_limitation,omitempty"`
	API            string           `json:"api"`
	Location       ArgumentLocation `json:"location"`
	Roles          []ArgumentRole   `json:"roles"`
	CallRef        string           `json:"call_ref,omitempty"`
	Resolution     string           `json:"resolution"`
}
type ArgumentRole struct {
	Role     string `json:"role"`
	Index    int    `json:"index"`
	ValueRef string `json:"value_ref,omitempty"`
	Boundary string `json:"boundary,omitempty"`
}
type ArgumentCall struct {
	ID          string           `json:"id"`
	CallSiteRef string           `json:"callsite_ref,omitempty"`
	Caller      string           `json:"caller"`
	Callee      string           `json:"callee,omitempty"`
	Location    ArgumentLocation `json:"location"`
	Resolution  string           `json:"resolution"`
	Arguments   []string         `json:"arguments"`
	Results     []string         `json:"results"`
}
type ArgumentTruncation struct {
	Limit     string `json:"limit"`
	At        string `json:"at,omitempty"`
	Preserved string `json:"preserved"`
}

type argumentProjector struct {
	g         *graph
	opts      ArgumentProvenanceOptions
	out       ArgumentProvenance
	ids       map[ssa.Value]string
	active    map[ssa.Value]bool
	values    map[string]bool
	edges     map[string]bool
	truncated map[string]bool
	calls     int
	callIDs   map[string]bool
}

func (g *graph) analyzeArgumentProvenance(opts ArgumentProvenanceOptions) (ArgumentProvenance, error) {
	defaults := []struct {
		p *int
		n int
	}{{&opts.MaxDepth, 8}, {&opts.MaxNodes, 2000}, {&opts.MaxCalls, 500}, {&opts.MaxAlternatives, 32}, {&opts.MaxJSONBytes, 1 << 20}}
	for _, d := range defaults {
		if *d.p <= 0 {
			*d.p = d.n
		}
	}
	p := &argumentProjector{g: g, opts: opts, ids: map[ssa.Value]string{}, active: map[ssa.Value]bool{}, values: map[string]bool{}, edges: map[string]bool{}, truncated: map[string]bool{}, callIDs: map[string]bool{}}
	p.out = ArgumentProvenance{SchemaVersion: 1, Schema: "reffector.argument-provenance.v1", Nodes: []ArgumentValue{}, Edges: []ArgumentEdge{}, Sites: []ArgumentSite{}, Calls: []ArgumentCall{}, Truncation: []ArgumentTruncation{}}
	p.project()
	p.repairReferences()
	p.sort()
	if err := p.enforceBytes(); err != nil {
		return ArgumentProvenance{}, err
	}
	return p.out, nil
}
func (p *argumentProjector) mark(limit, at string) {
	if p.truncated[limit] {
		return
	}
	p.truncated[limit] = true
	p.out.Truncation = append(p.out.Truncation, ArgumentTruncation{Limit: limit, At: at, Preserved: "partial_evidence"})
}

// argumentValueID includes function identity, instruction ordinal and source location; no short collision-prone names.
func (p *argumentProjector) valueID(v ssa.Value) string {
	if id := p.ids[v]; id != "" {
		return id
	}
	fn := v.Parent()
	fnID := ""
	if fn != nil {
		fnID = symbolKey(p.g.symbol(fn))
	}
	ordinal := 0
	if fn != nil {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if value, ok := in.(ssa.Value); ok && value == v {
					goto found
				}
				ordinal++
			}
		}
	}
found:
	position := p.g.location(v.Pos())
	key := fmt.Sprintf("%s\x00%s\x00%d\x00%s\x00%s", fnID, v.Name(), ordinal, p.g.positionKey(position), fmt.Sprintf("%T", v))
	id := stableID("value", key)
	p.ids[v] = id
	return id
}
func (p *argumentProjector) addValue(v ssa.Value, kind, label, boundary string) string {
	id := p.valueID(v)
	if p.values[id] {
		if kind == "cycle" {
			for i := range p.out.Nodes {
				if p.out.Nodes[i].ID == id {
					p.out.Nodes[i].Kind = "cycle"
					p.out.Nodes[i].Boundary = boundary
					break
				}
			}
		}
		return id
	}
	if len(p.out.Nodes) >= p.opts.MaxNodes {
		p.mark("ssa_value_nodes", id)
		return id
	}
	var loc ArgumentLocation
	if v.Pos().IsValid() {
		x := p.g.location(v.Pos())
		loc = ArgumentLocation{x.File, x.Line, x.Column}
	}
	p.values[id] = true
	p.out.Nodes = append(p.out.Nodes, ArgumentValue{ID: id, Kind: kind, Label: label, Location: loc, Boundary: boundary})
	return id
}
func (p *argumentProjector) edge(from, to, kind, resolution string) {
	p.edgeCall(from, to, kind, resolution, "")
}
func (p *argumentProjector) edgeCall(from, to, kind, resolution, callRef string) {
	k := from + "\x00" + to + "\x00" + kind + "\x00" + resolution + "\x00" + callRef
	if p.edges[k] {
		return
	}
	p.edges[k] = true
	p.out.Edges = append(p.out.Edges, ArgumentEdge{ID: stableID("origin", k), From: from, To: to, Kind: kind, Resolution: resolution, CallRef: callRef})
}
func (p *argumentProjector) trace(v ssa.Value, depth int) string {
	if v == nil {
		return ""
	}
	id := p.valueID(v)
	if p.active[v] {
		p.addValue(v, "cycle", v.Name(), "recursive_ssa_value")
		return id
	}
	if p.values[id] {
		return id
	}
	if depth > p.opts.MaxDepth {
		p.mark("depth", id)
		p.addValue(v, "unresolved", "depth_limit", "depth_limit")
		return id
	}
	kind, label, boundary := "unresolved", "unsupported_value", "unsupported_ssa_value"
	switch x := v.(type) {
	case *ssa.Const:
		kind = "constant"
		label = "nil"
		if x.Value != nil {
			label = x.Value.String()
		}
		boundary = ""
	case *ssa.Parameter:
		kind = "parameter"
		label = x.Name()
		boundary = ""
	case *ssa.Phi:
		kind = "phi"
		label = "phi"
		boundary = ""
	case *ssa.BinOp:
		kind = "composition"
		label = x.Op.String()
		boundary = ""
	case *ssa.ChangeType, *ssa.Convert, *ssa.ChangeInterface, *ssa.MakeInterface:
		kind = "conversion"
		label = "conversion"
		boundary = ""
	case *ssa.UnOp:
		if x.Op.String() == "*" {
			switch x.X.(type) {
			case *ssa.Global:
				kind, label, boundary = "unresolved", "global_state", "global_state"
			case *ssa.FreeVar:
				kind, label, boundary = "unresolved", "closure_capture", "closure_capture"
			case *ssa.Alloc, *ssa.FieldAddr, *ssa.IndexAddr:
				kind, label, boundary = "unresolved", "mutable_heap_state", "mutable_heap_state"
			}
		}
	case *ssa.Extract:
		kind = "extract"
		label = strconv.Itoa(x.Index)
		boundary = ""
	case *ssa.Call:
		kind = "call_result"
		label = "call_result"
		if callee := x.Common().StaticCallee(); callee == nil || len(callee.Blocks) == 0 || p.g.isStandardLibrary(callee) {
			boundary = "external_or_dynamic_call_return"
		} else {
			boundary = ""
		}
		if builtin, ok := x.Common().Value.(*ssa.Builtin); ok && builtin.Name() == "append" {
			kind, label, boundary = "aggregate", "append", ""
		}
	case *ssa.MakeSlice:
		kind = "aggregate"
		label = "slice"
		boundary = "slice_element_stores_unresolved"
	case *ssa.Alloc, *ssa.MakeMap, *ssa.MakeChan, *ssa.FieldAddr, *ssa.IndexAddr, *ssa.Lookup, *ssa.Global, *ssa.FreeVar, *ssa.MakeClosure, *ssa.Field, *ssa.Index:
		kind = "unresolved"
		label = "mutable_or_external_boundary"
		boundary = boundaryFor(v)
		if alloc, ok := v.(*ssa.Alloc); ok && alloc.Type() != nil {
			if _, isSlice := types.Unalias(alloc.Type()).Underlying().(*types.Slice); isSlice {
				boundary = "slice_element_stores_unresolved"
			}
		}
	}
	p.addValue(v, kind, label, boundary)
	p.active[v] = true
	defer delete(p.active, v)
	inputs := []ssa.Value{}
	edgeKind := "input"
	switch x := v.(type) {
	case *ssa.Phi:
		inputs = x.Edges
	case *ssa.UnOp:
		if x.Op.String() == "*" {
			switch x.X.(type) {
			case *ssa.FreeVar, *ssa.Alloc, *ssa.FieldAddr, *ssa.IndexAddr:
				inputs = []ssa.Value{x.X}
			}
		}
	case *ssa.BinOp:
		inputs = []ssa.Value{x.X, x.Y}
	case *ssa.ChangeType:
		inputs = []ssa.Value{x.X}
	case *ssa.Convert:
		inputs = []ssa.Value{x.X}
	case *ssa.ChangeInterface:
		inputs = []ssa.Value{x.X}
	case *ssa.MakeInterface:
		inputs = []ssa.Value{x.X}
	case *ssa.Extract:
		if _, ok := x.Tuple.(*ssa.Call); ok {
			inputs = p.callOriginsForExtract(x)
		} else {
			inputs = []ssa.Value{x.Tuple}
		}
	case *ssa.Slice:
		inputs = []ssa.Value{x.X, x.Low, x.High, x.Max}
	case *ssa.Call:
		if builtin, ok := x.Common().Value.(*ssa.Builtin); ok && builtin.Name() == "append" {
			// Append's explicit SSA operands are direct components only; do not
			// walk the backing slice or infer values from its element stores.
			inputs = x.Common().Args
		} else {
			inputs = p.callOrigins(x)
		}
	case *ssa.Parameter:
		// Caller relations retain their callsite and resolution below.
	}
	if param, ok := v.(*ssa.Parameter); ok {
		for i, actual := range p.parameterActuals(param) {
			if i >= p.opts.MaxAlternatives {
				p.mark("alternative_origins", id)
				break
			}
			from := p.trace(actual.value, depth+1)
			callID := p.projectCall(actual.caller, actual.call, p.g.location(actual.call.Pos()))
			if actual.resolution != "established" {
				baseID := stableID("call", symbolKey(p.g.symbol(actual.caller))+"\x00"+p.g.positionKey(p.g.location(actual.call.Pos())))
				callID = stableID("call", baseID+"\x00"+symbolKey(p.g.symbol(param.Parent()))+"\x00"+actual.resolution)
			}
			p.edgeCall(from, id, "input", actual.resolution, callID)
		}
	}
	for i, in := range inputs {
		if in == nil {
			continue
		}
		if i >= p.opts.MaxAlternatives {
			p.mark("alternative_origins", id)
			break
		}
		from := p.trace(in, depth+1)
		p.edge(from, id, edgeKind, "established")
	}
	return id
}

type parameterActual struct {
	value      ssa.Value
	caller     *ssa.Function
	call       ssa.CallInstruction
	resolution string
}

// parameterActuals links a formal to established and candidate callers without merging their origins.
func (p *argumentProjector) parameterActuals(param *ssa.Parameter) []parameterActual {
	fn := param.Parent()
	if fn == nil {
		return nil
	}
	index := -1
	for i, formal := range fn.Params {
		if formal == param {
			index = i
			break
		}
	}
	if index < 0 {
		return nil
	}
	var out []parameterActual
	callers := make([]*ssa.Function, 0, len(p.g.nodes))
	for caller := range p.g.nodes {
		callers = append(callers, caller)
	}
	sort.Slice(callers, func(i, j int) bool { return callers[i].String() < callers[j].String() })
	for _, caller := range callers {
		for _, block := range caller.Blocks {
			for _, ins := range block.Instrs {
				call, ok := ins.(ssa.CallInstruction)
				if !ok || index >= len(call.Common().Args) {
					continue
				}
				resolution := ""
				if call.Common().StaticCallee() == fn {
					resolution = "established"
				} else {
					for _, target := range p.g.callCandidates(call) {
						if target == fn {
							resolution = "candidate_vta"
							if containsExactFunction(p.g.functionCandidates[call], fn) {
								resolution = "candidate_function_value"
							}
							break
						}
					}
				}
				if resolution != "" {
					out = append(out, parameterActual{value: call.Common().Args[index], caller: caller, call: call, resolution: resolution})
				}
			}
		}
	}
	return out
}

func boundaryFor(v ssa.Value) string {
	switch v.(type) {
	case *ssa.Global:
		return "global_state"
	case *ssa.MakeClosure, *ssa.FreeVar:
		return "closure_capture"
	case *ssa.MakeMap, *ssa.Lookup:
		return "map_state"
	case *ssa.Alloc, *ssa.FieldAddr, *ssa.IndexAddr:
		return "mutable_heap_state"
	}
	return "unsupported_ssa_value"
}

// callOrigins follows only statically known, local bodies and their explicit return operands.
func (p *argumentProjector) callOrigins(call *ssa.Call) []ssa.Value {
	return p.callOriginsForResult(call, -1)
}
func (p *argumentProjector) callOriginsForExtract(extract *ssa.Extract) []ssa.Value {
	call, ok := extract.Tuple.(*ssa.Call)
	if !ok {
		return nil
	}
	return p.callOriginsForResult(call, extract.Index)
}
func (p *argumentProjector) callOriginsForResult(call *ssa.Call, resultIndex int) []ssa.Value {
	fn := call.Common().StaticCallee()
	if fn == nil || len(fn.Blocks) == 0 || p.g.isStandardLibrary(fn) || !sourceFunction(fn) {
		return nil
	}
	out := []ssa.Value{}
	for _, b := range fn.Blocks {
		if len(b.Instrs) == 0 {
			continue
		}
		if ret, ok := b.Instrs[len(b.Instrs)-1].(*ssa.Return); ok {
			for ri, result := range ret.Results {
				if resultIndex >= 0 && ri != resultIndex {
					continue
				}
				if param, ok := result.(*ssa.Parameter); ok {
					for i, formal := range fn.Params {
						if formal == param && i < len(call.Common().Args) {
							out = append(out, call.Common().Args[i])
							break
						}
					}
				} else {
					out = append(out, result)
				}
			}
		}
	}
	return out
}
func boundaryForCmd(api string) bool {
	return strings.HasPrefix(api, "os/exec.(*Cmd).") && (strings.HasSuffix(api, ".Start") || strings.HasSuffix(api, ".Run") || strings.HasSuffix(api, ".Output") || strings.HasSuffix(api, ".CombinedOutput"))
}
func argumentRoles(api string) []ArgumentRole {
	var names []string
	start := 0
	switch api {
	case "os.WriteFile", "io/ioutil.WriteFile":
		names = []string{"path", "data"}
	case "os.(*Root).WriteFile":
		start = 1
		names = []string{"path", "data"}
	case "os.Remove", "os.RemoveAll", "os.Mkdir", "os.MkdirAll", "os.Create", "os.Chdir", "os.OpenFile", "os.Truncate", "os.Chmod", "os.Chown", "os.Lchown", "os.Chtimes":
		names = []string{"path"}
	case "os.OpenInRoot":
		names = []string{"dir", "path"}
	case "os.(*Root).Open", "os.(*Root).OpenRoot", "os.(*Root).Create", "os.(*Root).OpenFile", "os.(*Root).Remove", "os.(*Root).RemoveAll", "os.(*Root).Mkdir", "os.(*Root).MkdirAll", "os.(*Root).Chdir", "os.(*Root).Chmod", "os.(*Root).Chown", "os.(*Root).Lchown", "os.(*Root).Chtimes":
		start = 1
		names = []string{"path"}
	case "io/ioutil.TempFile", "io/ioutil.TempDir", "os.CreateTemp", "os.MkdirTemp":
		names = []string{"dir", "pattern"}
	case "os.Rename", "os.Link", "os.Symlink":
		names = []string{"old_path", "new_path"}
	case "os.(*Root).Rename", "os.(*Root).Link", "os.(*Root).Symlink":
		start = 1
		names = []string{"old_path", "new_path"}
	case "os.StartProcess":
		names = []string{"path", "argv"}
	case "os/exec.(*Cmd).Start", "os/exec.(*Cmd).Run", "os/exec.(*Cmd).Output", "os/exec.(*Cmd).CombinedOutput":
		names = []string{"receiver"}
	}
	out := make([]ArgumentRole, 0, len(names))
	for _, name := range names {
		out = append(out, ArgumentRole{Role: name, Index: start + len(out)})
	}
	return out
}
func (p *argumentProjector) project() {
	funcs := make([]*ssa.Function, 0, len(p.g.nodes))
	for fn := range p.g.nodes {
		funcs = append(funcs, fn)
	}
	sort.Slice(funcs, func(i, j int) bool {
		a, b := p.g.variant(funcs[i]), p.g.variant(funcs[j])
		if a != b {
			return a < b
		}
		return funcs[i].String() < funcs[j].String()
	})
	for _, fn := range funcs {
		for _, b := range fn.Blocks {
			for _, ins := range b.Instrs {
				c, ok := ins.(ssa.CallInstruction)
				if !ok {
					continue
				}
				common := c.Common()
				callee := common.StaticCallee()
				loc := p.g.location(ins.Pos())
				baseSiteID := stableID("site", symbolKey(p.g.symbol(fn))+"\x00"+p.g.positionKey(loc))
				type apiVariant struct {
					api       string
					roles     []ArgumentRole
					candidate bool
				}
				variants := []apiVariant{}
				if callee != nil {
					if obj, ok := callee.Object().(*types.Func); ok {
						api := effects.Key(obj)
						if roles := argumentRoles(api); len(roles) > 0 || boundaryForCmd(api) {
							variants = append(variants, apiVariant{api, roles, false})
						}
					}
				}
				// Candidate targets are limited to graph evidence; interface dispatch
				// without a concrete candidate remains unresolved rather than inferred.
				targets := append([]*ssa.Function(nil), p.g.callCandidates(c)...)
				sort.Slice(targets, func(i, j int) bool { return symbolKey(p.g.symbol(targets[i])) < symbolKey(p.g.symbol(targets[j])) })
				for _, target := range targets {
					if target == nil || target == callee {
						continue
					}
					if obj, ok := target.Object().(*types.Func); ok {
						api := effects.Key(obj)
						roles := argumentRoles(api)
						if len(roles) > 0 {
							variants = append(variants, apiVariant{api, roles, true})
						}
					}
				}
				if len(variants) == 0 {
					continue
				}
				callID := p.projectCall(fn, c, loc)
				for _, variant := range variants {
					api, roleSpecs, candidateResolution := variant.api, variant.roles, variant.candidate
					candidateCallResolution := ""
					if candidateResolution {
						for _, target := range targets {
							if obj, ok := target.Object().(*types.Func); ok && effects.Key(obj) == api {
								res := "candidate_vta"
								if containsExactFunction(p.g.functionCandidates[c], target) {
									res = "candidate_function_value"
								}
								candidateCallResolution = res
								callID = stableID("call", stableID("call", symbolKey(p.g.symbol(fn))+"\x00"+p.g.positionKey(loc))+"\x00"+symbolKey(p.g.symbol(target))+"\x00"+res)
								break
							}
						}
					}
					cmd := boundaryForCmd(api)
					roles := len(roleSpecs) > 0
					if !roles && !cmd {
						continue
					}
					siteID := baseSiteID
					if candidateResolution {
						siteID = stableID("operation", baseSiteID+"\\x00"+api)
					}
					site := ArgumentSite{ID: siteID, API: api, Location: ArgumentLocation{loc.File, loc.Line, loc.Column}, Roles: []ArgumentRole{}, Resolution: "established", CallRef: callID}
					if candidateResolution {
						site.Resolution = candidateCallResolution
					} else if callee == nil {
						site.Resolution = "unresolved_dynamic"
					}
					if p.g.sites[baseSiteID] != nil {
						site.SiteRef = baseSiteID
					} else {
						site.SiteLimitation = "effect_operation_site_unavailable"
						if candidateResolution {
							site.SiteLimitation = "candidate_effect_api_without_established_operation_site"
						} else if callee == nil {
							site.SiteLimitation = "dynamic_effect_api_without_established_operation_site"
						} else {
							site.Resolution = "unresolved_effect_site_unavailable"
						}
					}
					argOffset := 0
					if callee != nil && callee.Signature.Recv() != nil {
						argOffset = 1
					}
					if cmd {
						site.Resolution = "unresolved_mutable_receiver"
						receiver := ssa.Value(common.Value)
						if argOffset == 1 && len(common.Args) > 0 {
							receiver = common.Args[0]
						}
						rid := p.trace(receiver, 0)
						site.Roles = append(site.Roles, ArgumentRole{Role: "receiver", ValueRef: rid, Boundary: "mutable_cmd_receiver_fields_not_inferred"})
					} else {
						for _, spec := range roleSpecs {
							var arg ssa.Value
							if spec.Index < len(common.Args) {
								arg = common.Args[spec.Index]
							}
							id := p.trace(arg, 0)
							site.Roles = append(site.Roles, ArgumentRole{Role: spec.Role, Index: spec.Index, ValueRef: id})
						}
					}
					duplicate := false
					for _, existing := range p.out.Sites {
						if existing.ID == site.ID {
							duplicate = true
							break
						}
					}
					if !duplicate {
						p.out.Sites = append(p.out.Sites, site)
					}
				}
			}
		}
	}
}
func (p *argumentProjector) projectCall(fn *ssa.Function, call ssa.CallInstruction, loc Location) string {
	if p.calls >= p.opts.MaxCalls {
		p.mark("call_relations", "")
		return ""
	}
	p.calls++
	common := call.Common()
	id := stableID("call", symbolKey(p.g.symbol(fn))+"\x00"+p.g.positionKey(loc))
	resolution := "unresolved_dynamic"
	callee := ""
	if target := common.StaticCallee(); target != nil {
		callee = symbolKey(p.g.symbol(target))
		resolution = "established"
	}
	args := []string{}
	for _, arg := range common.Args {
		args = append(args, p.trace(arg, 0))
	}
	results := []string{}
	if v, ok := call.(ssa.Value); ok && v.Type() != nil { // Only instruction results that are values.
		if _, isVoid := types.Unalias(v.Type()).Underlying().(*types.Tuple); !isVoid {
			results = append(results, p.trace(v, 0))
		}
	}
	if !p.callIDs[id] {
		p.callIDs[id] = true
		siteRef := stableID("site", symbolKey(p.g.symbol(fn))+"\x00"+p.g.positionKey(loc))
		p.out.Calls = append(p.out.Calls, ArgumentCall{ID: id, CallSiteRef: siteRef, Caller: symbolKey(p.g.symbol(fn)), Callee: callee, Location: ArgumentLocation{loc.File, loc.Line, loc.Column}, Resolution: resolution, Arguments: args, Results: results})
	}
	for _, target := range p.g.callCandidates(call) {
		if target == nil || target == common.StaticCallee() {
			continue
		}
		if p.calls >= p.opts.MaxCalls {
			p.mark("call_relations", id)
			break
		}
		p.calls++
		res := "candidate_vta"
		if containsExactFunction(p.g.functionCandidates[call], target) {
			res = "candidate_function_value"
		}
		candidateID := stableID("call", id+"\x00"+symbolKey(p.g.symbol(target))+"\x00"+res)
		if !p.callIDs[candidateID] {
			p.callIDs[candidateID] = true
			siteRef := stableID("site", symbolKey(p.g.symbol(fn))+"\x00"+p.g.positionKey(loc))
			p.out.Calls = append(p.out.Calls, ArgumentCall{ID: candidateID, CallSiteRef: siteRef, Caller: symbolKey(p.g.symbol(fn)), Callee: symbolKey(p.g.symbol(target)), Location: ArgumentLocation{loc.File, loc.Line, loc.Column}, Resolution: res, Arguments: args, Results: []string{}})
		}
	}
	return id
}
func (p *argumentProjector) sort() {
	sort.Slice(p.out.Nodes, func(i, j int) bool { return p.out.Nodes[i].ID < p.out.Nodes[j].ID })
	sort.Slice(p.out.Edges, func(i, j int) bool { return p.out.Edges[i].ID < p.out.Edges[j].ID })
	sort.Slice(p.out.Sites, func(i, j int) bool { return p.out.Sites[i].ID < p.out.Sites[j].ID })
	sort.Slice(p.out.Calls, func(i, j int) bool { return p.out.Calls[i].ID < p.out.Calls[j].ID })
	sort.Slice(p.out.Truncation, func(i, j int) bool { return p.out.Truncation[i].Limit < p.out.Truncation[j].Limit })
}
func (p *argumentProjector) enforceBytes() error {
	// Deterministically remove least-essential trailing records until compact JSON fits.
	for {
		b, _ := json.Marshal(p.out)
		if len(b) <= p.opts.MaxJSONBytes {
			return nil
		}
		if len(p.out.Edges) > 0 {
			p.out.Edges = p.out.Edges[:len(p.out.Edges)-1]
		} else if len(p.out.Nodes) > 0 {
			p.out.Nodes = p.out.Nodes[:len(p.out.Nodes)-1]
		} else if len(p.out.Calls) > 0 {
			p.out.Calls = p.out.Calls[:len(p.out.Calls)-1]
		} else if len(p.out.Sites) > 0 {
			p.out.Sites = p.out.Sites[:len(p.out.Sites)-1]
		} else {
			return fmt.Errorf("argument provenance JSON byte limit %d is below the minimum valid schema payload", p.opts.MaxJSONBytes)
		}
		p.mark("json_bytes", "")
		p.repairReferences()
	}
}

// repairReferences removes links to records omitted by independent output limits.
func (p *argumentProjector) repairReferences() {
	nodes := map[string]bool{}
	for _, n := range p.out.Nodes {
		nodes[n.ID] = true
	}
	edges := p.out.Edges[:0]
	for _, e := range p.out.Edges {
		if nodes[e.From] && nodes[e.To] {
			edges = append(edges, e)
		}
	}
	p.out.Edges = edges
	calls := map[string]bool{}
	for _, c := range p.out.Calls {
		calls[c.ID] = true
	}
	for i := range p.out.Edges {
		if p.out.Edges[i].CallRef != "" && !calls[p.out.Edges[i].CallRef] {
			p.out.Edges[i].CallRef = ""
		}
	}
	for i := range p.out.Sites {
		if p.out.Sites[i].CallRef != "" && !calls[p.out.Sites[i].CallRef] {
			p.out.Sites[i].CallRef = ""
			p.out.Sites[i].SiteLimitation = "call_relation_omitted_by_output_limit"
		}
		for j := range p.out.Sites[i].Roles {
			if !nodes[p.out.Sites[i].Roles[j].ValueRef] {
				p.out.Sites[i].Roles[j].ValueRef = ""
				p.out.Sites[i].Roles[j].Boundary = "value_omitted_by_output_limit"
			}
		}
	}
	for i := range p.out.Calls {
		args := p.out.Calls[i].Arguments[:0]
		for _, ref := range p.out.Calls[i].Arguments {
			if nodes[ref] {
				args = append(args, ref)
			}
		}
		p.out.Calls[i].Arguments = args
		results := p.out.Calls[i].Results[:0]
		for _, ref := range p.out.Calls[i].Results {
			if nodes[ref] {
				results = append(results, ref)
			}
		}
		p.out.Calls[i].Results = results
	}
}
func ptrArgumentProvenance(p ArgumentProvenance) *ArgumentProvenance { return &p }
func appendUniqueString(xs []string, x string) []string {
	for _, v := range xs {
		if v == x {
			return xs
		}
	}
	return append(xs, x)
}
