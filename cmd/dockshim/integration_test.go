//go:build integration

package main

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
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

// baseConfig is the global section shared by the suite.
const baseConfig = `
global:
  env:
    deny: [HIDDEN]
    vars: {FROM_CONFIG: configured}
`

// project writes the config, installs the shims and returns the project root.
func project(t *testing.T, cfg string, files map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files = maps.Clone(files)
	files[".dockshim.yaml"] = baseConfig + cfg
	files["sub/.keep"] = ""
	for name, content := range files {
		p := filepath.Join(root, name)
		must(t, os.MkdirAll(filepath.Dir(p), 0o755))
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command(filepath.Join(binDir, "dockshim"), "install")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install: %v\n%s", err, out)
	}

	fi, err := os.Lstat(filepath.Join(root, ".dockshim", "bin", "sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !fi.Mode().IsRegular() || fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("shim should be an executable regular file, mode %v", fi.Mode())
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
	// Shims run dockshim from PATH: the binary under test comes first.
	cmd.Env = append(append(os.Environ(), "PATH="+binDir+string(filepath.ListSeparator)+os.Getenv("PATH")), env...)
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

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// commonChecks exercises an alias named sh whose project root is mapped to /app. stop stops the target, to check auto start;
// it runs last.
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
	t.Run("path translation", func(t *testing.T) {
		ext, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		must(t, os.MkdirAll(filepath.Join(ext, "dir"), 0o755))
		must(t, os.WriteFile(filepath.Join(ext, "file.txt"), []byte("external"), 0o600))
		must(t, os.WriteFile(filepath.Join(root, "mapped.txt"), []byte("mapped"), 0o644))
		must(t, os.Symlink(filepath.Join(ext, "file.txt"), filepath.Join(ext, "link.md")))
		// A link may not carry a copy out of an allowed directory.
		escape := filepath.Join(ext, "escape.md")
		must(t, os.Symlink("/etc/hostname", escape))

		// A copied file keeps its name and belongs to the container user; mapped paths are
		// rewritten; anything outside the allowed directories keeps its value.
		script := `cat "$1"; echo; stat -c %u:%g "$1"; basename "$1"; cat "$2"; echo; echo "$3"; echo "$4"; echo "$5"`
		r := shim(t, root, sub, "", nil, "sh", "-c", script, "_",
			filepath.Join(ext, "link.md"), filepath.Join(root, "mapped.txt"), "/etc/hostname", escape,
			"--in="+filepath.Join(root, "sub"))
		want := "external\n" + hostUser + "\nlink.md\nmapped\n/etc/hostname\n" + escape + "\n--in=/app/sub"
		expect(t, r, want, 0)

		// Directories are left to the container, with a warning.
		r = shim(t, root, sub, "", nil, "sh", "-c", `echo "$1"`, "_", filepath.Join(ext, "dir"))
		expect(t, r, filepath.Join(ext, "dir"), 0)
		if !strings.Contains(r.stderr, "is a directory") {
			t.Fatalf("stderr = %q", r.stderr)
		}

		// Copies are removed, on success and on failure.
		expect(t, shim(t, root, sub, "", nil, "sh", "-c", `ls /tmp | grep -c dockshim-`), "0", 1)
		failing := shim(t, root, sub, "", nil, "sh", "-c", `test -f "$1" && exit 7`, "_", filepath.Join(ext, "file.txt"))
		expect(t, failing, "", 7)
		expect(t, shim(t, root, sub, "", nil, "sh", "-c", `ls /tmp | grep -c dockshim-`), "0", 1)
	})
	t.Run("auto start", func(t *testing.T) {
		stop()
		expect(t, shim(t, root, sub, "", nil, "sh", "-c", "echo up"), "up", 0)
	})
}

func TestIntegrationCompose(t *testing.T) {
	// Mappings inferred from the compose volumes must match the explicit ones.
	for _, c := range []struct{ name, mapping string }{{"inferred", ""}, {"explicit", "path_mapping: {.: /app}"}} {
		t.Run(c.name, func(t *testing.T) {
			root := composeProject(t, fmt.Sprintf("dockshim-it-%d", time.Now().UnixNano()), c.mapping)
			t.Cleanup(func() {
				cmd := exec.Command("docker", "compose", "down", "--volumes", "--timeout", "0")
				cmd.Dir = root
				_ = cmd.Run()
			})

			commonChecks(t, root, func() { docker(t, root, "compose", "stop", "--timeout", "0") })
		})
	}
}

// composeProject mounts the project root at /app, and /etc/hostname, which is outside it.
func composeProject(t *testing.T, name, mapping string) string {
	return project(t, `
compose:
  project_name: `+name+`
aliases:
  sh:
    service: tools
    `+mapping+`
`, map[string]string{
		"compose.yaml": `
name: ` + name + `
services:
  tools:
    image: ` + image + `
    init: true
    command: [sleep, infinity]
    working_dir: /
    volumes: [".:/app", "/etc/hostname:/host-hostname:ro"]
`,
	})
}
