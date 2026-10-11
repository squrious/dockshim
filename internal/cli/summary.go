package cli

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/envfilter"
	"github.com/squrious/dockshim/internal/pathmap"
	"github.com/squrious/dockshim/internal/style"
)

// printSummary writes what matters of a project: service, user and path mappings of each alias,
// and the other settings only where they differ from the defaults. Paths are relative to the root.
// infer returns what inference gives for an alias, nil when that is unknown.
func printSummary(w io.Writer, s style.Style, p *config.Project, infer func(*config.ResolvedAlias) *inference) error {
	t := &table{w: w, style: s}
	t.row("", "config", p.File)
	if c := p.Compose; c != nil {
		var parts []string
		if c.ProjectName != "" {
			parts = append(parts, "project_name "+c.ProjectName)
		}
		for _, f := range c.Files {
			parts = append(parts, "file "+relTo(p.Root, f))
		}
		t.row("", "compose", parts...)
	}
	t.flush()

	for _, name := range slices.Sorted(maps.Keys(p.Aliases)) {
		a := p.Aliases[name]
		t.printf("\n%s\n", s.Title(name))
		t.row("  ", "service", a.Service)
		t.row("  ", "user", a.User)
		t.row("  ", "path_mapping", mappingLines(s, p.Root, a, infer)...)
		t.row("  ", "env", envLines(a)...)
		t.row("  ", "env_forwarding", forwardingLines(a)...)
		t.row("  ", "path_translation", translationLines(s, a.PathTranslation)...)
		t.flush()
	}
	return t.err
}

// table aligns key/value rows. Unlike tabwriter, it pads keys before styling them, so that escape
// codes don't count as width.
type table struct {
	w     io.Writer
	style style.Style
	rows  [][3]string // indent, key, value
	err   error
}

// row adds a key with one row per value, the key only on the first one.
func (t *table) row(indent, key string, values ...string) {
	for i, v := range values {
		if i > 0 {
			key = ""
		}
		t.rows = append(t.rows, [3]string{indent, key, v})
	}
}

func (t *table) flush() {
	width := 0
	for _, r := range t.rows {
		width = max(width, utf8.RuneCountInString(r[0]+r[1]))
	}
	for _, r := range t.rows {
		pad := strings.Repeat(" ", width-utf8.RuneCountInString(r[0]+r[1])+2)
		t.printf("%s%s%s%s\n", r[0], t.style.Key(r[1]), pad, r[2])
	}
	t.rows = nil
}

func (t *table) printf(format string, args ...any) {
	if t.err == nil {
		_, t.err = fmt.Fprintf(t.w, format, args...)
	}
}

func mappingLines(s style.Style, root string, a *config.ResolvedAlias, infer func(*config.ResolvedAlias) *inference) []string {
	if !a.InferPathMapping {
		if len(a.PathMapping) == 0 {
			return []string{s.Faint("none")}
		}
		return pairLines(root, a.PathMapping)
	}
	inf := infer(a)
	switch {
	case inf == nil:
		return []string{s.Faint("inferred from the container's bind mounts")}
	case inf.running:
		return inferredLines(s, root, "inferred from the running container", inf)
	default:
		return inferredLines(s, root, "inferred from the compose config, "+a.Service+" isn't running", inf)
	}
}

func inferredLines(s style.Style, root, head string, inf *inference) []string {
	lines := []string{s.Faint(head)}
	if len(inf.mappings) == 0 {
		lines = append(lines, s.Faint("no bind mount inside the project"))
	}
	lines = append(lines, pairLines(root, inf.mappings)...)
	for _, src := range inf.outside {
		lines = append(lines, s.Faint("not mapped, outside the project:")+" "+src)
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

// envLines lists the names of the variables the config sets. Values are left out: they may come
// from the environment.
func envLines(a *config.ResolvedAlias) []string {
	if len(a.Env) == 0 {
		return nil
	}
	return []string{strings.Join(slices.Sorted(maps.Keys(a.Env)), ", ")}
}

// forwardingLines lists what the config adds to the built-in rules.
func forwardingLines(a *config.ResolvedAlias) []string {
	var lines []string
	add := func(label string, items []string) {
		if len(items) > 0 {
			lines = append(lines, label+" "+strings.Join(items, ", "))
		}
	}
	add("allow", a.EnvForwarding.Allow)
	add("deny", without(a.EnvForwarding.Deny, envfilter.DefaultDeny))
	return lines
}

func translationLines(s style.Style, pt config.ResolvedPathTranslation) []string {
	if !pt.Enabled {
		return []string{s.Faint("disabled")}
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
