// Package execplan turns a resolved alias invocation into docker calls, and runs them.
package execplan

import (
	"fmt"
	"io"
	"maps"
	"slices"
	"syscall"
	"time"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/docker"
	"github.com/squrious/dockshim/internal/hostpath"
)

// Step is a side action run around the command (e.g. copying files in and cleaning them up).
type Step func() error

// Plan is everything needed to run one alias invocation.
type Plan struct {
	Target docker.Target
	Runner docker.Runner
	Args   []string // docker CLI arguments
	Env    []string // environment of the docker process
	// PreRun steps run before the command; PostRun steps always run after it, in reverse order.
	PreRun  []Step
	PostRun []Step
}

// Input describes one alias invocation.
type Input struct {
	Project *config.Project
	Alias   *config.ResolvedAlias
	Runner  docker.Runner
	// Target is where the command runs. Nil means the alias service.
	Target  docker.Target
	Cwd     string // real path
	Environ []string
	// Args is the command line to run in the container. Args[0], the command name, is never translated.
	Args   []string
	TTY    bool
	Stderr io.Writer // warnings
	// HostPaths decides what a host path means on this machine. Detected when nil and needed.
	HostPaths *hostpath.Resolver
}

// Run starts the target if it is down, infers the path mappings when the alias sets none,
// then builds the plan and executes it.
func Run(in Input, stdio Stdio) (int, error) {
	if in.Target == nil {
		in.Target = NewTarget(in.Project, in.Alias, in.Runner)
	}
	if !in.Target.IsRunning() {
		env, _ := environment(in)
		if err := in.Target.EnsureUp(env, stdio.Err); err != nil {
			return 1, fmt.Errorf("starting %s: %w", in.Target, err)
		}
	}
	if in.Alias.InferPathMapping {
		// Skipped mounts are left to `dockshim config`: they would be reported on every call.
		m, _, err := InferMappings(in.Target, in.Project.Root)
		if err != nil {
			return 1, err
		}
		a := *in.Alias
		a.PathMapping = m
		in.Alias = &a
	}
	p, err := Build(in)
	if err != nil {
		return 1, err
	}
	return Execute(p, stdio)
}

// Build computes the docker exec call for in: host paths in the arguments are translated,
// forwarded variables and alias vars are passed by name, and the working directory is mapped.
// It uses the alias path mappings as they are: Run infers them first when needed.
func Build(in Input) (*Plan, error) {
	if in.Target == nil {
		in.Target = NewTarget(in.Project, in.Alias, in.Runner)
	}
	p := &Plan{Target: in.Target, Runner: in.Runner}
	workdir, _ := in.Alias.PathMapping.ToContainer(in.Cwd)

	args := in.Args
	if in.Alias.PathTranslation.Enabled {
		if in.HostPaths == nil {
			in.HostPaths = hostpath.Detect(hostOptions(in.Alias))
		}
		warn := func(format string, args ...any) { warnf(in.Stderr, format, args...) }
		args = newPathTranslator(in, workdir, warn).transform(p, args)
	}

	var names []string
	p.Env, names = environment(in)
	p.Args = p.Target.ExecArgs(docker.ExecOptions{
		TTY:     in.TTY,
		User:    in.Alias.User,
		Workdir: workdir,
		Env:     names,
	}, args)
	return p, nil
}

// environment returns the docker process environment, and the names of the variables exec forwards.
// Vars are appended after the inherited environment: for duplicate keys, os/exec keeps the last.
func environment(in Input) (env, names []string) {
	env = slices.Clone(in.Environ)
	names = in.Alias.Env.Names(in.Environ)
	for _, k := range slices.Sorted(maps.Keys(in.Alias.Vars)) {
		env = append(env, k+"="+in.Alias.Vars[k])
		if !slices.Contains(names, k) {
			names = append(names, k)
		}
	}
	return env, names
}

// NewTarget returns the compose service an alias runs in.
func NewTarget(p *config.Project, a *config.ResolvedAlias, r docker.Runner) docker.Target {
	c := &docker.Compose{Runner: r, ProjectDir: p.Root, Service: a.Service}
	if p.Compose != nil {
		c.Files, c.ProjectName = p.Compose.Files, p.Compose.ProjectName
	}
	return c
}

// Stdio are the streams given to the command. Err also receives dockshim warnings.
type Stdio struct {
	In       io.Reader
	Out, Err io.Writer
}

// Execute runs PreRun, the command and PostRun on a running target, and returns the command exit code.
// A command failing while the target is down gets a warning, as its exit code may come from
// the container stopping. It is never run again. PostRun failures are only warnings.
func Execute(p *Plan, stdio Stdio) (int, error) {
	defer func() {
		for _, step := range slices.Backward(p.PostRun) {
			if err := step(); err != nil {
				warnf(stdio.Err, "%v", err)
			}
		}
	}()
	if err := runSteps(p.PreRun); err != nil {
		return 1, err
	}
	code, err := p.Runner.Run(docker.Cmd{Dir: p.Target.Dir(), Args: p.Args, Env: p.Env, Stdin: stdio.In, Stdout: stdio.Out, Stderr: stdio.Err})
	if err == nil && code != 0 && stopped(p.Target, code) {
		warnf(stdio.Err, "%s is not running anymore: exit status %d may come from the container stopping, not from the command", p.Target, code)
	}
	return code, err
}

// Stopping a container kills the commands exec started (137, or 143 when they exit on SIGTERM)
// a moment before docker reports it stopped. For those codes, the state gets time to settle.
var (
	settleTries = 10
	settleDelay = 100 * time.Millisecond
)

// stopped reports whether the target is down after the command failed with code.
func stopped(t docker.Target, code int) bool {
	tries := 1
	if code == 128+int(syscall.SIGKILL) || code == 128+int(syscall.SIGTERM) {
		tries = settleTries
	}
	for i := range tries {
		if i > 0 {
			time.Sleep(settleDelay)
		}
		if !t.IsRunning() {
			return true
		}
	}
	return false
}

func runSteps(steps []Step) error {
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func warnf(w io.Writer, format string, args ...any) {
	if w != nil {
		_, _ = fmt.Fprintf(w, config.ToolName+": warning: "+format+"\n", args...)
	}
}
