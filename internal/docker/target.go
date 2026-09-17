package docker

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

type ExecOptions struct {
	TTY     bool
	User    string
	Workdir string
	Env     []string // variable names, values are read from the docker process env
}

// Target is where commands run: a compose service or a plain container.
type Target interface {
	String() string
	// Dir is the directory docker must run from.
	Dir() string
	IsRunning() bool
	// EnsureUp starts the target, keeping existing containers and volumes. Idempotent.
	EnsureUp(stderr io.Writer) error
	ExecArgs(opts ExecOptions, argv []string) []string
	// ContainerID identifies the running container, for docker cp and exec.
	ContainerID() (string, error)
}

func commonExecFlags(opts ExecOptions) []string {
	var args []string
	for _, name := range opts.Env {
		args = append(args, "--env", name)
	}
	if opts.User != "" {
		args = append(args, "--user", opts.User)
	}
	if opts.Workdir != "" {
		args = append(args, "--workdir", opts.Workdir)
	}
	return args
}

func output(r Runner, dir string, args ...string) (string, int) {
	var out bytes.Buffer
	code, err := r.Run(Cmd{Dir: dir, Args: args, Stdout: &out, Stderr: io.Discard})
	if err != nil {
		return "", 127
	}
	return strings.TrimSpace(out.String()), code
}

func run(r Runner, dir string, env []string, stdout, stderr io.Writer, args ...string) error {
	code, err := r.Run(Cmd{Dir: dir, Args: args, Env: env, Stdout: stdout, Stderr: stderr})
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("docker %s: exit status %d", strings.Join(args, " "), code)
	}
	return nil
}

type Compose struct {
	Runner      Runner
	ProjectDir  string
	Files       []string
	ProjectName string
	Service     string
}

func (c *Compose) String() string { return "compose service " + c.Service }
func (c *Compose) Dir() string    { return c.ProjectDir }

func (c *Compose) base() []string {
	args := []string{"compose"}
	for _, f := range c.Files {
		args = append(args, "--file", f)
	}
	if c.ProjectName != "" {
		args = append(args, "--project-name", c.ProjectName)
	}
	return args
}

func (c *Compose) IsRunning() bool {
	out, code := output(c.Runner, c.ProjectDir, append(c.base(), "ps", "--status", "running", "--quiet", c.Service)...)
	return code == 0 && out != ""
}

// ContainerID returns the first container of the service; scaled services are not distinguished.
func (c *Compose) ContainerID() (string, error) {
	out, code := output(c.Runner, c.ProjectDir, append(c.base(), "ps", "--status", "running", "--quiet", c.Service)...)
	id, _, _ := strings.Cut(out, "\n")
	if code != 0 || id == "" {
		return "", fmt.Errorf("%s is not running", c)
	}
	return id, nil
}

func (c *Compose) EnsureUp(stderr io.Writer) error {
	env := append(os.Environ(), "COMPOSE_PROGRESS=quiet")
	return run(c.Runner, c.ProjectDir, env, stderr, stderr, append(c.base(), "up", "--detach", c.Service)...)
}

func (c *Compose) ExecArgs(opts ExecOptions, argv []string) []string {
	args := append(c.base(), "exec")
	if !opts.TTY {
		args = append(args, "-T")
	}
	args = append(args, commonExecFlags(opts)...)
	args = append(args, c.Service)
	return append(args, argv...)
}

type Container struct {
	Runner     Runner
	ProjectDir string
	Name       string
}

func (c *Container) String() string { return "container " + c.Name }
func (c *Container) Dir() string    { return c.ProjectDir }

func (c *Container) IsRunning() bool {
	out, code := output(c.Runner, c.ProjectDir, "inspect", "--format", "{{.State.Running}}", c.Name)
	return code == 0 && out == "true"
}

func (c *Container) ContainerID() (string, error) { return c.Name, nil }

func (c *Container) EnsureUp(stderr io.Writer) error {
	return run(c.Runner, c.ProjectDir, nil, io.Discard, stderr, "start", c.Name)
}

func (c *Container) ExecArgs(opts ExecOptions, argv []string) []string {
	args := []string{"exec", "--interactive"}
	if opts.TTY {
		args = append(args, "--tty")
	}
	args = append(args, commonExecFlags(opts)...)
	args = append(args, c.Name)
	return append(args, argv...)
}
