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
	Stderr       io.Writer // warnings
}

func Build(in Input) (*Plan, error) {
	p := &Plan{Target: NewTarget(in.Project, in.Alias, in.Runner)}

	transformers := in.Transformers
	if in.Alias.PathTranslation.Enabled {
		warn := func(format string, args ...any) {
			if in.Stderr != nil {
				_, _ = fmt.Fprintf(in.Stderr, "dockshim: warning: "+format+"\n", args...)
			}
		}
		transformers = append([]ArgTransformer{newPathTranslator(in, warn)}, transformers...)
	}

	args := in.Args
	for _, t := range transformers {
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
// When the command fails with 1 because the target went down meanwhile, it is started again,
// PreRun steps replayed and the command retried once. PostRun failures are only warnings.
func Execute(p *Plan, r docker.Runner, stdio Stdio) (int, error) {
	defer func() {
		for _, step := range slices.Backward(p.PostRun) {
			if err := step(); err != nil && stdio.Err != nil {
				_, _ = fmt.Fprintf(stdio.Err, "dockshim: warning: %v\n", err)
			}
		}
	}()
	start := func(verb string) error {
		if err := p.Target.EnsureUp(stdio.Err); err != nil {
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
	code, err := r.Run(cmd)
	if err != nil || code != 1 || p.Target.IsRunning() {
		return code, err
	}
	if err := start("restarting"); err != nil {
		return 1, err
	}
	return r.Run(cmd)
}

func runSteps(steps []Step) error {
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}
