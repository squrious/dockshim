package cli

import (
	"os"
	"path/filepath"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/execplan"
	"github.com/squrious/dockshim/internal/shim"
)

const exitUnknownAlias = 127

func runAlias(argv0 string, args []string, e *Env) int {
	cwd, err := e.cwd()
	if err != nil {
		e.errorf("%v", err)
		return 1
	}

	// Look next to the shim first: IDEs may run it from any directory.
	shimPath := shim.Locate(argv0, e.Executable)
	var starts []string
	if shimPath != "" {
		starts = append(starts, filepath.Dir(shimPath))
	}
	starts = append(starts, cwd)

	file, err := config.Discover(starts...)
	if err != nil {
		e.errorf("%v", err)
		return 1
	}
	proj, err := config.Load(file)
	if err != nil {
		e.errorf("%v", err)
		return 1
	}
	return execAlias(e, proj, filepath.Base(argv0), shimPath, cwd, args)
}

func execAlias(e *Env, proj *config.Project, name, shimPath, cwd string, args []string) int {
	alias, ok := proj.Aliases[name]
	if !ok {
		return unknownAlias(e, proj, name, shimPath)
	}
	if shimPath != "" && !sameDir(filepath.Dir(shimPath), proj.BinDir) {
		e.errorf("warning: %s is outside bin_dir %s, run `dockshim install` and update PATH", shimPath, proj.BinDir)
	}

	plan, err := execplan.Build(execplan.Input{
		Project: proj,
		Alias:   alias,
		Runner:  e.Runner,
		Cwd:     cwd,
		Environ: e.Environ,
		Args:    append([]string{name}, args...),
		TTY:     e.TTY,
	})
	if err != nil {
		e.errorf("%v", err)
		return 1
	}
	code, err := execplan.Execute(plan, e.Runner, execplan.Stdio{In: e.Stdin, Out: e.Stdout, Err: e.Stderr})
	if err != nil {
		e.errorf("%v", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

func unknownAlias(e *Env, proj *config.Project, name, shimPath string) int {
	e.errorf("alias %q is not defined in %s", name, proj.File)
	if shimPath == "" {
		return exitUnknownAlias
	}
	// Only shims in this project's bin_dir are candidates for removal.
	if !sameDir(filepath.Dir(shimPath), proj.BinDir) {
		e.errorf("%s is a stale shim outside bin_dir %s, remove it manually", shimPath, proj.BinDir)
		return exitUnknownAlias
	}
	if !e.Interactive {
		e.errorf("%s is a stale shim, run `dockshim install` to remove it", shimPath)
		return exitUnknownAlias
	}
	remove, err := e.Prompter.Confirm("Remove stale shim " + shimPath + "?")
	if err != nil {
		e.errorf("%v", err)
		return exitUnknownAlias
	}
	if remove {
		if err := os.Remove(shimPath); err != nil {
			e.errorf("%v", err)
		} else {
			e.errorf("removed %s", shimPath)
		}
	}
	return exitUnknownAlias
}

func sameDir(a, b string) bool {
	fa, errA := os.Stat(a)
	fb, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(fa, fb)
}
