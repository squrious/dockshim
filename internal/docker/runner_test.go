package docker

import (
	"bytes"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// Signals dropped by dockshim must keep their default action in docker.
func TestExecRunnerLeavesSignalsToChild(t *testing.T) {
	if _, err := os.Stat("/proc/self/status"); err != nil {
		t.Skip("needs /proc")
	}
	var out bytes.Buffer
	code, err := ExecRunner{Binary: "/bin/sh"}.Run(Cmd{Args: []string{"-c", "grep SigIgn /proc/self/status"}, Stdout: &out})
	if err != nil || code != 0 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	mask, err := strconv.ParseUint(strings.TrimSpace(strings.TrimPrefix(out.String(), "SigIgn:")), 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGQUIT} {
		if mask&(1<<(sig-1)) != 0 {
			t.Errorf("docker would inherit %v as ignored", sig)
		}
	}
}

func TestExecRunnerExitCodes(t *testing.T) {
	for _, tt := range []struct {
		script string
		want   int
	}{
		{"exit 0", 0},
		{"exit 42", 42},
		{"kill -TERM $$", 143},
	} {
		code, err := ExecRunner{Binary: "/bin/sh"}.Run(Cmd{Args: []string{"-c", tt.script}})
		if err != nil || code != tt.want {
			t.Errorf("%s: code=%d err=%v, want %d", tt.script, code, err, tt.want)
		}
	}
	if code, err := (ExecRunner{Binary: "/nonexistent/docker"}).Run(Cmd{}); code != 127 || err == nil {
		t.Errorf("missing binary: code=%d err=%v", code, err)
	}
}
