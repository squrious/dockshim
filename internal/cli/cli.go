// Package cli dispatches between alias mode (invoked through a shim) and the dockshim manager commands.
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/docker"
	"github.com/squrious/dockshim/internal/hostpath"
)

// Env holds everything the CLI reads from or writes to the outside world.
type Env struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Environ        []string
	Getwd          func() (string, error)
	// Executable is the real path of the dockshim binary, which symlink shims point at. Empty when unknown.
	Executable string
	Runner     docker.Runner
	Prompter   Prompter
	// Interactive is true when a human can answer prompts (stdin and stderr are terminals).
	Interactive bool
	// TTY is true when the container command should get a pseudo-terminal (stdin and stdout are terminals).
	TTY bool
}

// Prompter asks the user a yes/no question.
type Prompter interface {
	Confirm(question string) (bool, error)
}

// SystemEnv returns the Env of the running process.
func SystemEnv() *Env {
	exe, _ := os.Executable()
	if exe != "" {
		exe = hostpath.Real(exe)
	}
	stdin, stdout, stderr := isTerm(os.Stdin), isTerm(os.Stdout), isTerm(os.Stderr)
	return &Env{
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Environ:     os.Environ(),
		Getwd:       os.Getwd,
		Executable:  exe,
		Runner:      docker.ExecRunner{},
		Prompter:    ttyPrompter{},
		Interactive: stdin && stderr,
		TTY:         stdin && stdout,
	}
}

func isTerm(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}

// ttyPrompter talks to /dev/tty so that stdin, meant for the aliased command, is left untouched.
type ttyPrompter struct{}

func (ttyPrompter) Confirm(question string) (bool, error) {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false, err
	}
	defer func() { _ = tty.Close() }()
	if _, err := fmt.Fprintf(tty, "%s [y/N] ", question); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(tty).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// Main runs dockshim and returns the process exit code.
func Main(args []string, e *Env) int {
	if filepath.Base(args[0]) == config.ToolName {
		return runManager(args[1:], e)
	}
	return runAlias(args[0], args[1:], e)
}

func (e *Env) errorf(format string, args ...any) {
	_, _ = fmt.Fprintf(e.Stderr, config.ToolName+": "+format+"\n", args...)
}

// cwd returns the real working directory, so that it compares equal to resolved config paths.
func (e *Env) cwd() (string, error) {
	wd, err := e.Getwd()
	if err != nil {
		return "", err
	}
	return hostpath.Real(wd), nil
}
