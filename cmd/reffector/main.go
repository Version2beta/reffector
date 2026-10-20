// Command reffector reports statically reachable Go side effects.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

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
	direct := flags.Bool("direct", false, "direct function effects; effect view restricts origins to selected roots")
	tests := flags.Bool("tests", false, "include functions defined in test files")
	asJSON := flags.Bool("json", false, "write structured JSON for the selected view")
	view := flags.String("view", "effects", "output view: effects (groups/sites/reverse callers) or functions (compatible summaries)")
	apiFilter := flags.String("api", "", "select an exact Go API/method identity (effect view)")
	siteFilter := flags.String("site", "", "select one canonical direct site ID (effect view)")
	unresolved := flags.Bool("unresolved", false, "switch from recognized operations to unresolved-only leads (effect view)")
	reasonFilter := flags.String("reason", "", "filter uncertainty reasons by substring, e.g. interface_dispatch (effect view)")
	contracts := flags.Bool("contracts", false, "include method-set-compatible implementation leads for matching interface calls (effect view)")
	exact := flags.Bool("exact", false, "select local directory packages without subpackages")
	all := flags.Bool("all", false, "include full source inventory, independently of effect-view filters")
	stats := flags.Bool("stats", false, "write coverage counts as JSON to stderr")
	details := flags.Bool("details", false, "write compatibility function/witness JSON (implies -view functions -json)")
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
	if (*view != "effects" && *view != "functions") || *maxDepth < 0 || *maxPaths < 1 || *maxSites < 1 || *maxEdges < 1 || (*view == "functions" && ((!*details && *operationFilter != "") || *apiFilter != "" || *siteFilter != "" || *unresolved || *reasonFilter != "" || *contracts)) {
		fmt.Fprintln(stderr, "reffector: use -view effects|functions, nonnegative depth, positive limits; -api/-site/-unresolved/-reason/-contracts require effect view; -operation requires effect view or -details")
		return 2
	}
	report, err := analyzer.AnalyzeReport(flags.Args(), analyzer.Options{
		Direct: *direct, Tests: *tests, Exact: *exact, All: *all, Details: *details,
		EffectView: *view == "effects", API: *apiFilter, Site: *siteFilter, Unresolved: *unresolved, Contracts: *contracts, Reason: *reasonFilter,
		Package: *packageFilter, Function: *functionFilter, Effect: *effectFilter, Operation: *operationFilter,
		MaxDepth: *maxDepth, MaxPaths: *maxPaths, MaxSites: *maxSites, MaxEdges: *maxEdges,
	})
	if err != nil {
		fmt.Fprintln(stderr, "reffector:", err)
		return 1
	}
	if *stats {
		var coverage any = report.Stats
		if report.Effects != nil {
			coverage = report.Effects.Coverage
		}
		if err := json.NewEncoder(stderr).Encode(coverage); err != nil {
			return 1
		}
	}
	if report.Effects != nil {
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
