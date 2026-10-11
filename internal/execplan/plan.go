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

// WarnFunc reports a problem that doesn't stop the command. A nil WarnFunc discards it.
type WarnFunc func(format string, args ...any)

func (w WarnFunc) warn(format string, args ...any) {
	if w != nil {
		w(format, args...)
	}
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
	Args []string
	TTY  bool
	Warn WarnFunc
	// HostPaths decides what a host path means on this machine. Detected when nil and needed.
	HostPaths *hostpath.Resolver
}

// Run starts the target if it is down, infers the path mappings when the alias sets none,
// then builds the plan and executes it.
func Run(in Input, stdio Stdio) (int, error) {
	if in.Target == nil {
		in.Target = NewTarget(in.Project, in.Alias.Service, in.Runner)
	}
	if !in.Target.IsRunning() {
		// The caller's environment, so that compose interpolates the project as it will for the exec.
		if err := in.Target.EnsureUp(in.Environ, stdio.Err); err != nil {
			return 1, fmt.Errorf("starting %s: %w", in.Target, err)
		}
	}
	if in.Alias.InferPathMapping {
		// Skipped mounts are left to `dockshim config`: they would be reported on every call.
		mounts, err := in.Target.Mounts()
		if err != nil {
			return 1, err
		}
		a := *in.Alias
		a.PathMapping, _ = InferMappings(mounts, in.Project.Root)
		in.Alias = &a
	}
	p, err := Build(in)
	if err != nil {
		return 1, err
	}
	return Execute(p, stdio)
}

// Build computes the docker exec call for in: host paths in the arguments are translated,
// forwarded variables are passed by name and alias vars by value, and the working directory is
// mapped. Docker runs with the caller's environment, unchanged.
// It uses the alias path mappings as they are: Run infers them first when needed.
func Build(in Input) (*Plan, error) {
	if in.Target == nil {
		in.Target = NewTarget(in.Project, in.Alias.Service, in.Runner)
	}
	p := &Plan{Target: in.Target, Runner: in.Runner}
	workdir, _ := in.Alias.PathMapping.ToContainer(in.Cwd)

	args := in.Args
	if in.Alias.PathTranslation.Enabled {
		if in.HostPaths == nil {
			in.HostPaths = hostpath.Detect(in.Alias.PathTranslation.Allow)
		}
		args = newPathTranslator(in, workdir, in.Warn.warn).transform(p, args)
	}

	names, set := envArgs(in)
	p.Env = in.Environ
	p.Args = p.Target.ExecArgs(docker.ExecOptions{
		TTY:     in.TTY,
		User:    in.Alias.User,
		Workdir: workdir,
		Env:     names,
		Set:     set,
	}, args)
	return p, nil
}

// envArgs returns the names of the host variables exec forwards, and the alias vars it sets, as
// sorted NAME=value entries. A var wins over a forwarded variable of the same name.
func envArgs(in Input) (names, set []string) {
	for _, n := range in.Alias.Env.Names(in.Environ) {
		if _, ok := in.Alias.Vars[n]; !ok {
			names = append(names, n)
		}
	}
	for _, k := range slices.Sorted(maps.Keys(in.Alias.Vars)) {
		set = append(set, k+"="+in.Alias.Vars[k])
	}
	return names, set
}

// NewTarget returns a service of the project's compose project.
func NewTarget(p *config.Project, service string, r docker.Runner) *docker.Compose {
	c := &docker.Compose{Runner: r, ProjectDir: p.Root, Service: service}
	if p.Compose != nil {
		c.Files, c.ProjectName = p.Compose.Files, p.Compose.ProjectName
	}
	return c
}

// Stdio are the streams given to the command.
type Stdio struct {
	In       io.Reader
	Out, Err io.Writer
}

// Execute runs PreRun, the command and PostRun on a running target, and returns the command exit code.
// The command is never run again. PostRun failures are ignored: the command already ran, and what
// they leave behind is harmless.
func Execute(p *Plan, stdio Stdio) (int, error) {
	defer func() {
		for _, step := range slices.Backward(p.PostRun) {
			_ = step()
		}
	}()
	if err := runSteps(p.PreRun); err != nil {
		return 1, err
	}
	return p.Runner.Run(docker.Cmd{Dir: p.Target.Dir(), Args: p.Args, Env: p.Env, Stdin: stdio.In, Stdout: stdio.Out, Stderr: stdio.Err})
}

func runSteps(steps []Step) error {
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}
