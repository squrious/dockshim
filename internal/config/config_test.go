package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/squrious/dockshim/internal/envfilter"
	"github.com/squrious/dockshim/internal/pathmap"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func noEnv(string) (string, bool) { return "", false }

const minimal = "aliases: {php: {service: tools}}\n"

func TestDiscover(t *testing.T) {
	t.Run("root file from a subdirectory", func(t *testing.T) {
		root := t.TempDir()
		write(t, filepath.Join(root, ".dockshim.yaml"), minimal)
		sub := filepath.Join(root, "a", "b")
		os.MkdirAll(sub, 0o755)

		file, err := Discover(sub)
		if err != nil || file != filepath.Join(root, ".dockshim.yaml") {
			t.Fatalf("got %q, %v", file, err)
		}
		if RootOf(file) != root {
			t.Fatalf("root = %q", RootOf(file))
		}
	})

	t.Run("file in .dockshim dir", func(t *testing.T) {
		root := t.TempDir()
		write(t, filepath.Join(root, ".dockshim", "config.yml"), minimal)
		file, err := Discover(root)
		if err != nil || RootOf(file) != root {
			t.Fatalf("got %q, %v", file, err)
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		root := t.TempDir()
		write(t, filepath.Join(root, ".dockshim.yaml"), minimal)
		write(t, filepath.Join(root, ".dockshim", "config.yaml"), minimal)
		if _, err := Discover(root); err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("expected ambiguity error, got %v", err)
		}
	})

	t.Run("falls back to next start", func(t *testing.T) {
		empty, root := t.TempDir(), t.TempDir()
		write(t, filepath.Join(root, ".dockshim.yaml"), minimal)
		file, err := Discover(empty, root)
		if err != nil || RootOf(file) != root {
			t.Fatalf("got %q, %v", file, err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := Discover(t.TempDir()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("got %v", err)
		}
	})
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []string // substrings of problems, in order
	}{
		{"valid", `
global: {user: "1000:1000", env: {deny: [A], deny_prefixes: [B_], allow: [C], vars: {D: 1}}}
aliases:
  php: {service: tools, path_mapping: {.: /app, ./lib: /lib}, user: host}
  node: {container: node, user: www-data}
`, nil},
		{"no aliases", `global: {}`, nil},
		{"target required", `aliases: {php: {}}`, []string{"aliases.php: one of service or container is required"}},
		{"target exclusive", `aliases: {php: {service: a, container: b}}`, []string{"aliases.php: service and container are mutually exclusive"}},
		{"bad names", `aliases: {dockshim: {service: a}, "a/b": {service: a}}`, []string{
			`aliases.a/b: invalid alias name`,
			`aliases.dockshim: invalid alias name`,
		}},
		{"path mapping", `
aliases:
  php:
    service: a
    path_mapping: {/abs: /x, ../up: /y, lib: rel, ./lib: /z}
`, []string{
			"aliases.php.path_mapping[../up]: host path must be inside the project directory",
			"aliases.php.path_mapping[/abs]: host path must be inside the project directory",
			`aliases.php.path_mapping[lib]: duplicates "./lib"`,
			`aliases.php.path_mapping[lib]: container path "rel" must be absolute`,
		}},
		{"env and user", `
global: {user: "a b", env: {deny: [1X], vars: {"B-C": x}}}
aliases: {php: {service: a, env: {deny_prefixes: [""]}}}
`, []string{
			`global.user: invalid user "a b"`,
			`global.env.deny[0]: invalid variable name "1X"`,
			`global.env.vars: invalid variable name "B-C"`,
			`aliases.php.env.deny_prefixes[0]: invalid variable name ""`,
		}},
		{"compose without service", `
compose: {files: [c.yaml]}
aliases: {php: {container: a}}
`, []string{"compose: set but no alias uses a service"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse([]byte(tt.yaml), noEnv)
			if err != nil {
				t.Fatal(err)
			}
			err = f.Validate()
			var verr *ValidationError
			if tt.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.As(err, &verr) {
				t.Fatalf("expected ValidationError, got %v", err)
			}
			if len(verr.Problems) != len(tt.want) {
				t.Fatalf("got problems:\n%s", strings.Join(verr.Problems, "\n"))
			}
			for i, w := range tt.want {
				if !strings.HasPrefix(verr.Problems[i], w) {
					t.Errorf("problem %d = %q, want prefix %q", i, verr.Problems[i], w)
				}
			}
		})
	}
}

func TestParseRejectsUnknownFields(t *testing.T) {
	if _, err := Parse([]byte("aliases: {php: {servce: a}}"), noEnv); err == nil || !strings.Contains(err.Error(), "servce") {
		t.Fatalf("got %v", err)
	}
	if _, err := Parse([]byte("global: {user: [1]}"), noEnv); err == nil {
		t.Fatal("expected error for non-scalar user")
	}
}

func TestResolve(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, ".dockshim.yaml")
	write(t, file, `
bin_dir: tools/bin
compose: {files: [docker/compose.yaml], project_name: proj}
global:
  user: 1000
  env:
    deny: [FOO]
    deny_prefixes: [MISE_]
    vars: {SOME_VAR: global, ONLY_GLOBAL: true}
aliases:
  php:
    service: tools
    path_mapping: {.: /app}
  node:
    container: node
    path_mapping: {assets: /assets/build, ./lib: /app/lib}
    user: 1001
    env:
      vars: {NODE_ENV: dev, SOME_VAR: alias}
      deny: [BAZ]
      allow: [SSH_AUTH_SOCK]
`)
	p, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	root, _ = filepath.EvalSymlinks(root)

	if p.Root != root || p.BinDir != filepath.Join(root, "tools/bin") {
		t.Errorf("root=%q bin=%q", p.Root, p.BinDir)
	}
	if p.Compose.Files[0] != filepath.Join(root, "docker/compose.yaml") || p.Compose.ProjectName != "proj" {
		t.Errorf("compose = %+v", p.Compose)
	}

	php, node := p.Aliases["php"], p.Aliases["node"]
	if php.User != "1000" || node.User != "1001" {
		t.Errorf("users: php=%q node=%q", php.User, node.User)
	}
	if !slices.Equal(php.PathMapping, pathmap.Map{{Host: root, Container: "/app"}}) {
		t.Errorf("php mapping = %v", php.PathMapping)
	}
	wantNodeMap := pathmap.Map{
		{Host: filepath.Join(root, "lib"), Container: "/app/lib"},
		{Host: filepath.Join(root, "assets"), Container: "/assets/build"},
	}
	if !slices.Equal(node.PathMapping, wantNodeMap) {
		t.Errorf("node mapping = %v", node.PathMapping)
	}
	if php.Vars["SOME_VAR"] != "global" || node.Vars["SOME_VAR"] != "alias" || node.Vars["ONLY_GLOBAL"] != "true" || node.Vars["NODE_ENV"] != "dev" {
		t.Errorf("vars: php=%v node=%v", php.Vars, node.Vars)
	}
	wantDeny := append(slices.Clone(envfilter.DefaultDeny), "FOO", "BAZ")
	if !slices.Equal(node.Env.Deny, wantDeny) || !slices.Equal(node.Env.Allow, []string{"SSH_AUTH_SOCK"}) {
		t.Errorf("node env = %+v", node.Env)
	}
	if !slices.Contains(php.Env.DenyPrefixes, "MISE_") || slices.Contains(php.Env.Deny, "BAZ") {
		t.Errorf("php env = %+v", php.Env)
	}
}

func TestResolveUserDefaultsToHost(t *testing.T) {
	f, _ := Parse([]byte(minimal), noEnv)
	p := f.Resolve(filepath.Join(t.TempDir(), ".dockshim.yaml"))
	want := strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
	if got := p.Aliases["php"].User; got != want {
		t.Fatalf("user = %q, want %q", got, want)
	}
	if p.BinDir != filepath.Join(p.Root, ".dockshim", "bin") {
		t.Fatalf("bin_dir = %q", p.BinDir)
	}
}
