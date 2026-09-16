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

func staleSetup(t *testing.T) (shimPath string, e *Env, stderr *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	exe := filepath.Join(t.TempDir(), "dockshim")
	os.WriteFile(exe, nil, 0o755)
	os.WriteFile(filepath.Join(root, ".dockshim.yaml"), []byte("aliases: {php: {service: tools}}\n"), 0o644)
	bin := filepath.Join(root, ".dockshim", "bin")
	if _, err := shim.Install(bin, exe, []string{"php", "gone"}); err != nil {
		t.Fatal(err)
	}
	stderr = &bytes.Buffer{}
	e = &Env{
		Stderr:     stderr,
		Getwd:      func() (string, error) { return "/", nil },
		Executable: exe,
	}
	return filepath.Join(bin, "gone"), e, stderr
}

func TestStaleShim(t *testing.T) {
	t.Run("interactive, accepted", func(t *testing.T) {
		p, e, stderr := staleSetup(t)
		e.Interactive, e.Prompter = true, answer(true)
		if code := Main([]string{p}, e); code != exitUnknownAlias {
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
		Main([]string{p}, e)
		if _, err := os.Lstat(p); err != nil {
			t.Fatal("shim should be kept")
		}
	})

	t.Run("non interactive", func(t *testing.T) {
		p, e, stderr := staleSetup(t)
		e.Prompter = answer(true)
		if code := Main([]string{p}, e); code != exitUnknownAlias {
			t.Fatalf("code = %d", code)
		}
		if _, err := os.Lstat(p); err != nil {
			t.Fatal("shim should be kept")
		}
		if !strings.Contains(stderr.String(), "stale shim, run `dockshim install`") {
			t.Fatalf("stderr = %s", stderr)
		}
	})

	t.Run("shim outside bin_dir is never removed", func(t *testing.T) {
		p, e, _ := staleSetup(t)
		other := filepath.Join(t.TempDir(), "gone")
		os.Symlink(e.Executable, other)
		// Config is still found through cwd.
		root := filepath.Dir(filepath.Dir(filepath.Dir(p)))
		e.Getwd = func() (string, error) { return root, nil }
		e.Interactive, e.Prompter = true, answer(true)
		Main([]string{other}, e)
		if _, err := os.Lstat(other); err != nil {
			t.Fatal("foreign shim should be kept")
		}
	})
}
