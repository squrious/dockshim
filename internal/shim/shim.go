// Package shim manages the alias entry points: symlinks to the dockshim executable, or wrapper scripts calling it.
package shim

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/squrious/dockshim/internal/config"
)

// marker identifies a wrapper script as ours.
const marker = "# " + config.ToolName + " shim"

// Shim is one alias entry point to create.
type Shim struct {
	Name string
	Mode string
}

// IsShim reports whether path is one of our entry points: a symlink to exe or to any file
// named dockshim (a moved or removed binary), or a wrapper script carrying our marker.
func IsShim(path, exe string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		return fi.Mode().IsRegular() && isWrapper(path)
	}
	target, err := os.Readlink(path)
	if err != nil {
		return false
	}
	if filepath.Base(target) == config.ToolName {
		return true
	}
	a, errA := os.Stat(path)
	b, errB := os.Stat(exe)
	return errA == nil && errB == nil && os.SameFile(a, b)
}

func isWrapper(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	head := make([]byte, 256)
	n, _ := f.Read(head)
	return strings.Contains(string(head[:n]), marker)
}

// Locate returns the shim path the process was started from, or "" when unknown.
// argv0 holds a bare name when the shell found it through PATH.
func Locate(argv0, exe string) string {
	p := argv0
	if !strings.ContainsRune(argv0, filepath.Separator) {
		var err error
		if p, err = exec.LookPath(argv0); err != nil {
			return ""
		}
	}
	p, err := filepath.Abs(p)
	if err != nil || !IsShim(p, exe) {
		return ""
	}
	return p
}

// wrapperScript calls dockshim the way a symlink would, passing its own path so that
// discovery is anchored at the shim, exactly as in argv[0] dispatch.
func wrapperScript(exe, alias string) []byte {
	return fmt.Appendf(nil, `#!/bin/sh
%s for %q, created by `+"`"+config.ToolName+" install"+"`"+`.
exec %s run --shim "$0" %s "$@"
`, marker, alias, shellQuote(exe), alias)
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type Result struct {
	Created []string
	Removed []string
	Skipped []string // existing files that are not shims
}

// Install makes dir contain exactly one entry point per shim, in its configured mode.
func Install(dir, exe string, shims []Shim) (Result, error) {
	var res Result
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, err
	}

	names := make([]string, len(shims))
	for i, s := range shims {
		names[i] = s.Name
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return res, err
	}
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		if !slices.Contains(names, e.Name()) && IsShim(p, exe) {
			if err := os.Remove(p); err != nil {
				return res, err
			}
			res.Removed = append(res.Removed, e.Name())
		}
	}

	for _, s := range shims {
		p := filepath.Join(dir, s.Name)
		switch _, err := os.Lstat(p); {
		case err == nil && upToDate(p, exe, s):
			continue
		case err == nil && !IsShim(p, exe):
			res.Skipped = append(res.Skipped, s.Name)
			continue
		}
		if err := create(p, exe, s); err != nil {
			return res, fmt.Errorf("installing %s: %w", s.Name, err)
		}
		res.Created = append(res.Created, s.Name)
	}
	return res, nil
}

func upToDate(path, exe string, s Shim) bool {
	if s.Mode == config.ShimWrapper {
		content, err := os.ReadFile(path)
		return err == nil && string(content) == string(wrapperScript(exe, s.Name))
	}
	target, err := os.Readlink(path)
	return err == nil && target == exe
}

// create writes the entry point through a temporary file, so an in-use shim is replaced atomically.
func create(path, exe string, s Shim) error {
	tmp := path + ".dockshim-tmp"
	_ = os.Remove(tmp)
	var err error
	if s.Mode == config.ShimWrapper {
		err = os.WriteFile(tmp, wrapperScript(exe, s.Name), 0o755)
	} else {
		err = os.Symlink(exe, tmp)
	}
	if err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
