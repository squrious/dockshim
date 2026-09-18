// Package docker drives the docker CLI against compose services or plain containers.
package docker

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// Cmd is one docker CLI invocation.
type Cmd struct {
	Dir    string
	Args   []string
	Env    []string // nil means inherit
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Runner runs docker commands. Fakes implement it in tests.
type Runner interface {
	// Run returns the exit code of the command. err is set only when it could not run at all.
	Run(Cmd) (int, error)
}

// ExecRunner runs the docker binary found in PATH, or Binary when set.
type ExecRunner struct {
	Binary string
}

// Run behaves like a shell running docker: it returns 127 with an error when docker cannot start,
// 128+n when docker is killed by signal n, and otherwise docker's exit code.
// SIGTERM and SIGHUP are forwarded to docker; SIGINT and SIGQUIT only reach it from the terminal.
func (r ExecRunner) Run(c Cmd) (int, error) {
	bin := r.Binary
	if bin == "" {
		bin = "docker"
	}
	cmd := exec.Command(bin, c.Args...)
	cmd.Dir, cmd.Env = c.Dir, c.Env
	cmd.Stdin, cmd.Stdout, cmd.Stderr = c.Stdin, c.Stdout, c.Stderr

	// The terminal delivers SIGINT/SIGQUIT to the whole foreground group, docker included, so they
	// are caught and dropped here. Not ignored: docker would inherit the ignored disposition.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		return 127, err
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case s := <-sigs:
				if s == syscall.SIGTERM || s == syscall.SIGHUP {
					_ = cmd.Process.Signal(s)
				}
			case <-done:
				return
			}
		}
	}()

	err := cmd.Wait()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			return 128 + int(ws.Signal()), nil
		}
		return exitErr.ExitCode(), nil
	}
	if err != nil {
		return 1, err
	}
	return 0, nil
}
