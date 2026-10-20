package output

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/Version2beta/reffector/internal/analyzer"
)

// WriteEffects defines each site once. Groups and reverse relationships refer
// to these definitions; human output uses short local node labels for sharing.
func WriteEffects(w io.Writer, report *analyzer.EffectReport, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	b := bufio.NewWriter(w)
	sites := make(map[string]analyzer.Site, len(report.Sites))
	for _, site := range report.Sites {
		sites[site.ID] = site
	}
	writeSiteReference := func(id string) {
		site := sites[id]
		api := strings.Join(site.APIs, ", ")
		if api == "" {
			api = "unresolved operation"
		}
		fmt.Fprintf(b, "  %s — %s:%d:%d in %s.%s\n    %s\n", api, site.Location.File, site.Location.Line, site.Location.Column, site.Package, site.Function, id)
	}
	fmt.Fprintln(b, "Possible effects — static reachability, not execution proof")
	for _, group := range report.Groups {
		fmt.Fprintf(b, "%s (%d sites)\n", group.Effect, len(group.Sites))
		for _, id := range group.Sites {
			writeSiteReference(id)
		}
	}
	if len(report.Unresolved) > 0 {
		fmt.Fprintf(b, "Unresolved-only leads (%d; not confirmed effects)\n", len(report.Unresolved))
		for _, id := range report.Unresolved {
			writeSiteReference(id)
		}
	}
	nodes, labels, edges := map[string]analyzer.GraphNode{}, map[string]string{}, map[string]analyzer.GraphEdge{}
	symbolCounts := map[analyzer.Symbol]int{}
	for i, n := range report.Nodes {
		symbolCounts[n.Symbol]++
		nodes[n.ID] = n
		labels[n.ID] = fmt.Sprintf("n%d", i+1)
	}
	for _, e := range report.Edges {
		edges[e.ID] = e
	}
	graphs := map[string]analyzer.ReverseGraph{}
	for _, g := range report.Reverse {
		graphs[g.Site] = g
	}
	for _, site := range report.Sites {
		fmt.Fprintf(b, "\n%s\n  %s.%s — %s\n  %s:%d:%d [%s; %s]\n", site.ID, site.Package, site.Function, strings.Join(site.APIs, ", "), site.Location.File, site.Location.Line, site.Location.Column, site.Kind, site.Recognition)
		if len(site.Effects) > 0 {
			fmt.Fprintf(b, "  possible: %s\n", strings.Join(site.Effects, ", "))
		}
		if site.Uncertain {
			fmt.Fprintf(b, "  unresolved: %s\n", strings.Join(site.Reasons, ", "))
		}
		if len(site.Operations) > 1 {
			for _, op := range site.Operations {
				fmt.Fprintf(b, "  alternative %s [%s] possible=%s uncertain=%t reasons=%s\n", op.API, op.Recognition, strings.Join(op.Effects, ","), op.Uncertain, strings.Join(op.Reasons, ","))
			}
		}
		g := graphs[site.ID]
		origins := []string{}
		for _, id := range g.Origins {
			origins = append(origins, labels[id])
		}
		fmt.Fprintf(b, "  origins: %s; reverse callers:\n", strings.Join(origins, ", "))
		for _, ref := range g.Nodes {
			n := nodes[ref.ID]
			boundary := "dependency"
			if n.Selected {
				boundary = "selected"
			} else if n.Synthetic {
				boundary = "synthetic forwarding"
			}
			uncertainty := ""
			if n.DirectUncertain {
				uncertainty += "; direct uncertainty"
			}
			if n.InheritedUncertain {
				uncertainty += "; inherited uncertainty"
			}
			fmt.Fprintf(b, "    depth %d %s %s.%s [%s%s]\n", ref.Depth, labels[ref.ID], n.Package, n.Function, boundary, uncertainty)
			if symbolCounts[n.Symbol] > 1 {
				fmt.Fprintf(b, "      SSA variant: %s\n", n.Variant)
			}
		}
		for _, id := range g.Edges {
			e := edges[id]
			if e.Source == nil {
				fmt.Fprintf(b, "    %s -> %s [synthetic SSA forwarding]\n", labels[e.Caller], labels[e.Callee])
				continue
			}
			resolution := e.Source.Resolution
			if resolution == "" {
				resolution = "established"
			}
			fmt.Fprintf(b, "    %s -> %s at %s:%d:%d [%s]\n", labels[e.Caller], labels[e.Callee], e.Source.Location.File, e.Source.Location.Line, e.Source.Location.Column, resolution)
		}
		fmt.Fprintf(b, "  shared/revisited=%t cycle=%t; truncated depth=%t edges=%t\n", g.Revisit, g.Cycle, g.Truncated.Depth, g.Truncated.Edges)
	}
	c := report.Coverage
	fmt.Fprintln(b, "\nSelected packages:")
	for _, pkg := range c.SelectedPackages {
		fmt.Fprintf(b, "  %s\n", pkg)
	}
	fmt.Fprintf(b, "\nCoverage: %d packages, %d source functions; %d effectful, %d direct, %d inherited, %d uncertain, %d unresolved-only, %d no-effects/no-uncertainty (not purity)\n", c.Packages, c.SourceFunctions, c.Effectful, c.Direct, c.Inherited, c.Uncertain, c.UnresolvedOnly, c.Uncharacterized)
	fmt.Fprintf(b, "Sites: %d operations (%d selected, %d dependency), %d unresolved-only; %d match filters, %d emitted\n", c.OperationSites, c.SelectedOperationSites, c.DependencyOperationSites, c.UnresolvedSites, c.FilteredSites, c.EmittedSites)
	fmt.Fprintf(b, "Truncated: sites=%t edges=%t depth=%t. Unresolved origins: -unresolved; full source inventory: -all -json (or -view functions -all -json).\n", report.Truncated.Sites, report.Truncated.Edges, report.Truncated.Depth)
	if report.Inventory != nil {
		fmt.Fprintln(b, "\nSelected source inventory (unfiltered):")
		for _, r := range report.Inventory {
			fmt.Fprintf(b, "  %s.%s effects=%v uncertain=%t\n", r.Package, r.Function, r.Effects, r.Uncertain)
		}
	}
	return b.Flush()
}
