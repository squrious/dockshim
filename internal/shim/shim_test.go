package shim

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/squrious/dockshim/internal/config"
)

func setup(t *testing.T) (dir, exe string) {
	t.Helper()
	base := t.TempDir()
	exe = filepath.Join(base, "dockshim")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(base, "bin"), exe
}

func shims(mode string, names ...string) []Shim {
	out := make([]Shim, len(names))
	for i, n := range names {
		out[i] = Shim{Name: n, Mode: mode}
	}
	return out
}

// assertShim checks that path is an entry point of the given mode for alias name.
func assertShim(t *testing.T, path, exe, mode, name string) {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !IsShim(path, exe) {
		t.Fatalf("%s is not recognized as a shim", path)
	}
	if mode == config.ShimSymlink {
		if target, _ := os.Readlink(path); target != exe {
			t.Fatalf("%s -> %q, want %q", path, target, exe)
		}
		return
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s should be an executable regular file, mode %v", path, fi.Mode())
	}
	for _, want := range []string{"#!/bin/sh", marker, "exec '" + exe + "' run --shim \"$0\" " + name + " \"$@\""} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("wrapper %s missing %q:\n%s", path, want, content)
		}
	}
}

func TestInstall(t *testing.T) {
	for _, mode := range []string{config.ShimSymlink, config.ShimWrapper} {
		t.Run(mode, func(t *testing.T) {
			dir, exe := setup(t)
			res, err := Install(dir, exe, shims(mode, "node", "php"))
			if err != nil || !slices.Equal(res.Created, []string{"node", "php"}) {
				t.Fatalf("res=%+v err=%v", res, err)
			}
			assertShim(t, filepath.Join(dir, "php"), exe, mode, "php")

			// Idempotent.
			res, _ = Install(dir, exe, shims(mode, "node", "php"))
			if len(res.Created)+len(res.Removed) != 0 {
				t.Fatalf("second install changed things: %+v", res)
			}

			// Foreign files are left alone, stale shims removed, moved binaries re-linked.
			os.WriteFile(filepath.Join(dir, "composer"), nil, 0o755)
			os.Symlink("/old/place/dockshim", filepath.Join(dir, "old"))
			os.Symlink("/usr/bin/env", filepath.Join(dir, "other"))
			os.Remove(filepath.Join(dir, "php"))
			os.Symlink("/old/place/dockshim", filepath.Join(dir, "php"))

			res, err = Install(dir, exe, shims(mode, "composer", "php"))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(res.Created, []string{"php"}) || !slices.Equal(res.Removed, []string{"node", "old"}) || !slices.Equal(res.Skipped, []string{"composer"}) {
				t.Fatalf("res=%+v", res)
			}
			assertShim(t, filepath.Join(dir, "php"), exe, mode, "php")
			if _, err := os.Lstat(filepath.Join(dir, "other")); err != nil {
				t.Fatal("unrelated symlink removed")
			}
		})
	}
}

func TestInstallSwitchesMode(t *testing.T) {
	dir, exe := setup(t)
	for _, mode := range []string{config.ShimSymlink, config.ShimWrapper, config.ShimSymlink} {
		res, err := Install(dir, exe, shims(mode, "php"))
		if err != nil || !slices.Equal(res.Created, []string{"php"}) || len(res.Skipped) != 0 {
			t.Fatalf("%s: res=%+v err=%v", mode, res, err)
		}
		assertShim(t, filepath.Join(dir, "php"), exe, mode, "php")
	}
}

func TestInstallMixedModes(t *testing.T) {
	dir, exe := setup(t)
	if _, err := Install(dir, exe, []Shim{{Name: "php", Mode: config.ShimWrapper}, {Name: "node", Mode: config.ShimSymlink}}); err != nil {
		t.Fatal(err)
	}
	assertShim(t, filepath.Join(dir, "php"), exe, config.ShimWrapper, "php")
	assertShim(t, filepath.Join(dir, "node"), exe, config.ShimSymlink, "node")
}

func TestIsShim(t *testing.T) {
	dir, exe := setup(t)
	os.MkdirAll(dir, 0o755)
	foreign := filepath.Join(dir, "foreign")
	os.WriteFile(foreign, []byte("#!/bin/sh\nexec something\n"), 0o755)
	if IsShim(foreign, exe) || IsShim(filepath.Join(dir, "missing"), exe) || IsShim(dir, exe) {
		t.Fatal("false positive")
	}
}

func TestLocate(t *testing.T) {
	for _, mode := range []string{config.ShimSymlink, config.ShimWrapper} {
		t.Run(mode, func(t *testing.T) {
			dir, exe := setup(t)
			Install(dir, exe, shims(mode, "php"))
			php := filepath.Join(dir, "php")

			if got := Locate(php, exe); got != php {
				t.Fatalf("absolute: %q", got)
			}
			t.Setenv("PATH", dir)
			if got := Locate("php", exe); got != php {
				t.Fatalf("from PATH: %q", got)
			}
			if got := Locate("missing", exe); got != "" {
				t.Fatalf("missing: %q", got)
			}
			if got := Locate(exe, exe); got != "" {
				t.Fatalf("binary itself is not a shim: %q", got)
			}
		})
	}
}
