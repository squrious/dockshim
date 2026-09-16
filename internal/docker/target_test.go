package docker

import (
	"io"
	"slices"
	"strings"
	"testing"
)

type call struct {
	dir  string
	args string
}

// fakeRunner answers "ps"/"inspect" according to running, and records calls.
type fakeRunner struct {
	running bool
	calls   []call
}

func (f *fakeRunner) Run(c Cmd) (int, error) {
	f.calls = append(f.calls, call{c.Dir, strings.Join(c.Args, " ")})
	switch {
	case slices.Contains(c.Args, "ps") && f.running:
		io.WriteString(c.Stdout, "abc123\n")
	case c.Args[0] == "inspect":
		if f.running {
			io.WriteString(c.Stdout, "true\n")
		} else {
			io.WriteString(c.Stdout, "false\n")
		}
	}
	return 0, nil
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
	if err := c.EnsureUp(io.Discard); err != nil {
		t.Fatal(err)
	}
	want := []call{
		{"/proj", "compose --file /proj/a.yaml --project-name p ps --status running --quiet tools"},
		{"/proj", "compose --file /proj/a.yaml --project-name p ps --status running --quiet tools"},
		{"/proj", "compose --file /proj/a.yaml --project-name p up --detach tools"},
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

func TestContainer(t *testing.T) {
	r := &fakeRunner{running: true}
	c := &Container{Runner: r, ProjectDir: "/proj", Name: "node"}
	if !c.IsRunning() {
		t.Fatal("should be running")
	}
	if err := c.EnsureUp(io.Discard); err != nil {
		t.Fatal(err)
	}
	if r.calls[1].args != "start node" {
		t.Fatalf("calls = %v", r.calls)
	}
	if got := strings.Join(c.ExecArgs(ExecOptions{}, []string{"node"}), " "); got != "exec --interactive node node" {
		t.Fatalf("exec args = %s", got)
	}
	if got := strings.Join(c.ExecArgs(ExecOptions{TTY: true, Workdir: "/w"}, []string{"node"}), " "); got != "exec --interactive --tty --workdir /w node node" {
		t.Fatalf("exec args = %s", got)
	}
}
