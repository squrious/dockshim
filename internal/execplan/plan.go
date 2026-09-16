// Package execplan turns a resolved alias invocation into docker calls, and runs them.
package execplan

import (
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/docker"
)

// Step is a side action run around the command (e.g. copying files in and cleaning them up).
type Step func() error

// ArgTransformer rewrites the command arguments, possibly registering steps on the plan.
type ArgTransformer interface {
	Transform(p *Plan, args []string) ([]string, error)
}

type Plan struct {
	Target docker.Target
	Args   []string // docker CLI arguments
	Env    []string // environment of the docker process
	// PreRun steps run before the command; PostRun steps always run after it, in reverse order.
	PreRun  []Step
	PostRun []Step
}

type Input struct {
	Project      *config.Project
	Alias        *config.ResolvedAlias
	Runner       docker.Runner
	Cwd          string // real path
	Environ      []string
	Args         []string
	TTY          bool
	Transformers []ArgTransformer
}

func Build(in Input) (*Plan, error) {
	p := &Plan{Target: NewTarget(in.Project, in.Alias, in.Runner)}

	args := in.Args
	for _, t := range in.Transformers {
		var err error
		if args, err = t.Transform(p, args); err != nil {
			return nil, err
		}
	}

	env := slices.Clone(in.Environ)
	names := in.Alias.Env.Names(in.Environ)
	for _, k := range slices.Sorted(maps.Keys(in.Alias.Vars)) {
		env = append(env, k+"="+in.Alias.Vars[k])
		if !slices.Contains(names, k) {
			names = append(names, k)
		}
	}
	p.Env = env

	workdir, _ := in.Alias.PathMapping.ToContainer(in.Cwd)
	p.Args = p.Target.ExecArgs(docker.ExecOptions{
		TTY:     in.TTY,
		User:    in.Alias.User,
		Workdir: workdir,
		Env:     names,
	}, args)
	return p, nil
}

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

type Stdio struct {
	In       io.Reader
	Out, Err io.Writer
}

// Execute starts the target if needed, runs the command and returns its exit code.
// When the command fails with 1 because the target went down meanwhile, it is started again and the command retried once.
func Execute(p *Plan, r docker.Runner, stdio Stdio) (code int, err error) {
	defer func() {
		for _, step := range slices.Backward(p.PostRun) {
			if e := step(); e != nil && err == nil {
				err = e
			}
		}
	}()
	if !p.Target.IsRunning() {
		if err := p.Target.EnsureUp(stdio.Err); err != nil {
			return 1, fmt.Errorf("starting %s: %w", p.Target, err)
		}
	}

	for _, step := range p.PreRun {
		if err := step(); err != nil {
			return 1, err
		}
	}

	cmd := docker.Cmd{Dir: p.Target.Dir(), Args: p.Args, Env: p.Env, Stdin: stdio.In, Stdout: stdio.Out, Stderr: stdio.Err}
	code, err = r.Run(cmd)
	if err != nil || code != 1 || p.Target.IsRunning() {
		return code, err
	}
	if err := p.Target.EnsureUp(stdio.Err); err != nil {
		return 1, fmt.Errorf("restarting %s: %w", p.Target, err)
	}
	return r.Run(cmd)
}
