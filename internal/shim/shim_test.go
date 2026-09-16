package shim

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
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

func TestInstall(t *testing.T) {
	dir, exe := setup(t)
	res, err := Install(dir, exe, []string{"node", "php"})
	if err != nil || !slices.Equal(res.Created, []string{"node", "php"}) {
		t.Fatalf("res=%+v err=%v", res, err)
	}

	// Idempotent.
	res, _ = Install(dir, exe, []string{"node", "php"})
	if len(res.Created)+len(res.Removed) != 0 {
		t.Fatalf("second install changed things: %+v", res)
	}

	// Foreign files are left alone, stale shims (even dangling ones) removed, moved binaries re-linked.
	os.WriteFile(filepath.Join(dir, "composer"), nil, 0o755)
	os.Symlink("/old/place/dockshim", filepath.Join(dir, "old"))
	os.Symlink("/usr/bin/env", filepath.Join(dir, "other"))
	os.Remove(filepath.Join(dir, "php"))
	os.Symlink("/old/place/dockshim", filepath.Join(dir, "php"))

	res, err = Install(dir, exe, []string{"composer", "php"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Created, []string{"php"}) || !slices.Equal(res.Removed, []string{"node", "old"}) || !slices.Equal(res.Skipped, []string{"composer"}) {
		t.Fatalf("res=%+v", res)
	}
	if target, _ := os.Readlink(filepath.Join(dir, "php")); target != exe {
		t.Fatalf("php -> %s", target)
	}
	if _, err := os.Lstat(filepath.Join(dir, "other")); err != nil {
		t.Fatal("unrelated symlink removed")
	}
}

func TestLocate(t *testing.T) {
	dir, exe := setup(t)
	Install(dir, exe, []string{"php"})
	shim := filepath.Join(dir, "php")

	if got := Locate(shim, exe); got != shim {
		t.Fatalf("absolute: %q", got)
	}
	t.Setenv("PATH", dir)
	if got := Locate("php", exe); got != shim {
		t.Fatalf("from PATH: %q", got)
	}
	if got := Locate("missing", exe); got != "" {
		t.Fatalf("missing: %q", got)
	}
	if got := Locate(exe, exe); got != "" {
		t.Fatalf("binary itself is not a shim: %q", got)
	}
}
