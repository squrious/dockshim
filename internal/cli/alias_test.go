package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/squrious/dockshim/internal/shim"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestOutdatedShimWarning(t *testing.T) {
	tests := []struct {
		name      string
		installed []string // aliases installed before the config changes
		edit      func(t *testing.T, bin string)
		want      string // the warning, "" for none
	}{
		{"stale", []string{"node", "php", "gone"}, nil, "dockshim: warning: shims are out of date (stale: gone), run `dockshim install`\n"},
		{"missing and outdated", []string{"php"}, func(t *testing.T, bin string) {
			must(t, os.WriteFile(filepath.Join(bin, "php"), []byte("#!/bin/sh\n"), 0o755))
		}, "dockshim: warning: shims are out of date (missing: node; outdated: php), run `dockshim install`\n"},
		{"up to date", []string{"node", "php"}, nil, ""},
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

			var stdout, stderr bytes.Buffer
			e := &Env{Stdout: &stdout, Stderr: &stderr, Getwd: func() (string, error) { return root, nil }}
			if code := Main([]string{"dockshim", "config"}, e); code != 0 {
				t.Fatalf("code = %d, stderr = %s", code, stderr.String())
			}
			if stderr.String() != tt.want {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tt.want)
			}
		})
	}
}
