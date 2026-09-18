package execplan

import (
	"bytes"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/docker"
	"github.com/squrious/dockshim/internal/envfilter"
	"github.com/squrious/dockshim/internal/pathmap"
)

func alias() *config.ResolvedAlias {
	return &config.ResolvedAlias{
		Service:     "tools",
		User:        "1000:1000",
		PathMapping: pathmap.Map{{Host: "/proj", Container: "/app"}},
		Env:         envfilter.Rules{Deny: []string{"HOME", "SECRET"}},
		Vars:        map[string]string{"SECRET": "v", "APP_ENV": "dev"},
	}
}

func TestBuild(t *testing.T) {
	proj := &config.Project{Root: "/proj"}
	in := Input{
		Project: proj,
		Alias:   alias(),
		Cwd:     "/proj/src",
		Environ: []string{"HOME=/h", "FOO=1"},
		Args:    []string{"php", "-r", "echo 1;"},
	}
	p, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"compose", "exec", "-T", "--env", "FOO", "--env", "APP_ENV", "--env", "SECRET",
		"--user", "1000:1000", "--workdir", "/app/src", "tools", "php", "-r", "echo 1;"}
	if !slices.Equal(p.Args, want) {
		t.Fatalf("args = %q", p.Args)
	}
	if !slices.Equal(p.Env, []string{"HOME=/h", "FOO=1", "APP_ENV=dev", "SECRET=v"}) {
		t.Fatalf("env = %q", p.Env)
	}

	in.Cwd = "/elsewhere"
	if p, err = Build(in); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(p.Args, "--workdir") {
		t.Fatalf("no workdir expected outside mappings: %q", p.Args)
	}
}

func TestNewTarget(t *testing.T) {
	proj := &config.Project{Root: "/proj", Compose: &config.Compose{Files: []string{"/proj/c.yaml"}, ProjectName: "p"}}
	a := alias()
	if c, ok := NewTarget(proj, a, nil).(*docker.Compose); !ok || c.Service != "tools" || c.ProjectName != "p" || c.ProjectDir != "/proj" {
		t.Fatalf("compose target = %+v", c)
	}
	a.Service, a.Container = "", "ctr"
	if c, ok := NewTarget(proj, a, nil).(*docker.Container); !ok || c.Name != "ctr" || c.ProjectDir != "/proj" {
		t.Fatalf("container target = %+v", c)
	}
}

// fakeTarget has a scripted running state; each IsRunning call pops the next value.
type fakeTarget struct {
	running []bool
	upErr   error
	upEnv   []string
	log     *[]string
}

func (f *fakeTarget) String() string { return "fake" }
func (f *fakeTarget) Dir() string    { return "/proj" }
func (f *fakeTarget) IsRunning() bool {
	*f.log = append(*f.log, "running?")
	r := f.running[0]
	if len(f.running) > 1 {
		f.running = f.running[1:]
	}
	return r
}
func (f *fakeTarget) EnsureUp(env []string, _ io.Writer) error {
	*f.log = append(*f.log, "up")
	f.upEnv = env
	return f.upErr
}
func (f *fakeTarget) ExecArgs(docker.ExecOptions, []string) []string { return nil }
func (f *fakeTarget) ContainerID() (string, error)                   { return "ctr", nil }

type fakeRunner struct {
	codes []int
	log   *[]string
}

func (f *fakeRunner) Run(docker.Cmd) (int, error) {
	*f.log = append(*f.log, "exec")
	c := f.codes[0]
	f.codes = f.codes[1:]
	return c, nil
}

func TestExecute(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		running []bool
		codes   []int
		upErr   error
		preErr  error
		want    int
		wantErr bool
		log     string
	}{
		{name: "running", running: []bool{true}, codes: []int{0}, log: "running? pre exec post"},
		{name: "starts when down", running: []bool{false, true}, codes: []int{0}, log: "running? up pre exec post"},
		{name: "exit code kept", running: []bool{true}, codes: []int{42}, want: 42, log: "running? pre exec post"},
		{name: "exit 1 while still running", running: []bool{true}, codes: []int{1}, want: 1, log: "running? pre exec running? post"},
		{name: "retries once when stopped meanwhile", running: []bool{true, false}, codes: []int{1, 1}, want: 1, log: "running? pre exec running? up pre exec post"},
		{name: "start failure", running: []bool{false}, upErr: boom, want: 1, wantErr: true, log: "running? up post"},
		{name: "pre-run failure", running: []bool{true}, preErr: boom, want: 1, wantErr: true, log: "running? pre post"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var log []string
			p := &Plan{
				Target:  &fakeTarget{running: tt.running, upErr: tt.upErr, log: &log},
				Runner:  &fakeRunner{codes: tt.codes, log: &log},
				PreRun:  []Step{func() error { log = append(log, "pre"); return tt.preErr }},
				PostRun: []Step{func() error { log = append(log, "post"); return nil }},
			}
			code, err := Execute(p, Stdio{})
			if code != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("code=%d err=%v", code, err)
			}
			if got := strings.Join(log, " "); got != tt.log {
				t.Fatalf("log = %q, want %q", got, tt.log)
			}
		})
	}
}

// Starting sees the same environment as the command, so compose interpolates the project identically.
func TestExecuteStartsWithPlanEnv(t *testing.T) {
	var log []string
	target := &fakeTarget{running: []bool{false, true}, log: &log}
	p := &Plan{Target: target, Runner: &fakeRunner{codes: []int{0}, log: &log}, Env: []string{"APP_ENV=dev"}}
	if _, err := Execute(p, Stdio{}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(target.upEnv, p.Env) {
		t.Fatalf("up env = %q", target.upEnv)
	}
}

func TestExecutePostRunFailureWarns(t *testing.T) {
	var log []string
	var stderr bytes.Buffer
	p := &Plan{
		Target:  &fakeTarget{running: []bool{true}, log: &log},
		Runner:  &fakeRunner{codes: []int{0}, log: &log},
		PostRun: []Step{func() error { return errors.New("cleanup failed") }},
	}
	if code, err := Execute(p, Stdio{Err: &stderr}); code != 0 || err != nil {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if stderr.String() != "dockshim: warning: cleanup failed\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
