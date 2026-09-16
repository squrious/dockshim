// Package shim manages alias symlinks pointing to the dockshim executable.
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

// IsShim reports whether path is a symlink to exe, or to any file named dockshim (e.g. a moved or removed binary).
func IsShim(path, exe string) bool {
	fi, err := os.Lstat(path)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return false
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

type Result struct {
	Created []string
	Removed []string
	Skipped []string // existing files that are not shims
}

// Install makes dir contain exactly one shim per name, pointing to exe.
func Install(dir, exe string, names []string) (Result, error) {
	var res Result
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return res, err
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

	for _, name := range names {
		p := filepath.Join(dir, name)
		if target, err := os.Readlink(p); err == nil && target == exe {
			continue
		}
		if _, err := os.Lstat(p); err == nil && !IsShim(p, exe) {
			res.Skipped = append(res.Skipped, name)
			continue
		}
		tmp := p + ".dockshim-tmp"
		_ = os.Remove(tmp)
		if err := os.Symlink(exe, tmp); err != nil {
			return res, err
		}
		if err := os.Rename(tmp, p); err != nil {
			_ = os.Remove(tmp)
			return res, fmt.Errorf("installing %s: %w", name, err)
		}
		res.Created = append(res.Created, name)
	}
	return res, nil
}
