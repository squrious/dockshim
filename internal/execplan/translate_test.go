package execplan

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/docker"
	"github.com/squrious/dockshim/internal/pathmap"
)

type tarEntry struct {
	name, link, content string
	uid, gid            int
	mode                int64
}

// recordingRunner records docker calls and decodes archives sent on stdin.
type recordingRunner struct {
	calls   []string
	entries []tarEntry
}

func (r *recordingRunner) Run(c docker.Cmd) (int, error) {
	r.calls = append(r.calls, strings.Join(c.Args, " "))
	if c.Stdin == nil {
		return 0, nil
	}
	tr := tar.NewReader(c.Stdin)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return 0, nil
		}
		if err != nil {
			return 1, err
		}
		b, _ := io.ReadAll(tr)
		r.entries = append(r.entries, tarEntry{h.Name, h.Linkname, string(b), h.Uid, h.Gid, h.Mode})
	}
}

type fixture struct {
	base, proj, outside string
	runner              *recordingRunner
	alias               *config.ResolvedAlias
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{
		base:    base,
		proj:    filepath.Join(base, "proj"),
		outside: filepath.Join(base, "outside"),
		runner:  &recordingRunner{},
	}
	for p, content := range map[string]string{
		"proj/src/in.php":    "in",
		"proj/assets/x.js":   "x",
		"outside/a.txt":      "A",
		"outside/dir/b.txt":  "B",
		"outside/dir/big.js": "big",
	} {
		p = filepath.Join(base, p)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(content), 0o640)
	}
	os.Symlink("b.txt", filepath.Join(f.outside, "dir", "link"))
	f.alias = &config.ResolvedAlias{
		Name:        "php",
		Container:   "ctr",
		User:        "1000:1001",
		PathMapping: pathmap.Map{
			{Host: f.proj, Container: "/app"},
			{Host: filepath.Join(f.proj, "assets"), Container: "/assets/build"},
		},
		PathTranslation: config.ResolvedPathTranslation{
			Enabled:   true,
			Exclude:   pathmap.DefaultCopyExclude,
			MaxCopyMB: 1,
		},
	}
	return f
}

func (f *fixture) build(t *testing.T, cwd string, args ...string) (*Plan, []string, string) {
	t.Helper()
	var stderr bytes.Buffer
	p, err := Build(Input{
		Project: &config.Project{Root: f.proj},
		Alias:   f.alias,
		Runner:  f.runner,
		Cwd:     cwd,
		Args:    append([]string{"php"}, args...),
		Stderr:  &stderr,
	})
	if err != nil {
		t.Fatal(err)
	}
	i := slices.Index(p.Args, "php")
	return p, p.Args[i+1:], stderr.String()
}

var tmpRe = regexp.MustCompile(`/tmp/dockshim-[0-9a-f]{16}/`)

func normalize(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = tmpRe.ReplaceAllString(a, "/tmp/X/")
	}
	return out
}

func TestTranslateArgs(t *testing.T) {
	f := newFixture(t)
	src := filepath.Join(f.proj, "src")
	tests := []struct {
		name string
		cwd  string
		args []string
		want []string
		copy bool
		warn string
	}{
		{name: "mapped absolute, even missing", cwd: src,
			args: []string{f.proj + "/src/in.php", f.proj + "/out.json"},
			want: []string{"/app/src/in.php", "/app/out.json"}},
		{name: "relative resolved by the working directory", cwd: src,
			args: []string{"in.php", "../src", "missing"},
			want: []string{"in.php", "../src", "missing"}},
		{name: "relative under another mapping", cwd: f.proj,
			args: []string{"src/in.php", "assets/x.js", "./assets/x.js"},
			want: []string{"src/in.php", "/assets/build/x.js", "/assets/build/x.js"}},
		{name: "relative from unmapped cwd into mapping", cwd: f.base,
			args: []string{"proj/src/in.php"},
			want: []string{"/app/src/in.php"}},
		{name: "options", cwd: src,
			args: []string{"-v", "--config=" + f.proj + "/c.yaml", "-", "--x=in.php", "-o=" + f.outside + "/a.txt"},
			want: []string{"-v", "--config=/app/c.yaml", "-", "--x=in.php", "-o=/tmp/X/0/a.txt"}, copy: true},
		{name: "outside copies, deduplicated", cwd: src,
			args: []string{f.outside + "/a.txt", "../../outside/dir/b.txt", f.outside + "/a.txt", "../../outside/missing"},
			want: []string{"/tmp/X/0/a.txt", "/tmp/X/1/b.txt", "/tmp/X/0/a.txt", "../../outside/missing"}, copy: true},
		{name: "host system files are host files", cwd: src,
			args: []string{"/usr/bin/env"},
			want: []string{"/tmp/X/0/env"}, copy: true},
		{name: "virtual filesystems stay in the container", cwd: src,
			args: []string{"/dev/null", "/proc/self/status"},
			want: []string{"/dev/null", "/proc/self/status"}},
		{name: "directories are not copied", cwd: src,
			args: []string{f.outside + "/dir"},
			want: []string{f.outside + "/dir"}, warn: "is a directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, args, stderr := f.build(t, tt.cwd, tt.args...)
			if got := normalize(args); !slices.Equal(got, tt.want) {
				t.Fatalf("args = %q, want %q", got, tt.want)
			}
			if copied := len(p.PreRun) > 0; copied != tt.copy || len(p.PostRun) != len(p.PreRun) {
				t.Fatalf("steps pre=%d post=%d, want copy=%v", len(p.PreRun), len(p.PostRun), tt.copy)
			}
			if tt.warn == "" && stderr != "" || tt.warn != "" && !strings.Contains(stderr, tt.warn) {
				t.Fatalf("stderr = %q, want %q", stderr, tt.warn)
			}
		})
	}

	t.Run("command name untouched", func(t *testing.T) {
		p, err := Build(Input{Project: &config.Project{}, Alias: f.alias, Cwd: f.outside, Args: []string{"a.txt"}})
		if err != nil || p.Args[len(p.Args)-1] != "a.txt" || len(p.PreRun) != 0 {
			t.Fatalf("args = %q, err = %v", p.Args, err)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		f := newFixture(t)
		f.alias.PathTranslation.Enabled = false
		_, args, _ := f.build(t, f.base, "proj/src/in.php", f.outside+"/a.txt")
		if !slices.Equal(args, []string{"proj/src/in.php", f.outside + "/a.txt"}) {
			t.Fatalf("args = %q", args)
		}
	})

	t.Run("configured exclusion", func(t *testing.T) {
		f := newFixture(t)
		f.alias.PathTranslation.Exclude = append(slices.Clone(pathmap.DefaultCopyExclude), f.outside+"/dir")
		p, args, stderr := f.build(t, f.base, "outside/dir/b.txt", "outside/a.txt")
		if got := normalize(args); !slices.Equal(got, []string{"outside/dir/b.txt", "/tmp/X/0/a.txt"}) || len(p.PreRun) != 1 {
			t.Fatalf("args = %q", got)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}
	})

	t.Run("too large", func(t *testing.T) {
		f := newFixture(t)
		os.WriteFile(filepath.Join(f.outside, "big"), make([]byte, 1<<20+1), 0o644)
		p, args, stderr := f.build(t, f.base, "outside/big", "outside/a.txt")
		if got := normalize(args); !slices.Equal(got, []string{"outside/big", "/tmp/X/0/a.txt"}) || len(p.PreRun) != 1 {
			t.Fatalf("args = %q", got)
		}
		if !strings.Contains(stderr, "not copying outside/big into the container: too large") {
			t.Fatalf("stderr = %s", stderr)
		}
	})

	t.Run("unreadable file", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("root reads everything")
		}
		f := newFixture(t)
		secret := filepath.Join(f.outside, "secret")
		os.WriteFile(secret, []byte("s"), 0o000)
		_, args, stderr := f.build(t, f.base, secret)
		if !slices.Equal(args, []string{secret}) || !strings.Contains(stderr, "permission denied") {
			t.Fatalf("args = %q, stderr = %q", args, stderr)
		}
	})

	t.Run("symlink keeps its own name", func(t *testing.T) {
		f := newFixture(t)
		os.Symlink(filepath.Join(f.outside, "a.txt"), filepath.Join(f.base, "link.txt"))
		os.Symlink(filepath.Join(f.proj, "src", "in.php"), filepath.Join(f.base, "mapped.php"))
		_, args, _ := f.build(t, f.base, "link.txt", "mapped.php", "broken")
		if got := normalize(args); !slices.Equal(got, []string{"/tmp/X/0/link.txt", "/app/src/in.php", "broken"}) {
			t.Fatalf("args = %q", got)
		}
	})
}

func TestCopySteps(t *testing.T) {
	f := newFixture(t)
	p, args, _ := f.build(t, f.base, "outside/dir/b.txt", "outside/a.txt")
	dir := strings.TrimSuffix(args[0], "/0/b.txt")
	name := strings.TrimPrefix(dir, "/tmp/")

	if err := runSteps(p.PreRun); err != nil {
		t.Fatal(err)
	}
	if err := runSteps(p.PostRun); err != nil {
		t.Fatal(err)
	}
	if want := []string{"cp --archive - ctr:/tmp", "exec --user 0 ctr rm -rf " + dir}; !slices.Equal(f.runner.calls, want) {
		t.Fatalf("calls = %q", f.runner.calls)
	}

	want := []tarEntry{
		{name: name + "/", mode: 0o700},
		{name: name + "/0/", mode: 0o700},
		{name: name + "/0/b.txt", content: "B", mode: 0o640},
		{name: name + "/1/", mode: 0o700},
		{name: name + "/1/a.txt", content: "A", mode: 0o640},
	}
	for i := range want {
		want[i].uid, want[i].gid = 1000, 1001
	}
	if !slices.Equal(f.runner.entries, want) {
		t.Fatalf("entries:\n%v\nwant:\n%v", f.runner.entries, want)
	}

	t.Run("unknown user gets readable root files", func(t *testing.T) {
		f := newFixture(t)
		f.alias.User = "www-data"
		p, _, _ := f.build(t, f.base, "outside/a.txt")
		if err := runSteps(p.PreRun); err != nil {
			t.Fatal(err)
		}
		last := f.runner.entries[len(f.runner.entries)-1]
		if last.uid != 0 || last.gid != 0 || last.mode != 0o644 || f.runner.entries[0].mode != 0o755 {
			t.Fatalf("entries = %v", f.runner.entries)
		}
	})

	t.Run("no cleanup when copy never started", func(t *testing.T) {
		f := newFixture(t)
		p, _, _ := f.build(t, f.base, "outside/a.txt")
		if err := runSteps(p.PostRun); err != nil || len(f.runner.calls) != 0 {
			t.Fatalf("calls = %q, err = %v", f.runner.calls, err)
		}
	})
}

func TestNumericUser(t *testing.T) {
	for in, want := range map[string][2]int{"1000": {1000, 0}, "1000:33": {1000, 33}, "www-data": {-1, -1}, "1000:www": {-1, -1}} {
		if uid, gid := numericUser(in); uid != want[0] || gid != want[1] {
			t.Errorf("numericUser(%q) = %d, %d", in, uid, gid)
		}
	}
}
