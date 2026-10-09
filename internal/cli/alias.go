package cli

import (
	"maps"
	"slices"
	"strings"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/execplan"
	"github.com/squrious/dockshim/internal/shim"
)

const exitUnknownAlias = 127

// execAlias runs an alias of proj for a shim of its project: when the alias is gone, the shim is one
// install would remove.
func execAlias(e *Env, proj *config.Project, name, cwd string, args []string) int {
	alias, ok := proj.Aliases[name]
	if !ok {
		e.errorf("alias %q is not defined in %s: run `%s install` to remove its shim", name, proj.File, config.ToolName)
		return exitUnknownAlias
	}
	code, err := execplan.Run(execplan.Input{
		Project: proj,
		Alias:   alias,
		Runner:  e.Runner,
		Cwd:     cwd,
		Environ: e.Environ,
		Args:    append([]string{name}, args...),
		TTY:     e.TTY,
		Warn:    e.warnf,
	}, execplan.Stdio{In: e.Stdin, Out: e.Stdout, Err: e.Stderr})
	if err != nil {
		e.errorf("%v", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

// warnOutdatedShims tells when the shims differ from the config.
func warnOutdatedShims(e *Env, proj *config.Project) {
	st, err := shim.Check(proj.BinDir(), slices.Sorted(maps.Keys(proj.Aliases)))
	if err != nil || st.UpToDate() {
		return
	}
	var parts []string
	for _, g := range []struct {
		label string
		names []string
	}{{"missing", st.Missing}, {"outdated", st.Outdated}, {"stale", st.Stale}} {
		if len(g.names) > 0 {
			parts = append(parts, g.label+": "+strings.Join(g.names, ", "))
		}
	}
	e.warnf("shims are out of date (%s), run `%s install`", strings.Join(parts, "; "), config.ToolName)
}
