// Package execplan turns a resolved alias invocation into docker calls, and runs them.
package execplan

import (
	"fmt"
	"io"
	"maps"
	"slices"

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
	Cwd     string // real path
	Environ []string
	// Args is the command line to run in the container. Args[0], the command name, is never translated.
	Args   []string
	TTY    bool
	Stderr io.Writer // warnings
	// HostPaths decides what a host path means on this machine. Detected when nil and needed.
	HostPaths *hostpath.Resolver
}

// Build computes the docker exec call for in: host paths in the arguments are translated,
// forwarded variables and alias vars are passed by name, and the working directory is mapped.
func Build(in Input) (*Plan, error) {
	p := &Plan{Target: NewTarget(in.Project, in.Alias, in.Runner), Runner: in.Runner}
	workdir, _ := in.Alias.PathMapping.ToContainer(in.Cwd)

	args := in.Args
	if in.Alias.PathTranslation.Enabled {
		if in.HostPaths == nil {
			in.HostPaths = hostpath.Detect(hostOptions(in.Alias))
		}
		warn := func(format string, args ...any) { warnf(in.Stderr, format, args...) }
		args = newPathTranslator(in, workdir, warn).transform(p, args)
	}

	// Vars are appended after the inherited environment: for duplicate keys, os/exec keeps the last.
	env := slices.Clone(in.Environ)
	names := in.Alias.Env.Names(in.Environ)
	for _, k := range slices.Sorted(maps.Keys(in.Alias.Vars)) {
		env = append(env, k+"="+in.Alias.Vars[k])
		if !slices.Contains(names, k) {
			names = append(names, k)
		}
	}
	p.Env = env

	p.Args = p.Target.ExecArgs(docker.ExecOptions{
		TTY:     in.TTY,
		User:    in.Alias.User,
		Workdir: workdir,
		Env:     names,
	}, args)
	return p, nil
}

// NewTarget returns the compose service or the container an alias runs in.
func NewTarget(p *config.Project, a *config.ResolvedAlias, r docker.Runner) docker.Target {
	if a.Service != "" {
		c := &docker.Compose{Runner: r, ProjectDir: p.Root, Service: a.Service}
		if p.Compose != nil {
			c.Files, c.ProjectName = p.Compose.Files, p.Compose.ProjectName
		}
		return c
	}
	return &docker.Container{Runner: r, ProjectDir: p.Root, Name: a.Container}
}

// Stdio are the streams given to the command. Err also receives dockshim warnings.
type Stdio struct {
	In       io.Reader
	Out, Err io.Writer
}

// Execute starts the target if needed, runs the command and returns its exit code.
// When the command fails with 1 because the target went down meanwhile, it is started again,
// PreRun steps replayed and the command retried once. PostRun failures are only warnings.
func Execute(p *Plan, stdio Stdio) (int, error) {
	defer func() {
		for _, step := range slices.Backward(p.PostRun) {
			if err := step(); err != nil {
				warnf(stdio.Err, "%v", err)
			}
		}
	}()
	start := func(verb string) error {
		if err := p.Target.EnsureUp(p.Env, stdio.Err); err != nil {
			return fmt.Errorf("%s %s: %w", verb, p.Target, err)
		}
		return runSteps(p.PreRun)
	}

	if p.Target.IsRunning() {
		if err := runSteps(p.PreRun); err != nil {
			return 1, err
		}
	} else if err := start("starting"); err != nil {
		return 1, err
	}

	cmd := docker.Cmd{Dir: p.Target.Dir(), Args: p.Args, Env: p.Env, Stdin: stdio.In, Stdout: stdio.Out, Stderr: stdio.Err}
	code, err := p.Runner.Run(cmd)
	if err != nil || code != 1 || p.Target.IsRunning() {
		return code, err
	}
	if err := start("restarting"); err != nil {
		return 1, err
	}
	return p.Runner.Run(cmd)
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
