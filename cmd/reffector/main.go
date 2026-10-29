// Command reffector reports statically reachable Go side effects.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/Version2beta/reffector/internal/analyzer"
	"github.com/Version2beta/reffector/internal/output"
	"github.com/Version2beta/reffector/internal/skillinstall"
	"github.com/Version2beta/reffector/skills/reffector"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "skill" {
		return runSkill(args[1:], stdout, stderr)
	}
	flags := flag.NewFlagSet("reffector", flag.ContinueOnError)
	flags.SetOutput(stderr)
	audit := flags.Bool("audit-unresolved", false, "write unlimited uncertain source sites as JSON Lines without reverse graphs")
	auditKnown := flags.Bool("audit-include-known", false, "also export recognized-effect sites with uncertainty (requires -audit-unresolved)")
	direct := flags.Bool("direct", false, "direct function effects; effect view restricts origins to selected roots")
	tests := flags.Bool("tests", false, "include functions defined in test files")
	asJSON := flags.Bool("json", false, "write structured JSON for the selected view")
	view := flags.String("view", "effects", "output view: effects (groups/sites/reverse callers) or functions (compatible summaries)")
	apiFilter := flags.String("api", "", "select an exact Go API/method identity (effect view)")
	siteFilter := flags.String("site", "", "select one canonical direct site ID (effect view)")
	unresolved := flags.Bool("unresolved", false, "switch from recognized operations to unresolved-only leads (effect view)")
	reasonFilter := flags.String("reason", "", "filter uncertainty reasons by substring, e.g. interface_dispatch (effect view)")
	contracts := flags.Bool("contracts", false, "include method-set-compatible implementation leads for matching interface calls (effect view)")
	dependencyVTAFunctions := flags.Int("dependency-vta-functions", 0, "maximum loaded dependency SSA functions added to VTA input (requires -dependency-vta-edges)")
	dependencyVTAEdges := flags.Int("dependency-vta-edges", 0, "maximum dependency static call edges traversed for VTA input (requires -dependency-vta-functions)")
	exact := flags.Bool("exact", false, "select local directory packages without subpackages")
	all := flags.Bool("all", false, "include full source inventory, independently of effect-view filters")
	stats := flags.Bool("stats", false, "write coverage counts as JSON to stderr")
	details := flags.Bool("details", false, "write compatibility function/witness JSON (implies -view functions -json)")
	argumentProvenance := flags.Bool("argument-provenance", false, "emit bounded SSA argument provenance JSON (standalone opt-in projection)")
	controlFlow := flags.Bool("control-flow", false, "emit bounded SSA control-flow JSON (standalone opt-in projection)")
	controlFlowFunctions := flags.Int("control-flow-max-functions", 64, "control-flow maximum unique functions")
	controlFlowBlocks := flags.Int("control-flow-max-blocks-per-function", 128, "control-flow maximum blocks per function")
	controlFlowVisits := flags.Int("control-flow-max-block-visits", 4096, "control-flow maximum total block visits")
	controlFlowBranches := flags.Int("control-flow-max-branches", 256, "control-flow maximum branches")
	controlFlowRelationships := flags.Int("control-flow-max-relationships", 2048, "control-flow maximum relationships")
	controlFlowDepth := flags.Int("control-flow-max-depth", 8, "control-flow maximum export depth")
	controlFlowBytes := flags.Int("control-flow-max-json-bytes", 1048576, "control-flow maximum compact JSON bytes")
	argumentMaxDepth := flags.Int("argument-max-depth", 8, "argument provenance maximum origin depth")
	argumentMaxNodes := flags.Int("argument-max-nodes", 2000, "argument provenance maximum SSA value nodes")
	argumentMaxCalls := flags.Int("argument-max-calls", 500, "argument provenance maximum call relations")
	argumentMaxAlternatives := flags.Int("argument-max-alternatives", 32, "argument provenance maximum alternative origins per value")
	argumentMaxJSONBytes := flags.Int("argument-max-json-bytes", 1048576, "argument provenance maximum compact JSON bytes")
	packageFilter := flags.String("package", "", "select source-root packages by substring (summaries in function view)")
	functionFilter := flags.String("function", "", "select source-root functions by substring; prefix = for one exact function")
	effectFilter := flags.String("effect", "", "filter effect identifiers by substring (e.g. fs.rename, resource.release); see README for vocabulary")
	operationFilter := flags.String("operation", "", "filter site API identities by substring (effect view or -details)")
	maxDepth := flags.Int("max-depth", 6, "maximum reverse source-call depth (or compatibility witness depth); zero shows origins")
	maxPaths := flags.Int("max-paths", 1000, "compatibility -details witness budget")
	maxSites := flags.Int("max-sites", 1000, "maximum source sites")
	maxEdges := flags.Int("max-edges", 1000, "maximum unique reverse edges (effect view)")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: reffector [flags] [packages]")
		fmt.Fprintln(stderr, "       reffector skill install")
		fmt.Fprintln(stderr, "Report statically reachable side effects; local directories recurse (default .)")
		fmt.Fprintln(stderr, "Install the bundled Pi skill with: reffector skill install")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	specified := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { specified[f.Name] = true })
	if *auditKnown && !*audit {
		fmt.Fprintln(stderr, "reffector: -audit-include-known requires -audit-unresolved")
		return 2
	}
	if specified["dependency-vta-functions"] != specified["dependency-vta-edges"] || (specified["dependency-vta-functions"] && (*dependencyVTAFunctions < 1 || *dependencyVTAEdges < 1)) {
		fmt.Fprintln(stderr, "reffector: dependency VTA requires both positive -dependency-vta-functions and -dependency-vta-edges budgets")
		return 2
	}
	if *audit {
		allowed := map[string]bool{"audit-unresolved": true, "audit-include-known": true, "tests": true, "exact": true, "stats": true, "all": true, "dependency-vta-functions": true, "dependency-vta-edges": true}
		for name := range specified {
			if !allowed[name] {
				fmt.Fprintln(stderr, "reffector: -audit-unresolved supports only -audit-include-known/-tests/-exact/-stats/-all; incompatible flag: -"+name)
				return 2
			}
		}
	}
	explicitView := specified["view"]
	if *details {
		if explicitView && *view != "functions" {
			fmt.Fprintln(stderr, "reffector: -details is the function compatibility view; omit -view or use -view functions")
			return 2
		}
		*view = "functions"
	}
	if (*view == "effects" && specified["max-paths"]) || (*view == "functions" && specified["max-edges"]) || (*view == "functions" && !*details && (specified["max-depth"] || specified["max-sites"] || specified["max-paths"])) {
		fmt.Fprintln(stderr, "reffector: reverse -max-edges requires effect view; -max-paths requires -details; traversal limits require a graph view")
		return 2
	}
	if (*view != "effects" && *view != "functions") || *controlFlowFunctions < 1 || *controlFlowBlocks < 1 || *controlFlowVisits < 1 || *controlFlowBranches < 1 || *controlFlowRelationships < 1 || *controlFlowDepth < 1 || *controlFlowBytes < 1 || *argumentMaxDepth < 1 || *argumentMaxNodes < 1 || *argumentMaxCalls < 1 || *argumentMaxAlternatives < 1 || *argumentMaxJSONBytes < 1 || *maxDepth < 0 || *maxPaths < 1 || *maxSites < 1 || *maxEdges < 1 || (*view == "functions" && ((!*details && *operationFilter != "") || *apiFilter != "" || *siteFilter != "" || *unresolved || *reasonFilter != "" || *contracts)) {
		fmt.Fprintln(stderr, "reffector: use -view effects|functions, nonnegative depth, positive limits; -api/-site/-unresolved/-reason/-contracts require effect view; -operation requires effect view or -details")
		return 2
	}
	if *controlFlow && *argumentProvenance {
		fmt.Fprintln(stderr, "reffector: -control-flow and -argument-provenance emit mutually exclusive projections")
		return 2
	}
	if *controlFlow && (*view != "effects" || *audit) {
		fmt.Fprintln(stderr, "reffector: -control-flow requires effect view and is incompatible with -audit-unresolved")
		return 2
	}
	analysisStarted := time.Now()
	report, err := analyzer.AnalyzeReport(flags.Args(), analyzer.Options{
		Direct: *direct, Tests: *tests, Exact: *exact, All: *all, Details: *details, ArgumentProvenance: *argumentProvenance, ControlFlow: *controlFlow,
		ControlFlowLimits:        analyzer.ControlFlowOptions{MaxFunctions: *controlFlowFunctions, MaxBlocksPerFunction: *controlFlowBlocks, MaxBlockVisits: *controlFlowVisits, MaxBranches: *controlFlowBranches, MaxRelationships: *controlFlowRelationships, MaxDepth: *controlFlowDepth, MaxJSONBytes: *controlFlowBytes},
		ArgumentProvenanceLimits: analyzer.ArgumentProvenanceOptions{MaxDepth: *argumentMaxDepth, MaxNodes: *argumentMaxNodes, MaxCalls: *argumentMaxCalls, MaxAlternatives: *argumentMaxAlternatives, MaxJSONBytes: *argumentMaxJSONBytes},
		AuditUnresolved:          *audit, AuditIncludeKnown: *auditKnown,
		DependencyVTAFunctions: *dependencyVTAFunctions, DependencyVTAEdges: *dependencyVTAEdges,
		EffectView: *view == "effects" && !*audit, API: *apiFilter, Site: *siteFilter, Unresolved: *unresolved, Contracts: *contracts, Reason: *reasonFilter,
		Package: *packageFilter, Function: *functionFilter, Effect: *effectFilter, Operation: *operationFilter,
		MaxDepth: *maxDepth, MaxPaths: *maxPaths, MaxSites: *maxSites, MaxEdges: *maxEdges,
	})
	if err != nil {
		fmt.Fprintln(stderr, "reffector:", err)
		return 1
	}
	if report.Stats.DependencyVTA != nil && !*stats && (report.Effects == nil || !*asJSON) {
		status := report.Stats.DependencyVTA.LimitHit
		if status == "" {
			status = "none"
		}
		fmt.Fprintf(stderr, "reffector: dependency VTA added %d functions across %d edges; truncated=%t limit_hit=%s (budgets %d functions/%d edges); context_states=%d/%d context_truncated=%t fallback_roots=%d\n",
			report.Stats.DependencyVTA.FunctionsAdded, report.Stats.DependencyVTA.EdgesAdded,
			report.Stats.DependencyVTA.Truncated, status,
			report.Stats.DependencyVTA.FunctionBudget, report.Stats.DependencyVTA.EdgeBudget,
			report.Stats.DependencyVTA.ContextStates, report.Stats.DependencyVTA.ContextBudget,
			report.Stats.DependencyVTA.ContextTruncated, report.Stats.DependencyVTA.ContextRootsFallback)
	}
	if *stats {
		var coverage any = report.Stats
		if report.Effects != nil {
			coverage = report.Effects.Coverage
		}
		if report.Audit != nil {
			coverage = report.Audit.Coverage
		}
		summary := make(map[string]json.RawMessage)
		encoded, marshalErr := json.Marshal(coverage)
		if marshalErr != nil {
			fmt.Fprintln(stderr, "reffector: encode stats:", marshalErr)
			return 1
		}
		if err := json.Unmarshal(encoded, &summary); err != nil {
			fmt.Fprintln(stderr, "reffector: encode stats:", err)
			return 1
		}
		duration, err := json.Marshal(time.Since(analysisStarted).Milliseconds())
		if err != nil {
			fmt.Fprintln(stderr, "reffector: encode stats:", err)
			return 1
		}
		summary["execution_duration_ms"] = duration
		if err := json.NewEncoder(stderr).Encode(summary); err != nil {
			return 1
		}
	}
	if report.ControlFlow != nil {
		err = json.NewEncoder(stdout).Encode(report.ControlFlow)
	} else if report.ArgumentProvenance != nil {
		err = json.NewEncoder(stdout).Encode(report.ArgumentProvenance)
	} else if report.Audit != nil {
		err = report.Audit.WriteJSONL(stdout)
	} else if report.Effects != nil {
		err = output.WriteEffects(stdout, report.Effects, *asJSON)
	} else if *details {
		err = output.WriteReport(stdout, report)
	} else {
		err = output.Write(stdout, report.Records, *asJSON)
	}
	if err != nil {
		fmt.Fprintln(stderr, "reffector:", err)
		return 1
	}
	return 0
}

func runSkill(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "install" {
		fmt.Fprintln(stderr, "Usage: reffector skill install")
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(stderr, "reffector: determine home directory:", err)
		return 1
	}
	result, err := skillinstall.Install(skillinstall.Config{Home: home, Name: "reffector", Content: reffectorskill.Content})
	if err != nil {
		fmt.Fprintln(stderr, "reffector:", err)
		return 1
	}
	if result.Updated {
		fmt.Fprintln(stdout, "Installed skill:", result.Path)
	} else {
		fmt.Fprintln(stdout, "Skill is current:", result.Path)
	}
	return 0
}
