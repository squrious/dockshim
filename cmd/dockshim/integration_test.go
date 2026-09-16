//go:build integration

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const image = "alpine:latest"

func docker(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("docker", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("docker %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// project writes the config, installs shims and returns the project root.
func project(t *testing.T, config string, files map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files[".dockshim.yaml"] = config
	files["sub/.keep"] = ""
	for name, content := range files {
		p := filepath.Join(root, name)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(filepath.Join(binDir, "dockshim"), "install")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}
	return root
}

type result struct {
	stdout, stderr string
	code           int
}

func shim(t *testing.T, root, dir, stdin string, env []string, alias string, args ...string) result {
	t.Helper()
	cmd := exec.Command(filepath.Join(root, ".dockshim", "bin", alias), args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = strings.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatal(err)
	}
	return result{strings.TrimSpace(stdout.String()), stderr.String(), cmd.ProcessState.ExitCode()}
}

func expect(t *testing.T, r result, stdout string, code int) {
	t.Helper()
	if r.stdout != stdout || r.code != code {
		t.Fatalf("got stdout %q code %d, want %q code %d\nstderr: %s", r.stdout, r.code, stdout, code, r.stderr)
	}
}

// commonChecks exercises an alias named sh mapped to /app.
func commonChecks(t *testing.T, root string, stop func()) {
	sub := filepath.Join(root, "sub")
	hostUser := strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())

	t.Run("workdir", func(t *testing.T) {
		expect(t, shim(t, root, sub, "", nil, "sh", "-c", "pwd"), "/app/sub", 0)
		expect(t, shim(t, root, "/", "", nil, "sh", "-c", "pwd"), "/", 0)
	})
	t.Run("stdin", func(t *testing.T) {
		expect(t, shim(t, root, sub, "piped\n", nil, "sh", "-c", "cat"), "piped", 0)
	})
	t.Run("exit code", func(t *testing.T) {
		expect(t, shim(t, root, sub, "", nil, "sh", "-c", "exit 42"), "", 42)
	})
	t.Run("env", func(t *testing.T) {
		env := []string{"DOCKSHIM_IT=forwarded", "HIDDEN=x"}
		expect(t, shim(t, root, sub, "", env, "sh", "-c", `echo "$DOCKSHIM_IT $FROM_CONFIG ${HIDDEN:-none} $HOME"`), "forwarded configured none /", 0)
	})
	t.Run("user defaults to host", func(t *testing.T) {
		expect(t, shim(t, root, sub, "", nil, "sh", "-c", `echo "$(id -u):$(id -g)"`), hostUser, 0)
	})
	t.Run("files are shared", func(t *testing.T) {
		expect(t, shim(t, root, sub, "", nil, "sh", "-c", "echo hi > out.txt"), "", 0)
		if b, _ := os.ReadFile(filepath.Join(sub, "out.txt")); string(b) != "hi\n" {
			t.Fatalf("out.txt = %q", b)
		}
	})
	t.Run("auto start", func(t *testing.T) {
		stop()
		expect(t, shim(t, root, sub, "", nil, "sh", "-c", "echo up"), "up", 0)
	})
}

const envConfig = `
global:
  env:
    deny: [HIDDEN]
    vars: {FROM_CONFIG: configured}
`

func TestIntegrationContainer(t *testing.T) {
	name := fmt.Sprintf("dockshim-it-%d", time.Now().UnixNano())
	root := project(t, envConfig+`
aliases:
  sh:
    container: `+name+`
    path_mapping: {.: /app}
`, map[string]string{})
	docker(t, root, "run", "--detach", "--init", "--name", name, "--workdir", "/", "--volume", root+":/app", image, "sleep", "infinity")
	t.Cleanup(func() { exec.Command("docker", "rm", "--force", name).Run() })

	commonChecks(t, root, func() { docker(t, root, "stop", "--time", "0", name) })
}

func TestIntegrationCompose(t *testing.T) {
	root := composeProject(t, fmt.Sprintf("dockshim-it-%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		cmd := exec.Command("docker", "compose", "down", "--volumes", "--timeout", "0")
		cmd.Dir = root
		cmd.Run()
	})

	commonChecks(t, root, func() { docker(t, root, "compose", "stop", "--timeout", "0") })
}

func composeProject(t *testing.T, name string) string {
	return project(t, envConfig+`
compose:
  project_name: `+name+`
aliases:
  sh:
    service: tools
    path_mapping: {.: /app}
`, map[string]string{
		"compose.yaml": `
name: ` + name + `
services:
  tools:
    image: ` + image + `
    init: true
    command: [sleep, infinity]
    working_dir: /
    volumes: [".:/app"]
`,
	})
}
