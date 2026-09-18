// Package hostpath knows what host paths mean on the machine dockshim runs on:
// which of them may be copied into a container, and how to read the Windows paths
// a Windows tool hands to a command running in WSL.
package hostpath

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/squrious/dockshim/internal/pathmap"
)

// unixTempDirs are always allowed: they are where tools put the throwaway files
// path translation exists for.
var unixTempDirs = []string{"/tmp", "/var/tmp"}

// Env is what a Resolver needs from the machine. A nil Mountinfo means no Windows drives.
type Env struct {
	Lookup    func(string) (string, bool)
	Mountinfo io.Reader
	TempDirs  []string
}

// Options mirrors the resolved path_translation configuration.
type Options struct {
	Allow          []string
	FollowSymlinks bool
}

// Resolver answers what a host path means on this machine, for one alias's path_translation.
type Resolver struct {
	distro string
	drives map[string]string // lowercase drive letter -> mount point
	roots  []string          // absolute, cleaned, symlinks resolved
	follow bool
}

// Detect builds a Resolver from the running machine.
func Detect(opts Options) *Resolver {
	env := Env{Lookup: os.LookupEnv, TempDirs: append([]string{os.TempDir()}, unixTempDirs...)}
	if f, err := os.Open("/proc/self/mountinfo"); err == nil {
		defer func() { _ = f.Close() }()
		env.Mountinfo = f
	}
	return New(env, opts)
}

// New builds a Resolver from what env reports about the machine. Allowed directories that are
// Windows paths are converted, relative ones ignored (validation rejects them).
func New(env Env, opts Options) *Resolver {
	r := &Resolver{drives: parseDrives(env.Mountinfo), follow: opts.FollowSymlinks}
	if env.Lookup != nil {
		r.distro, _ = env.Lookup("WSL_DISTRO_NAME")
	}
	for _, p := range slices.Concat(env.TempDirs, opts.Allow) {
		if p == "" {
			continue
		}
		if converted, ok := r.ToLinux(p); ok {
			p = converted
		}
		if !filepath.IsAbs(p) {
			continue
		}
		p = Real(filepath.Clean(p))
		if !slices.Contains(r.roots, p) {
			r.roots = append(r.roots, p)
		}
	}
	return r
}

// IsWindows reports whether value is written as a Windows path. The test is deliberately
// syntactic and strict: an argument that merely contains backslashes, such as a regular
// expression over namespaced class names, must not be mistaken for one.
func IsWindows(value string) bool {
	if _, ok := cutUNC(value); ok {
		return true
	}
	return len(value) >= 3 && isDriveLetter(value[0]) && value[1] == ':' && (value[2] == '\\' || value[2] == '/')
}

// cutUNC strips a UNC prefix and returns the rest, always slash separated.
// Windows tools spell the same path either way, and an IDE may well use both at once.
// The backslash form is unambiguous; the forward slash form is an ordinary absolute path
// until a host name says otherwise, so only the ones naming a distribution count.
func cutUNC(value string) (string, bool) {
	if rest, ok := strings.CutPrefix(value, `\\`); ok {
		return slashed(rest), true
	}
	rest, ok := strings.CutPrefix(value, "//")
	if !ok {
		return "", false
	}
	host, _, _ := strings.Cut(rest, "/")
	return rest, isDistroHost(host)
}

// isDistroHost reports whether host is a name Windows uses to reach a distribution.
func isDistroHost(host string) bool {
	return strings.EqualFold(host, "wsl.localhost") || strings.EqualFold(host, "wsl$")
}

// ToLinux converts a Windows path to the path that reaches the same file from this distribution.
// It fails for a path this distribution cannot reach: an unmounted drive, another distribution,
// or a network share.
func (r *Resolver) ToLinux(value string) (string, bool) {
	if rest, ok := cutUNC(value); ok {
		return r.fromUNC(rest)
	}
	if !IsWindows(value) {
		return "", false
	}
	mount, ok := r.drives[strings.ToLower(value[:1])]
	if !ok {
		return "", false
	}
	return filepath.Join(mount, filepath.FromSlash(slashed(value[2:]))), true
}

// fromUNC handles \\wsl.localhost\<distro>\... and its \\wsl$\<distro>\... alias,
// given the slash separated remainder.
func (r *Resolver) fromUNC(rest string) (string, bool) {
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 || r.distro == "" || !isDistroHost(parts[0]) {
		return "", false
	}
	if !strings.EqualFold(parts[1], r.distro) {
		return "", false
	}
	if len(parts) < 3 {
		return "/", true
	}
	return filepath.Join("/", parts[2]), true
}

// Allowed reports whether a host file may be copied into the container. abs is the argument
// resolved to an absolute path, real is that path with its symlinks resolved.
func (r *Resolver) Allowed(abs, real string) bool {
	return r.inRoot(real) || (r.follow && r.inRoot(abs))
}

func (r *Resolver) inRoot(p string) bool {
	return slices.ContainsFunc(r.roots, func(root string) bool { return pathmap.Within(root, p) }) ||
		r.isWindowsTemp(p)
}

// isWindowsTemp matches the default Windows temporary directories, seen through a drive mount.
// They are recognised by shape rather than read from %TEMP%, which would cost a subprocess.
func (r *Resolver) isWindowsTemp(p string) bool {
	for _, mount := range r.drives {
		rel, ok := pathmap.Rel(mount, p)
		if !ok {
			continue
		}
		seg := strings.Split(filepath.ToSlash(rel), "/")
		switch {
		case len(seg) >= 2 && strings.EqualFold(seg[0], "windows") && strings.EqualFold(seg[1], "temp"):
			return true
		case len(seg) >= 5 && strings.EqualFold(seg[0], "users") && strings.EqualFold(seg[2], "appdata") &&
			strings.EqualFold(seg[3], "local") && strings.EqualFold(seg[4], "temp"):
			return true
		}
	}
	return false
}

func isDriveLetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

func slashed(s string) string { return strings.ReplaceAll(s, `\`, "/") }

// Real resolves the symlinks of p, also when its last element does not exist.
// When that fails, p is returned as is.
func Real(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	if r, err := filepath.EvalSymlinks(filepath.Dir(p)); err == nil {
		return filepath.Join(r, filepath.Base(p))
	}
	return p
}
