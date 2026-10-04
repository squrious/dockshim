// Package cli implements the dockshim commands, including alias mode (`run --shim`, called by shims).
package cli

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/docker"
	"github.com/squrious/dockshim/internal/hostpath"
	"github.com/squrious/dockshim/internal/style"
)

// Env holds everything the CLI reads from or writes to the outside world.
type Env struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Environ        []string
	Getwd          func() (string, error)
	Runner         docker.Runner
	// TTY is true when the container command should get a pseudo-terminal (stdin and stdout are terminals).
	TTY bool
	// OutStyle and ErrStyle color dockshim's own output on Stdout and Stderr.
	OutStyle, ErrStyle style.Style
}

// SystemEnv returns the Env of the running process.
func SystemEnv() *Env {
	stdin, stdout, stderr := isTerm(os.Stdin), isTerm(os.Stdout), isTerm(os.Stderr)
	// https://no-color.org
	color := os.Getenv("NO_COLOR") == ""
	return &Env{
		Stdin:    os.Stdin,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
		Environ:  os.Environ(),
		Getwd:    os.Getwd,
		Runner:   docker.ExecRunner{},
		TTY:      stdin && stdout,
		OutStyle: style.Style{Enabled: color && stdout},
		ErrStyle: style.Style{Enabled: color && stderr},
	}
}

func isTerm(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// Main runs dockshim with its argv and returns the process exit code.
func Main(args []string, e *Env) int {
	return runManager(args[1:], e)
}

func (e *Env) errorf(format string, args ...any) {
	_, _ = fmt.Fprintf(e.Stderr, "%s %s\n", e.ErrStyle.Error(config.ToolName+":"), fmt.Sprintf(format, args...))
}

func (e *Env) warnf(format string, args ...any) {
	_, _ = fmt.Fprintf(e.Stderr, "%s: %s %s\n", config.ToolName, e.ErrStyle.Warning("warning:"), fmt.Sprintf(format, args...))
}

// cwd returns the real working directory, so that it compares equal to resolved config paths.
func (e *Env) cwd() (string, error) {
	wd, err := e.Getwd()
	if err != nil {
		return "", err
	}
	return hostpath.Real(wd), nil
}
