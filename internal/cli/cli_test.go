package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/squrious/dockshim/internal/style"
)

// Config values are interpolated from Env.Environ, not from the process environment.
func TestInterpolatesFromEnviron(t *testing.T) {
	root := t.TempDir()
	must(t, os.WriteFile(filepath.Join(root, ".dockshim.yaml"), []byte("aliases:\n  php:\n    service: ${DOCKSHIM_TEST_SVC}\n"), 0o644))
	var stdout, stderr bytes.Buffer
	e := &Env{
		Stdout:  &stdout,
		Stderr:  &stderr,
		Environ: []string{"DOCKSHIM_TEST_SVC=tools"},
		Getwd:   func() (string, error) { return root, nil },
	}
	if code := Main([]string{"dockshim", "config"}, e); code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	if !regexp.MustCompile(`(?m)^  service +tools$`).MatchString(stdout.String()) {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestMessages(t *testing.T) {
	tests := []struct {
		name  string
		style style.Style
		print func(*Env)
		want  string
	}{
		{"error", style.Style{}, func(e *Env) { e.errorf("bad %s", "x") }, "dockshim: bad x\n"},
		{"warning", style.Style{}, func(e *Env) { e.warnf("odd %s", "x") }, "dockshim: warning: odd x\n"},
		{"styled error", style.Style{Enabled: true}, func(e *Env) { e.errorf("bad") }, "\x1b[1;31mdockshim:\x1b[0m bad\n"},
		{"styled warning", style.Style{Enabled: true}, func(e *Env) { e.warnf("odd") }, "dockshim: \x1b[1;33mwarning:\x1b[0m odd\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stderr bytes.Buffer
			tt.print(&Env{Stderr: &stderr, ErrStyle: tt.style})
			if stderr.String() != tt.want {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.want)
			}
		})
	}
}
