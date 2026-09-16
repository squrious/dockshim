package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

// binDir holds the dockshim binary under test. It is a real binary (not testscript's
// in-process command) because shims dispatch on argv[0].
var binDir string

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	dir, err := os.MkdirTemp("", "dockshim-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	binDir = dir

	build := exec.Command("go", "build", "-o", filepath.Join(dir, "dockshim"), ".")
	build.Stdout, build.Stderr = os.Stderr, os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "building dockshim:", err)
		return 1
	}
	return m.Run()
}

// fakeDocker records calls in $FAKE_DOCKER_STATE/calls and emulates the subcommands dockshim uses.
// A target is running when $FAKE_DOCKER_STATE/running exists.
// Creating $FAKE_DOCKER_STATE/stop-on-exec makes the next exec stop the target and fail.
// exec prints what it received, and exits with $FAKE_EXEC_EXIT.
// With $FAKE_EXEC_SLEEP, exec creates $FAKE_DOCKER_STATE/sleeping then sleeps.
const fakeDocker = `#!/bin/sh
state=${FAKE_DOCKER_STATE:?}
printf '%s\n' "$*" >> "$state/calls"
if [ "$1" = compose ]; then
  shift
  while :; do case "$1" in --file|--project-name) shift 2;; *) break;; esac; done
fi
sub=$1; shift
case "$sub" in
  ps) [ -e "$state/running" ] && echo abc123; exit 0;;
  inspect) if [ -e "$state/running" ]; then echo true; else echo false; fi; exit 0;;
  up|start) touch "$state/running"; exit 0;;
  exec)
    [ -e "$state/running" ] || { echo "fake: not running" >&2; exit 1; }
    if [ -e "$state/stop-on-exec" ]; then rm -f "$state/stop-on-exec" "$state/running"; exit 1; fi
    echo "dir $PWD"
    while [ $# -gt 0 ]; do
      case "$1" in
        --env) eval "v=\${$2-<unset>}"; echo "env $2=$v"; shift 2;;
        --user|--workdir) echo "$1 $2"; shift 2;;
        -T|--interactive|--tty) echo "flag $1"; shift;;
        *) break;;
      esac
    done
    echo "target $1"; shift
    echo "cmd $*"
    [ "$1" = cat ] && /bin/cat
    if [ -n "$FAKE_EXEC_SLEEP" ]; then touch "$state/sleeping"; exec /bin/sleep "$FAKE_EXEC_SLEEP"; fi
    exit "${FAKE_EXEC_EXIT:-0}";;
esac
echo "fake: unsupported $sub" >&2
exit 99
`

func TestScripts(t *testing.T) {
	fakeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeDir, "docker"), []byte(fakeDocker), 0o755); err != nil {
		t.Fatal(err)
	}
	testscript.Run(t, testscript.Params{
		Dir: "testdata/script",
		Setup: func(env *testscript.Env) error {
			state := filepath.Join(env.WorkDir, ".docker-state")
			env.Setenv("FAKE_DOCKER_STATE", state)
			env.Setenv("PATH", binDir+string(filepath.ListSeparator)+fakeDir+string(filepath.ListSeparator)+env.Getenv("PATH"))
			return os.MkdirAll(state, 0o755)
		},
	})
}
