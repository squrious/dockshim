package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/squrious/dockshim/internal/style"
)

func TestHelp(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		style style.Style
		want  []string
	}{
		{"root", []string{"--help"}, style.Style{}, []string{"Usage:\n  dockshim [command]\n", "\n  config      Print the effective configuration\n", `Use "dockshim [command] --help"`}},
		{"subcommand", []string{"run", "--help"}, style.Style{}, []string{"Run an alias, as if invoked through its shim\n\nUsage:\n  dockshim run <alias> [args...] [flags]\n\nFlags:\n"}},
		{"styled", []string{"--help"}, style.Style{Enabled: true}, []string{"\x1b[1mUsage:\x1b[0m\n", "  \x1b[36mconfig    \x1b[0m  Print"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			e := &Env{Stdout: &stdout, Stderr: &stderr, OutStyle: tt.style}
			if code := Main(append([]string{"dockshim"}, tt.args...), e); code != 0 {
				t.Fatalf("code = %d, stderr = %s", code, stderr.String())
			}
			for _, w := range tt.want {
				if !strings.Contains(stdout.String(), w) {
					t.Errorf("stdout lacks %q:\n%s", w, stdout.String())
				}
			}
			if !tt.style.Enabled && strings.Contains(stdout.String(), "\x1b") {
				t.Errorf("unstyled stdout has escape codes: %q", stdout.String())
			}
		})
	}
}
