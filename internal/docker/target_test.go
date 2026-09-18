package docker

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
)

type call struct {
	dir  string
	args string
	env  string
}

// fakeRunner answers "ps"/"inspect" according to running, and records calls.
// With fail set, every call exits with it.
type fakeRunner struct {
	running bool
	fail    int
	calls   []call
}

func (f *fakeRunner) Run(c Cmd) (int, error) {
	f.calls = append(f.calls, call{c.Dir, strings.Join(c.Args, " "), strings.Join(c.Env, " ")})
	if f.fail != 0 {
		return f.fail, nil
	}
	var out string
	switch {
	case slices.Contains(c.Args, "ps") && f.running:
		out = "abc123\n"
	case c.Args[0] == "inspect":
		out = fmt.Sprintf("%v\n", f.running)
	}
	_, err := io.WriteString(c.Stdout, out)
	return 0, err
}

func TestCompose(t *testing.T) {
	r := &fakeRunner{}
	c := &Compose{Runner: r, ProjectDir: "/proj", Files: []string{"/proj/a.yaml"}, ProjectName: "p", Service: "tools"}

	if c.IsRunning() {
		t.Fatal("should not be running")
	}
	r.running = true
	if !c.IsRunning() {
		t.Fatal("should be running")
	}
	if err := c.EnsureUp([]string{"APP_ENV=dev"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	want := []call{
		{"/proj", "compose --file /proj/a.yaml --project-name p ps --status running --quiet tools", ""},
		{"/proj", "compose --file /proj/a.yaml --project-name p ps --status running --quiet tools", ""},
		{"/proj", "compose --file /proj/a.yaml --project-name p up --detach tools", "APP_ENV=dev COMPOSE_PROGRESS=quiet"},
	}
	if !slices.Equal(r.calls, want) {
		t.Fatalf("calls = %v", r.calls)
	}

	opts := ExecOptions{User: "1000:1000", Workdir: "/app/sub", Env: []string{"A", "B"}}
	got := strings.Join(c.ExecArgs(opts, []string{"php", "-v"}), " ")
	if got != "compose --file /proj/a.yaml --project-name p exec -T --env A --env B --user 1000:1000 --workdir /app/sub tools php -v" {
		t.Fatalf("exec args = %s", got)
	}
	opts = ExecOptions{TTY: true}
	if got := strings.Join((&Compose{Service: "s"}).ExecArgs(opts, []string{"x"}), " "); got != "compose exec s x" {
		t.Fatalf("tty exec args = %s", got)
	}
}

func TestContainerID(t *testing.T) {
	r := &fakeRunner{}
	c := &Compose{Runner: r, Service: "tools"}
	if _, err := c.ContainerID(); err == nil {
		t.Fatal("expected error when not running")
	}
	r.running = true
	if id, err := c.ContainerID(); id != "abc123" || err != nil {
		t.Fatalf("id=%q err=%v", id, err)
	}
	// A failing docker is reported as such, not as a stopped service.
	r.fail = 125
	if _, err := c.ContainerID(); err == nil || !strings.Contains(err.Error(), "exit status 125") {
		t.Fatalf("err=%v", err)
	}
}

func TestContainer(t *testing.T) {
	r := &fakeRunner{running: true}
	c := &Container{Runner: r, ProjectDir: "/proj", Name: "node"}
	if !c.IsRunning() {
		t.Fatal("should be running")
	}
	if err := c.EnsureUp([]string{"APP_ENV=dev"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	want := []call{
		{"/proj", "inspect --format {{.State.Running}} node", ""},
		{"/proj", "start node", "APP_ENV=dev"},
	}
	if !slices.Equal(r.calls, want) {
		t.Fatalf("calls = %v", r.calls)
	}
	r.running = false
	if c.IsRunning() {
		t.Fatal("should not be running")
	}
	if got := strings.Join(c.ExecArgs(ExecOptions{}, []string{"node"}), " "); got != "exec --interactive node node" {
		t.Fatalf("exec args = %s", got)
	}
	if got := strings.Join(c.ExecArgs(ExecOptions{TTY: true, Workdir: "/w"}, []string{"node"}), " "); got != "exec --interactive --tty --workdir /w node node" {
		t.Fatalf("exec args = %s", got)
	}
}
