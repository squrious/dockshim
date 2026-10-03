package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"
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
