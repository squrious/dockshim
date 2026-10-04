package cli

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/execplan"
	"github.com/squrious/dockshim/internal/shim"
)

const exitUnknownAlias = 127

// discover loads the config, looking next to the shim first: IDEs may run it from any directory.
func (e *Env) discover(cwd, shimPath string) (*config.Project, error) {
	var starts []string
	if shimPath != "" {
		starts = append(starts, filepath.Dir(shimPath))
	}
	file, err := config.Discover(append(starts, cwd)...)
	if err != nil {
		return nil, err
	}
	return config.Load(file, config.LookupEnviron(e.Environ))
}

func execAlias(e *Env, proj *config.Project, name, cwd string, args []string) int {
	alias, ok := proj.Aliases[name]
	if !ok {
		e.errorf("alias %q is not defined in %s", name, proj.File)
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
		Stderr:  e.Stderr,
	}, execplan.Stdio{In: e.Stdin, Out: e.Stdout, Err: e.Stderr})
	if err != nil {
		e.errorf("%v", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

// warnOutdatedShims tells a human when the shims differ from the config. Alias mode otherwise
// prints only errors (ADR 11): IDEs and scripts read its output, and they have no terminal.
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
	e.errorf("warning: shims are out of date (%s), run `%s install`", strings.Join(parts, "; "), config.ToolName)
}
