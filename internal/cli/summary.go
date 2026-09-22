package cli

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/envfilter"
	"github.com/squrious/dockshim/internal/pathmap"
)

// printSummary writes what matters of a project: service, user and path mappings of each alias,
// and the other settings only where they differ from the defaults. Paths are relative to the root.
// outside returns the bind mounts an inferred mapping skips, when that is known.
func printSummary(w io.Writer, p *config.Project, outside func(*config.ResolvedAlias) []string) error {
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
	row("", "bin_dir", relTo(p.Root, p.BinDir))
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
		row("  ", "path_mapping", mappingLines(p.Root, a, outside)...)
		row("  ", "env", envLines(a)...)
		row("  ", "path_translation", translationLines(a.PathTranslation)...)
	}
	return tw.Flush()
}

func mappingLines(root string, a *config.ResolvedAlias, outside func(*config.ResolvedAlias) []string) []string {
	if a.InferPathMapping {
		lines := []string{"inferred from the container's bind mounts"}
		for _, src := range outside(a) {
			lines = append(lines, "not mapped, outside the project: "+src)
		}
		return lines
	}
	if len(a.PathMapping) == 0 {
		return []string{"none"}
	}
	var lines []string
	for _, m := range a.PathMapping {
		lines = append(lines, relTo(root, m.Host)+" → "+m.Container)
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
	if pt.FollowSymlinks {
		lines = append(lines, "follow_symlinks")
	}
	if pt.MaxCopyMB != config.DefaultMaxCopyMB {
		lines = append(lines, "max_copy_mb "+strconv.Itoa(pt.MaxCopyMB))
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
