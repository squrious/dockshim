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
	"github.com/squrious/dockshim/internal/hostpath"
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
		b, err := io.ReadAll(tr)
		if err != nil {
			return 1, err
		}
		r.entries = append(r.entries, tarEntry{h.Name, h.Linkname, string(b), h.Uid, h.Gid, h.Mode})
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

type fixture struct {
	base, proj, tmp, other, drive string
	runner                        *recordingRunner
	alias                         *config.ResolvedAlias
}

const testDistro = "Ubuntu-24.04"

func newFixture(t *testing.T) *fixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{
		base:   base,
		proj:   filepath.Join(base, "proj"),
		tmp:    filepath.Join(base, "tmp"),   // stands for the system temporary directory
		other:  filepath.Join(base, "other"), // not allowed
		drive:  filepath.Join(base, "drive_c"),
		runner: &recordingRunner{},
	}
	winTmp := "drive_c/Users/me/AppData/Local/Temp"
	for p, content := range map[string]string{
		"proj/src/in.php":                      "in",
		"proj/assets/x.js":                     "x",
		"tmp/a.txt":                            "A",
		"tmp/dir/b.txt":                        "B",
		"other/hidden.txt":                     "H",
		winTmp + "/ide-phpunit.php":            "ide",
		"drive_c/Users/me/Documents/notes.php": "notes",
	} {
		p = filepath.Join(base, p)
		must(t, os.MkdirAll(filepath.Dir(p), 0o755))
		must(t, os.WriteFile(p, []byte(content), 0o640))
	}
	must(t, os.Symlink("b.txt", filepath.Join(f.tmp, "dir", "link")))
	f.alias = &config.ResolvedAlias{
		Service: "tools",
		User:    "1000:1001",
		PathMapping: pathmap.Map{
			{Host: f.proj, Container: "/app"},
			{Host: filepath.Join(f.proj, "assets"), Container: "/assets/build"},
		},
		PathTranslation: config.ResolvedPathTranslation{Enabled: true, MaxCopyMB: 1},
	}
	return f
}

// mountinfo exposes the fixture's drive_c directory as the Windows C: drive.
func (f *fixture) mountinfo() string {
	return "26 1 8:1 / / rw,relatime - ext4 /dev/sda1 rw\n" +
		`131 82 0:69 / ` + strings.ReplaceAll(f.drive, " ", `\040`) +
		` rw,noatime - 9p C:\134 rw,aname=drvfs;path=C:\;uid=1000` + "\n"
}

// win writes a host path the way a Windows tool would spell it.
func (f *fixture) win(p string) string {
	if rest, ok := strings.CutPrefix(p, f.drive+"/"); ok {
		return `C:\` + strings.ReplaceAll(rest, "/", `\`)
	}
	return `\\wsl.localhost\` + testDistro + strings.ReplaceAll(p, "/", `\`)
}

// winSlash is the same UNC path spelled with forward slashes, as an IDE may also write it.
func (f *fixture) winSlash(p string) string {
	return "//wsl.localhost/" + testDistro + p
}

func (f *fixture) build(t *testing.T, cwd string, args ...string) (*Plan, []string, string) {
	t.Helper()
	var stderr bytes.Buffer
	host := hostpath.New(hostpath.Env{
		Lookup:    func(k string) (string, bool) { return testDistro, k == "WSL_DISTRO_NAME" },
		Mountinfo: strings.NewReader(f.mountinfo()),
		TempDirs:  []string{f.tmp},
	}, hostOptions(f.alias))
	p, err := Build(Input{
		Project:   &config.Project{Root: f.proj},
		Alias:     f.alias,
		Runner:    f.runner,
		Target:    &fakeTarget{},
		Cwd:       cwd,
		Args:      append([]string{"php"}, args...),
		Stderr:    &stderr,
		HostPaths: host,
	})
	if err != nil {
		t.Fatal(err)
	}
	i := slices.Index(p.Args, "php")
	return p, p.Args[i+1:], stderr.String()
}

var tmpRe = regexp.MustCompile(`/tmp/dockshim-[A-Z2-7]{26}/`)

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
			args: []string{"-v", "--config=" + f.proj + "/c.yaml", "-", "--x=in.php", "-o=" + f.tmp + "/a.txt"},
			want: []string{"-v", "--config=/app/c.yaml", "-", "--x=in.php", "-o=/tmp/X/0/a.txt"}, copy: true},
		{name: "temporary files copy, deduplicated", cwd: src,
			args: []string{f.tmp + "/a.txt", "../../tmp/dir/b.txt", f.tmp + "/a.txt", "../../tmp/missing"},
			want: []string{"/tmp/X/0/a.txt", "/tmp/X/1/b.txt", "/tmp/X/0/a.txt", "../../tmp/missing"}, copy: true},
		{name: "files outside the allowed paths are the container's own", cwd: src,
			args: []string{f.other + "/hidden.txt", "/usr/bin/env", "/dev/null", "/proc/self/status"},
			want: []string{f.other + "/hidden.txt", "/usr/bin/env", "/dev/null", "/proc/self/status"}},
		{name: "directories are not copied", cwd: src,
			args: []string{f.tmp + "/dir"},
			want: []string{f.tmp + "/dir"}, warn: "is a directory"},
		{name: "windows temporary file copies", cwd: src,
			args: []string{f.win(f.drive + "/Users/me/AppData/Local/Temp/ide-phpunit.php")},
			want: []string{"/tmp/X/0/ide-phpunit.php"}, copy: true},
		{name: "unc path under a mapping is rewritten", cwd: src,
			args: []string{f.win(f.proj + "/src/in.php"), f.win(f.proj + "/src")},
			want: []string{"/app/src/in.php", "/app/src"}},
		// An IDE spells UNC paths both ways, sometimes in the same command line.
		{name: "unc path with forward slashes", cwd: src,
			args: []string{f.winSlash(f.proj + "/src/in.php"), f.win(f.proj + "/src")},
			want: []string{"/app/src/in.php", "/app/src"}},
		{name: "an ordinary path with a doubled slash is not a unc path", cwd: src,
			args: []string{"//etc/hostname"},
			want: []string{"//etc/hostname"}},
		{name: "windows path outside the allowed paths warns", cwd: src,
			args: []string{f.win(f.drive + "/Users/me/Documents/notes.php")},
			want: []string{f.win(f.drive + "/Users/me/Documents/notes.php")},
			warn: "is outside the allowed paths, see path_translation.allow"},
		{name: "unreachable windows path warns", cwd: src,
			args: []string{`Z:\build\x.php`, `\\wsl.localhost\Debian\home\me\x.php`, "//wsl.localhost/Debian/home/me/x.php", `\\server\share\x.php`},
			want: []string{`Z:\build\x.php`, `\\wsl.localhost\Debian\home\me\x.php`, "//wsl.localhost/Debian/home/me/x.php", `\\server\share\x.php`},
			warn: "is a Windows path this distribution cannot reach"},
		{name: "a backslashed argument is not a path", cwd: src,
			args: []string{`/(Squrious\\Tests\\RoutingTest::testIt)( .*)?$/`, "--test-suffix", "RoutingTest.php"},
			want: []string{`/(Squrious\\Tests\\RoutingTest::testIt)( .*)?$/`, "--test-suffix", "RoutingTest.php"}},
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
		f := newFixture(t)
		// A copyable file named like the command: only the argument is copied.
		must(t, os.WriteFile(filepath.Join(f.tmp, "php"), []byte("php"), 0o644))
		p, args, _ := f.build(t, f.tmp, "php")
		if got := normalize(args); !slices.Equal(got, []string{"/tmp/X/0/php"}) {
			t.Fatalf("args = %q", got)
		}
		if p.Args[len(p.Args)-2] != "php" {
			t.Fatalf("the command name was translated: %q", p.Args)
		}
	})

	t.Run("disabled", func(t *testing.T) {
		f := newFixture(t)
		f.alias.PathTranslation.Enabled = false
		_, args, _ := f.build(t, f.base, "proj/src/in.php", f.tmp+"/a.txt")
		if !slices.Equal(args, []string{"proj/src/in.php", f.tmp + "/a.txt"}) {
			t.Fatalf("args = %q", args)
		}
	})

	t.Run("configured allow", func(t *testing.T) {
		f := newFixture(t)
		f.alias.PathTranslation.Allow = []string{f.other}
		p, args, stderr := f.build(t, f.base, "other/hidden.txt", "tmp/a.txt")
		if got := normalize(args); !slices.Equal(got, []string{"/tmp/X/0/hidden.txt", "/tmp/X/1/a.txt"}) || len(p.PreRun) != 1 {
			t.Fatalf("args = %q", got)
		}
		if stderr != "" {
			t.Fatalf("stderr = %q", stderr)
		}
	})

	t.Run("too large", func(t *testing.T) {
		f := newFixture(t)
		must(t, os.WriteFile(filepath.Join(f.tmp, "big"), make([]byte, 1<<20+1), 0o644))
		p, args, stderr := f.build(t, f.base, "tmp/big", "tmp/a.txt")
		if got := normalize(args); !slices.Equal(got, []string{"tmp/big", "/tmp/X/0/a.txt"}) || len(p.PreRun) != 1 {
			t.Fatalf("args = %q", got)
		}
		if !strings.Contains(stderr, "not copying tmp/big into the container: too large") {
			t.Fatalf("stderr = %s", stderr)
		}
	})

	t.Run("unreadable file", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("root reads everything")
		}
		f := newFixture(t)
		secret := filepath.Join(f.tmp, "secret")
		must(t, os.WriteFile(secret, []byte("s"), 0o000))
		_, args, stderr := f.build(t, f.base, secret)
		if !slices.Equal(args, []string{secret}) || !strings.Contains(stderr, "permission denied") {
			t.Fatalf("args = %q, stderr = %q", args, stderr)
		}
	})

	t.Run("symlink keeps its own name", func(t *testing.T) {
		f := newFixture(t)
		must(t, os.Symlink(filepath.Join(f.tmp, "a.txt"), filepath.Join(f.tmp, "link.txt")))
		must(t, os.Symlink(filepath.Join(f.proj, "src", "in.php"), filepath.Join(f.tmp, "mapped.php")))
		_, args, _ := f.build(t, f.tmp, "link.txt", "mapped.php", "broken")
		if got := normalize(args); !slices.Equal(got, []string{"/tmp/X/0/link.txt", "/app/src/in.php", "broken"}) {
			t.Fatalf("args = %q", got)
		}
	})

	t.Run("symlink out of an allowed path", func(t *testing.T) {
		for _, follow := range []bool{false, true} {
			f := newFixture(t)
			f.alias.PathTranslation.FollowSymlinks = follow
			must(t, os.Symlink(filepath.Join(f.other, "hidden.txt"), filepath.Join(f.tmp, "escape.txt")))
			_, args, stderr := f.build(t, f.tmp, "escape.txt")
			want := []string{"escape.txt"}
			if follow {
				want = []string{"/tmp/X/0/escape.txt"}
			}
			if got := normalize(args); !slices.Equal(got, want) {
				t.Fatalf("follow_symlinks=%v: args = %q, want %q", follow, got, want)
			}
			if stderr != "" {
				t.Fatalf("stderr = %q", stderr)
			}
		}
	})
}

func TestCopySteps(t *testing.T) {
	f := newFixture(t)
	p, args, _ := f.build(t, f.base, "tmp/dir/b.txt", "tmp/a.txt")
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
		p, _, _ := f.build(t, f.base, "tmp/a.txt")
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
		p, _, _ := f.build(t, f.base, "tmp/a.txt")
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
