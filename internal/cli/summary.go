package cli

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/envfilter"
	"github.com/squrious/dockshim/internal/pathmap"
)

// printSummary writes what matters of a project: service, user and path mappings of each alias,
// and the other settings only where they differ from the defaults. Paths are relative to the root.
// infer returns what inference gives for an alias, nil when that is unknown.
func printSummary(w io.Writer, p *config.Project, infer func(*config.ResolvedAlias) *inference) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	row := func(indent, key string, values ...string) {
		for i, v := range values {
			if i > 0 {
				key = ""
			}
			_, _ = fmt.Fprintf(tw, "%s%s\t%s\n", indent, key, v)
		}
	}

	row("", "config", p.File)
	if c := p.Compose; c != nil {
		var parts []string
		if c.ProjectName != "" {
			parts = append(parts, "project_name "+c.ProjectName)
		}
		for _, f := range c.Files {
			parts = append(parts, "file "+relTo(p.Root, f))
		}
		row("", "compose", parts...)
	}

	for _, name := range slices.Sorted(maps.Keys(p.Aliases)) {
		a := p.Aliases[name]
		_, _ = fmt.Fprintf(tw, "\n%s\n", name)
		row("  ", "service", a.Service)
		row("  ", "user", a.User)
		row("  ", "path_mapping", mappingLines(p.Root, a, infer)...)
		row("  ", "env", envLines(a)...)
		row("  ", "path_translation", translationLines(a.PathTranslation)...)
	}
	return tw.Flush()
}

func mappingLines(root string, a *config.ResolvedAlias, infer func(*config.ResolvedAlias) *inference) []string {
	if !a.InferPathMapping {
		if len(a.PathMapping) == 0 {
			return []string{"none"}
		}
		return pairLines(root, a.PathMapping)
	}
	inf := infer(a)
	switch {
	case inf == nil:
		return []string{"inferred from the container's bind mounts"}
	case inf.running:
		return inferredLines(root, "inferred from the running container", inf)
	default:
		return inferredLines(root, "inferred from the compose config, "+a.Service+" isn't running", inf)
	}
}

func inferredLines(root, head string, inf *inference) []string {
	lines := []string{head}
	if len(inf.mappings) == 0 {
		lines = append(lines, "no bind mount inside the project")
	}
	lines = append(lines, pairLines(root, inf.mappings)...)
	for _, src := range inf.outside {
		lines = append(lines, "not mapped, outside the project: "+src)
	}
	return lines
}

func pairLines(root string, m pathmap.Map) []string {
	var lines []string
	for _, p := range m {
		lines = append(lines, relTo(root, p.Host)+" → "+p.Container)
	}
	return lines
}

// envLines lists what the config adds to the built-in rules. Var values are left out: they may
// come from the environment.
func envLines(a *config.ResolvedAlias) []string {
	var lines []string
	add := func(label string, items []string) {
		if len(items) > 0 {
			lines = append(lines, label+" "+strings.Join(items, ", "))
		}
	}
	add("allow", a.Env.Allow)
	add("deny", without(a.Env.Deny, envfilter.DefaultDeny))
	add("deny_prefixes", without(a.Env.DenyPrefixes, envfilter.DefaultDenyPrefixes))
	add("vars", slices.Sorted(maps.Keys(a.Vars)))
	return lines
}

func translationLines(pt config.ResolvedPathTranslation) []string {
	if !pt.Enabled {
		return []string{"disabled"}
	}
	var lines []string
	if len(pt.Allow) > 0 {
		lines = append(lines, "allow "+strings.Join(pt.Allow, ", "))
	}
	return lines
}

func without(items, drop []string) []string {
	return slices.DeleteFunc(slices.Clone(items), func(s string) bool { return slices.Contains(drop, s) })
}

func relTo(root, p string) string {
	if rel, ok := pathmap.Rel(root, p); ok {
		return rel
	}
	return p
}
