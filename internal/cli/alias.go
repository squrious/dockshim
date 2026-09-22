package cli

import (
	"os"
	"path/filepath"

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

func execAlias(e *Env, proj *config.Project, name, shimPath, cwd string, args []string) int {
	alias, ok := proj.Aliases[name]
	if !ok {
		return unknownAlias(e, proj, name, shimPath)
	}
	if shimPath != "" && !sameDir(filepath.Dir(shimPath), proj.BinDir) {
		e.errorf("warning: %s is outside bin_dir %s, run `dockshim install` and update PATH", shimPath, proj.BinDir)
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

func unknownAlias(e *Env, proj *config.Project, name, shimPath string) int {
	e.errorf("alias %q is not defined in %s", name, proj.File)
	// --shim is free input: only our own scripts are reported as stale.
	if shimPath == "" || !shim.IsShim(shimPath) {
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
