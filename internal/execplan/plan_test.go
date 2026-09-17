package execplan

import (
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
		Name:        "php",
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
	p, _ = Build(in)
	if slices.Contains(p.Args, "--workdir") {
		t.Fatalf("no workdir expected outside mappings: %q", p.Args)
	}

	in.Alias.Service, in.Alias.Container = "", "ctr"
	p, _ = Build(in)
	if p.Args[0] != "exec" || !slices.Contains(p.Args, "ctr") {
		t.Fatalf("container args = %q", p.Args)
	}
}

type prefixTransformer string

func (pt prefixTransformer) Transform(p *Plan, args []string) ([]string, error) {
	p.PostRun = append(p.PostRun, func() error { return nil })
	return append([]string{string(pt)}, args...), nil
}

func TestBuildTransformers(t *testing.T) {
	p, err := Build(Input{
		Project:      &config.Project{Root: "/proj"},
		Alias:        alias(),
		Args:         []string{"php"},
		Transformers: []ArgTransformer{prefixTransformer("env")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(p.Args[len(p.Args)-2:], []string{"env", "php"}) || len(p.PostRun) != 1 {
		t.Fatalf("plan = %+v", p)
	}
}

// fakeTarget has a scripted running state; each IsRunning call pops the next value.
type fakeTarget struct {
	running []bool
	ups     int
	upErr   error
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
func (f *fakeTarget) EnsureUp(io.Writer) error {
	*f.log = append(*f.log, "up")
	f.ups++
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
	tests := []struct {
		name    string
		running []bool
		codes   []int
		upErr   error
		want    int
		wantErr bool
		log     string
	}{
		{"running", []bool{true}, []int{0}, nil, 0, false, "running? pre exec post"},
		{"starts when down", []bool{false, true}, []int{0}, nil, 0, false, "running? up pre exec post"},
		{"exit code kept", []bool{true}, []int{42}, nil, 42, false, "running? pre exec post"},
		{"exit 1 while still running", []bool{true}, []int{1}, nil, 1, false, "running? pre exec running? post"},
		{"retries once when stopped meanwhile", []bool{true, false}, []int{1, 1}, nil, 1, false, "running? pre exec running? up pre exec post"},
		{"start failure", []bool{false}, nil, errors.New("boom"), 1, true, "running? up post"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var log []string
			p := &Plan{
				Target:  &fakeTarget{running: tt.running, upErr: tt.upErr, log: &log},
				PreRun:  []Step{func() error { log = append(log, "pre"); return nil }},
				PostRun: []Step{func() error { log = append(log, "post"); return nil }},
			}
			code, err := Execute(p, &fakeRunner{codes: tt.codes, log: &log}, Stdio{})
			if code != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("code=%d err=%v", code, err)
			}
			if got := strings.Join(log, " "); got != tt.log {
				t.Fatalf("log = %q, want %q", got, tt.log)
			}
		})
	}
}
