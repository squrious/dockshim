package shim

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// assertShim checks that path is an entry point for alias, written by version.
func assertShim(t *testing.T, path, version, alias string) {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("%s should be an executable regular file, mode %v", path, fi.Mode())
	}
	if !IsShim(path) {
		t.Fatalf("%s is not recognized as a shim", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"#!/bin/sh\n",
		"do not edit",
		"# dockshim shim format 1, alias \"" + alias + "\"\n",
		"# created by dockshim " + version + "\n",
		"exec dockshim run --shim \"$0\" '" + alias + "' \"$@\"\n",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("shim %s missing %q:\n%s", path, want, content)
		}
	}
}

func TestInstall(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bin")
	res, err := Install(dir, "1.0.0", []string{"node", "php"})
	if err != nil || !slices.Equal(res.Created, []string{"node", "php"}) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	assertShim(t, filepath.Join(dir, "php"), "1.0.0", "php")

	// Idempotent, and another dockshim version alone rewrites nothing.
	res, err = Install(dir, "1.1.0", []string{"node", "php"})
	if err != nil || len(res.Created)+len(res.Removed) != 0 {
		t.Fatalf("second install changed things: %+v", res)
	}
	assertShim(t, filepath.Join(dir, "php"), "1.0.0", "php")

	// A shim that lost its executable bit is repaired.
	must(t, os.Chmod(filepath.Join(dir, "node"), 0o644))
	res, err = Install(dir, "1.1.0", []string{"node", "php"})
	if err != nil || !slices.Equal(res.Created, []string{"node"}) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	assertShim(t, filepath.Join(dir, "node"), "1.1.0", "node")

	// Foreign files and symlinks are left alone, stale shims removed, changed shims rewritten.
	must(t, os.WriteFile(filepath.Join(dir, "composer"), nil, 0o755))
	must(t, os.Symlink("/old/place/dockshim", filepath.Join(dir, "old")))
	php := filepath.Join(dir, "php")
	content, err := os.ReadFile(php)
	must(t, err)
	must(t, os.WriteFile(php, []byte(strings.Replace(string(content), "'php'", "'node'", 1)), 0o755))

	res, err = Install(dir, "1.1.0", []string{"composer", "old", "php"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(res.Created, []string{"php"}) || !slices.Equal(res.Removed, []string{"node"}) || !slices.Equal(res.Skipped, []string{"composer", "old"}) {
		t.Fatalf("res=%+v", res)
	}
	assertShim(t, php, "1.1.0", "php")
	if _, err := os.Lstat(filepath.Join(dir, "old")); err != nil {
		t.Fatal("symlink removed")
	}
}

func TestIsShim(t *testing.T) {
	dir := t.TempDir()
	foreign := filepath.Join(dir, "foreign")
	must(t, os.WriteFile(foreign, []byte("#!/bin/sh\nexec something\n"), 0o755))
	link := filepath.Join(dir, "link")
	must(t, os.Symlink("/usr/bin/dockshim", link))
	if IsShim(foreign) || IsShim(link) || IsShim(filepath.Join(dir, "missing")) || IsShim(dir) {
		t.Fatal("false positive")
	}
}
