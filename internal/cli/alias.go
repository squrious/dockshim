package cli

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/execplan"
	"github.com/squrious/dockshim/internal/shim"
)

const exitUnknownAlias = 127

// discover loads the config of the shim's project, or of cwd without a shim. A shim never falls
// back to cwd: IDEs may run it from any directory, and `dockshim run` already follows cwd.
func (e *Env) discover(cwd, shimPath string) (*config.Project, error) {
	start := cwd
	if shimPath != "" {
		start = filepath.Dir(shimPath)
	}
	file, err := config.Discover(start)
	if err != nil {
		return nil, err
	}
	return config.Load(file, config.LookupEnviron(e.Environ))
}

// installed is true when the alias runs through a shim of .dockshim/bin.
func execAlias(e *Env, proj *config.Project, name, cwd string, args []string, installed bool) int {
	alias, ok := proj.Aliases[name]
	if !ok {
		hint := ""
		if installed {
			hint = fmt.Sprintf(": run `%s install` to remove its shim", config.ToolName)
		}
		e.errorf("alias %q is not defined in %s%s", name, proj.File, hint)
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
