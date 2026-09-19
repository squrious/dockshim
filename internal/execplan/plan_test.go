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
	if c, ok := NewTarget(proj, alias(), nil).(*docker.Compose); !ok || c.Service != "tools" || c.ProjectName != "p" || c.ProjectDir != "/proj" {
		t.Fatalf("compose target = %+v", c)
	}
}

// fakeTarget has a scripted running state; each IsRunning call pops the next value.
type fakeTarget struct {
	running []bool
	upErr   error
	upEnv   []string
	mounts  []docker.Mount
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
func (f *fakeTarget) ExecArgs(opts docker.ExecOptions, argv []string) []string {
	return append([]string{"--workdir", opts.Workdir}, argv...)
}
func (f *fakeTarget) ContainerID() (string, error) { return "ctr", nil }
func (f *fakeTarget) Mounts() ([]docker.Mount, error) {
	*f.log = append(*f.log, "mounts")
	return f.mounts, nil
}

type fakeRunner struct {
	codes []int
	log   *[]string
	args  []string
}

func (f *fakeRunner) Run(c docker.Cmd) (int, error) {
	*f.log = append(*f.log, "exec")
	f.args = c.Args
	c0 := f.codes[0]
	f.codes = f.codes[1:]
	return c0, nil
}

func TestExecute(t *testing.T) {
	boom := errors.New("boom")
	tests := []struct {
		name    string
		running []bool
		code    int
		preErr  error
		want    int
		wantErr bool
		log     string
		stderr  string
	}{
		{name: "success", running: []bool{true}, log: "pre exec post"},
		{name: "exit code kept", running: []bool{true}, code: 42, want: 42, log: "pre exec running? post"},
		{name: "never retried", running: []bool{false}, code: 1, want: 1, log: "pre exec running? post",
			stderr: "dockshim: warning: fake is not running anymore: exit status 1 may come from the container stopping, not from the command\n"},
		{name: "pre-run failure", running: []bool{true}, preErr: boom, want: 1, wantErr: true, log: "pre post"},
		{name: "killed while stopping", running: []bool{true, true, false}, code: 137, want: 137, log: "pre exec running? running? running? post",
			stderr: "dockshim: warning: fake is not running anymore: exit status 137 may come from the container stopping, not from the command\n"},
		{name: "killed, target up", running: []bool{true}, code: 143, want: 143, log: "pre exec running? running? running? post"},
	}
	tries, delay := settleTries, settleDelay
	settleTries, settleDelay = 3, 0
	t.Cleanup(func() { settleTries, settleDelay = tries, delay })
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var log []string
			var stderr bytes.Buffer
			p := &Plan{
				Target:  &fakeTarget{running: tt.running, log: &log},
				Runner:  &fakeRunner{codes: []int{tt.code}, log: &log},
				PreRun:  []Step{func() error { log = append(log, "pre"); return tt.preErr }},
				PostRun: []Step{func() error { log = append(log, "post"); return nil }},
			}
			code, err := Execute(p, Stdio{Err: &stderr})
			if code != tt.want || (err != nil) != tt.wantErr {
				t.Fatalf("code=%d err=%v", code, err)
			}
			if got := strings.Join(log, " "); got != tt.log {
				t.Fatalf("log = %q, want %q", got, tt.log)
			}
			if stderr.String() != tt.stderr {
				t.Fatalf("stderr = %q", stderr.String())
			}
		})
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

func TestRun(t *testing.T) {
	proj := &config.Project{Root: "/proj"}
	disabled := func(a *config.ResolvedAlias) *config.ResolvedAlias {
		a.PathTranslation.Enabled = false
		return a
	}
	inferred := disabled(alias())
	inferred.InferPathMapping, inferred.PathMapping = true, nil
	mounts := []docker.Mount{{Type: docker.MountBind, Source: "/proj/src", Destination: "/src"}}

	tests := []struct {
		name    string
		alias   *config.ResolvedAlias
		running []bool
		upErr   error
		wantErr bool
		log     string
		args    string
	}{
		{name: "running", alias: disabled(alias()), running: []bool{true}, log: "running? exec", args: "--workdir /app/src"},
		{name: "starts when down", alias: disabled(alias()), running: []bool{false}, log: "running? up exec", args: "--workdir /app/src"},
		{name: "start failure", alias: disabled(alias()), running: []bool{false}, upErr: errors.New("boom"), wantErr: true, log: "running? up"},
		{name: "infers mappings once up", alias: inferred, running: []bool{false}, log: "running? up mounts exec", args: "--workdir /src"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var log []string
			target := &fakeTarget{running: tt.running, upErr: tt.upErr, mounts: mounts, log: &log}
			runner := &fakeRunner{codes: []int{0}, log: &log}
			in := Input{Project: proj, Alias: tt.alias, Target: target, Runner: runner, Cwd: "/proj/src",
				Environ: []string{"FOO=1"}, Args: []string{"php"}}
			code, err := Run(in, Stdio{})
			if (err != nil) != tt.wantErr || (err == nil && code != 0) {
				t.Fatalf("code=%d err=%v", code, err)
			}
			if got := strings.Join(log, " "); got != tt.log {
				t.Fatalf("log = %q, want %q", got, tt.log)
			}
			if tt.args != "" && !strings.Contains(strings.Join(runner.args, " "), tt.args) {
				t.Fatalf("args = %q, want %q", runner.args, tt.args)
			}
			// Starting sees the same environment as the command, so compose interpolates the project identically.
			if target.upEnv != nil && !slices.Equal(target.upEnv, []string{"FOO=1", "APP_ENV=dev", "SECRET=v"}) {
				t.Fatalf("up env = %q", target.upEnv)
			}
		})
	}
}

func TestInferMappings(t *testing.T) {
	root := t.TempDir()
	mounts := []docker.Mount{
		{Type: docker.MountBind, Source: root, Destination: "/var/www"},
		{Type: "volume", Source: "/var/lib/docker/volumes/v/_data", Destination: "/data"},
		{Type: docker.MountBind, Source: root + "/src", Destination: "/src"},
		{Type: docker.MountBind, Source: "/var/run/docker.sock", Destination: "/var/run/docker.sock"},
		{Type: docker.MountBind, Source: root, Destination: "/app"},
	}
	got, outside := inferMappings(mounts, root)
	want := pathmap.Map{{Host: root, Container: "/app"}, {Host: root + "/src", Container: "/src"}, {Host: root, Container: "/var/www"}}
	if !slices.Equal(got, want) {
		t.Fatalf("mappings = %v", got)
	}
	if !slices.Equal(outside, []string{"/var/run/docker.sock"}) {
		t.Fatalf("outside = %q", outside)
	}
	// A directory mounted twice maps to the first destination, in sorted order.
	if ctr, _ := got.ToContainer(root + "/x"); ctr != "/app/x" {
		t.Fatalf("ToContainer = %q", ctr)
	}
}
