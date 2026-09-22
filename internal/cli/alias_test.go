package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/squrious/dockshim/internal/shim"
)

type answer bool

func (a answer) Confirm(string) (bool, error) { return bool(a), nil }

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func staleSetup(t *testing.T) (shimPath string, e *Env, stderr *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	must(t, os.WriteFile(filepath.Join(root, ".dockshim.yaml"), []byte("aliases: {php: {service: tools}}\n"), 0o644))
	bin := filepath.Join(root, ".dockshim", "bin")
	if _, err := shim.Install(bin, "dev", []string{"php", "gone"}); err != nil {
		t.Fatal(err)
	}
	stderr = &bytes.Buffer{}
	e = &Env{
		Stderr: stderr,
		Getwd:  func() (string, error) { return "/", nil },
	}
	return filepath.Join(bin, "gone"), e, stderr
}

// runShim runs dockshim the way the shim at p does.
func runShim(p string, e *Env) int {
	return Main([]string{"dockshim", "run", "--shim", p, filepath.Base(p)}, e)
}

func TestStaleShim(t *testing.T) {
	t.Run("interactive, accepted", func(t *testing.T) {
		p, e, stderr := staleSetup(t)
		e.Interactive, e.Prompter = true, answer(true)
		if code := runShim(p, e); code != exitUnknownAlias {
			t.Fatalf("code = %d", code)
		}
		if _, err := os.Lstat(p); !os.IsNotExist(err) {
			t.Fatal("shim should be removed")
		}
		if !strings.Contains(stderr.String(), `alias "gone" is not defined`) || !strings.Contains(stderr.String(), "removed "+p) {
			t.Fatalf("stderr = %s", stderr)
		}
	})

	t.Run("interactive, declined", func(t *testing.T) {
		p, e, _ := staleSetup(t)
		e.Interactive, e.Prompter = true, answer(false)
		runShim(p, e)
		if _, err := os.Lstat(p); err != nil {
			t.Fatal("shim should be kept")
		}
	})

	t.Run("non interactive", func(t *testing.T) {
		p, e, stderr := staleSetup(t)
		e.Prompter = answer(true)
		if code := runShim(p, e); code != exitUnknownAlias {
			t.Fatalf("code = %d", code)
		}
		if _, err := os.Lstat(p); err != nil {
			t.Fatal("shim should be kept")
		}
		if !strings.Contains(stderr.String(), "stale shim, run `dockshim install`") {
			t.Fatalf("stderr = %s", stderr)
		}
	})

	t.Run("a script that is not a shim is never removed", func(t *testing.T) {
		p, e, stderr := staleSetup(t)
		must(t, os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755))
		e.Interactive, e.Prompter = true, answer(true)
		if code := runShim(p, e); code != exitUnknownAlias {
			t.Fatalf("code = %d", code)
		}
		if _, err := os.Lstat(p); err != nil {
			t.Fatal("foreign script should be kept")
		}
		if strings.Contains(stderr.String(), "stale shim") {
			t.Fatalf("stderr = %s", stderr)
		}
	})

	t.Run("shim outside bin_dir is never removed", func(t *testing.T) {
		p, e, stderr := staleSetup(t)
		other := filepath.Join(t.TempDir(), "gone")
		content, err := os.ReadFile(p)
		must(t, err)
		must(t, os.WriteFile(other, content, 0o755))
		// Config is still found through cwd.
		root := filepath.Dir(filepath.Dir(filepath.Dir(p)))
		e.Getwd = func() (string, error) { return root, nil }
		e.Interactive, e.Prompter = true, answer(true)
		runShim(other, e)
		if _, err := os.Lstat(other); err != nil {
			t.Fatal("foreign shim should be kept")
		}
		if !strings.Contains(stderr.String(), "stale shim outside bin_dir") {
			t.Fatalf("stderr = %s", stderr)
		}
	})
}
