package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

// ExecOptions are the docker exec flags shared by every target.
type ExecOptions struct {
	TTY     bool
	User    string
	Workdir string   // empty keeps the container's own
	Env     []string // variable names, values are read from the docker process env
}

// Target is where commands run. Compose is the only implementation; tests fake it.
type Target interface {
	fmt.Stringer
	// Dir is the directory docker must run from.
	Dir() string
	// IsRunning reports whether the target can exec commands now. Any failure counts as not running.
	IsRunning() bool
	// EnsureUp starts the target, keeping existing containers and volumes. Idempotent.
	// env is the docker process environment, as for exec; nil means inherit.
	EnsureUp(env []string, stderr io.Writer) error
	// ExecArgs returns the docker CLI arguments running argv in the target.
	ExecArgs(opts ExecOptions, argv []string) []string
	// ContainerID identifies the running container, for docker cp and exec.
	ContainerID() (string, error)
	// Mounts lists the mounts of the running container.
	Mounts() ([]Mount, error)
}

// Mount is a mount of a container, as docker inspect reports it. Source is a host path for binds.
type Mount struct {
	Type        string
	Source      string
	Destination string
}

// MountBind is the Mount.Type of a bind mount.
const MountBind = "bind"

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

// output runs docker and returns its trimmed stdout. A non-zero exit is an error.
func output(r Runner, dir string, args ...string) (string, error) {
	var out bytes.Buffer
	if err := run(r, dir, nil, &out, io.Discard, args...); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
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

// Compose targets a service of a compose project, run from ProjectDir.
// Files and ProjectName are optional: compose then applies its own discovery.
type Compose struct {
	Runner      Runner
	ProjectDir  string
	Files       []string
	ProjectName string
	Service     string
	id          string // last container seen running, so one invocation asks compose once
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

// IsRunning always asks compose.
func (c *Compose) IsRunning() bool {
	c.id = ""
	_, err := c.ContainerID()
	return err == nil
}

// ContainerID returns the first container of the service; scaled services are not distinguished.
// It reuses the container found by the last IsRunning or ContainerID call, until EnsureUp.
func (c *Compose) ContainerID() (string, error) {
	if c.id != "" {
		return c.id, nil
	}
	out, err := output(c.Runner, c.ProjectDir, append(c.base(), "ps", "--status", "running", "--quiet", c.Service)...)
	if err != nil {
		return "", err
	}
	id, _, _ := strings.Cut(out, "\n")
	if id == "" {
		return "", fmt.Errorf("%s is not running", c)
	}
	c.id = id
	return id, nil
}

func (c *Compose) EnsureUp(env []string, stderr io.Writer) error {
	c.id = ""
	if env == nil {
		env = os.Environ()
	}
	env = append(slices.Clone(env), "COMPOSE_PROGRESS=quiet")
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

func (c *Compose) Mounts() ([]Mount, error) {
	id, err := c.ContainerID()
	if err != nil {
		return nil, err
	}
	out, err := output(c.Runner, c.ProjectDir, "inspect", "--format", "{{json .Mounts}}", id)
	if err != nil {
		return nil, err
	}
	var mounts []Mount
	if err := json.Unmarshal([]byte(out), &mounts); err != nil {
		return nil, fmt.Errorf("reading the mounts of %s: %w", c, err)
	}
	return mounts, nil
}
