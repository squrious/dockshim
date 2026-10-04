package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/squrious/dockshim/internal/shim"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Only undefined aliases are run, so that no docker call happens.
func TestOutdatedShimWarning(t *testing.T) {
	const warning = "warning: shims are out of date"
	tests := []struct {
		name        string
		interactive bool
		viaShim     bool
		installed   []string // aliases installed before the config changes
		edit        func(t *testing.T, bin string)
		want        string // part of the warning, "" for none
	}{
		{"stale", true, true, []string{"node", "php", "gone"}, nil, "(stale: gone)"},
		{"missing and outdated", true, true, []string{"php"}, func(t *testing.T, bin string) {
			must(t, os.WriteFile(filepath.Join(bin, "php"), []byte("#!/bin/sh\n"), 0o755))
		}, "(missing: node; outdated: php)"},
		{"up to date", true, true, []string{"node", "php"}, nil, ""},
		{"not a terminal", false, true, []string{"php", "gone"}, nil, ""},
		{"not through a shim", true, false, []string{"php", "gone"}, nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			bin := filepath.Join(root, ".dockshim", "bin")
			if _, err := shim.Install(bin, tt.installed); err != nil {
				t.Fatal(err)
			}
			if tt.edit != nil {
				tt.edit(t, bin)
			}
			must(t, os.WriteFile(filepath.Join(root, ".dockshim.yaml"), []byte("aliases: {php: {service: tools}, node: {service: tools}}\n"), 0o644))

			var stderr bytes.Buffer
			e := &Env{Stderr: &stderr, Getwd: func() (string, error) { return root, nil }, Interactive: tt.interactive}
			args := []string{"dockshim", "run", "undefined"}
			if tt.viaShim {
				args = []string{"dockshim", "run", "--shim", filepath.Join(bin, "undefined"), "undefined"}
			}
			if code := Main(args, e); code != exitUnknownAlias {
				t.Fatalf("code = %d", code)
			}
			out := stderr.String()
			if !strings.Contains(out, `alias "undefined" is not defined`) {
				t.Fatalf("stderr = %s", out)
			}
			switch {
			case tt.want == "" && strings.Contains(out, warning):
				t.Fatalf("unexpected warning: %s", out)
			case tt.want != "" && !strings.Contains(out, warning+" "+tt.want+", run `dockshim install`"):
				t.Fatalf("stderr = %s, want %q", out, tt.want)
			}
		})
	}
}
