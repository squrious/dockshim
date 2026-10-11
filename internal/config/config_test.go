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
	"github.com/squrious/dockshim/internal/hostpath"
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
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}

		file, err := Discover(sub)
		if err != nil || file != filepath.Join(root, ".dockshim.yaml") {
			t.Fatalf("got %q, %v", file, err)
		}
	})

	t.Run("ambiguous", func(t *testing.T) {
		root := t.TempDir()
		write(t, filepath.Join(root, ".dockshim.yaml"), minimal)
		write(t, filepath.Join(root, ".dockshim.yml"), minimal)
		if _, err := Discover(root); err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("expected ambiguity error, got %v", err)
		}
	})

	t.Run("unreadable is not absent", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("root reads everything")
		}
		dir := t.TempDir()
		if err := os.Chmod(dir, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
		if _, err := Discover(dir); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := Discover(t.TempDir()); !errors.Is(err, ErrNotFound) {
			t.Fatalf("got %v", err)
		}
	})
}

func TestForShim(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, root string) (shim string)
		want    string // config file, relative to root
		wantIs  error
		wantErr string
	}{
		{"yaml", func(t *testing.T, root string) string {
			write(t, filepath.Join(root, ".dockshim.yaml"), minimal)
			return shimAt(t, root, "")
		}, ".dockshim.yaml", nil, ""},
		{"yml", func(t *testing.T, root string) string {
			write(t, filepath.Join(root, ".dockshim.yml"), minimal)
			return shimAt(t, root, "")
		}, ".dockshim.yml", nil, ""},
		{"link to a shim", func(t *testing.T, root string) string {
			write(t, filepath.Join(root, "proj", ".dockshim.yaml"), minimal)
			return linkTo(t, shimAt(t, root, "proj"), filepath.Join(root, "elsewhere", "php"))
		}, "proj/.dockshim.yaml", nil, ""},
		{"linked shims directory", func(t *testing.T, root string) string {
			write(t, filepath.Join(root, "proj", ".dockshim.yaml"), minimal)
			bin := linkTo(t, filepath.Dir(shimAt(t, root, "proj")), filepath.Join(root, "elsewhere", "bin"))
			return filepath.Join(bin, "php")
		}, "proj/.dockshim.yaml", nil, ""},
		{"config above the project only", func(t *testing.T, root string) string {
			write(t, filepath.Join(root, ".dockshim.yaml"), minimal)
			return shimAt(t, root, "proj")
		}, "", ErrNotFound, "proj: no dockshim config found"},
		{"outside the shims directory", func(t *testing.T, root string) string {
			write(t, filepath.Join(root, ".dockshim.yaml"), minimal)
			write(t, filepath.Join(root, "php"), "")
			return filepath.Join(root, "php")
		}, "", nil, "php is not in a .dockshim/bin directory"},
		{"below the shims directory", func(t *testing.T, root string) string {
			write(t, filepath.Join(root, ".dockshim.yaml"), minimal)
			write(t, filepath.Join(root, ".dockshim", "bin", "sub", "php"), "")
			return filepath.Join(root, ".dockshim", "bin", "sub", "php")
		}, "", nil, "is not in a .dockshim/bin directory"},
		{"ambiguous", func(t *testing.T, root string) string {
			write(t, filepath.Join(root, ".dockshim.yaml"), minimal)
			write(t, filepath.Join(root, ".dockshim.yml"), minimal)
			return shimAt(t, root, "")
		}, "", nil, "ambiguous config"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := hostpath.Real(t.TempDir())
			file, err := ForShim(tt.setup(t, root))
			switch {
			case tt.wantErr != "" || tt.wantIs != nil:
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) || (tt.wantIs != nil && !errors.Is(err, tt.wantIs)) {
					t.Fatalf("got %q, %v", file, err)
				}
			case err != nil || file != filepath.Join(root, tt.want):
				t.Fatalf("got %q, %v, want %s", file, err, tt.want)
			}
		})
	}
}

// linkTo creates link pointing to target, and returns it.
func linkTo(t *testing.T, target, link string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	return link
}

// shimAt creates a shim in the shims directory of the project at root/sub, and returns its path.
func shimAt(t *testing.T, root, sub string) string {
	t.Helper()
	p := filepath.Join(root, sub, RelBinDir, "php")
	write(t, p, "#!/bin/sh\n")
	return p
}

func TestAliasNames(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		want    []string
		wantErr string
	}{
		{"sorted", "aliases: {php: {service: a}, composer: {service: a}}\n", []string{"composer", "php"}, ""},
		{"values neither interpolated nor checked", "defaults:\n  user: ${APP_USER_ID}\naliases:\n  php: {user: not valid}\n", []string{"php"}, ""},
		{"tagged value only valid once interpolated", "aliases:\n  php:\n    service: !!int ${X}\n", []string{"php"}, ""},
		{"null aliases", "aliases:\n", nil, ""},
		{"anchors and merge keys", "aliases:\n  php: &base {service: a}\n  composer: *base\n  <<: {node: {service: b}}\n", []string{"composer", "node", "php"}, ""},
		{"empty", "# nothing yet\n", nil, ""},
		{"unknown key", "aliases: {php: {servce: a}}\n", nil, "aliases.php.servce: unknown key"},
		{"invalid name", "aliases: {dockshim: {service: a}}\n", nil, "aliases.dockshim: invalid alias name"},
		{"syntax", "aliases: [\n", nil, "yaml:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), ".dockshim.yaml")
			write(t, file, tt.config)
			got, err := AliasNames(file)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got %v, %v, want error %q", got, err, tt.wantErr)
				}
				return
			}
			if err != nil || !slices.Equal(got, tt.want) {
				t.Fatalf("got %v, %v, want %v", got, err, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []string // substrings of problems, in order
	}{
		{"valid", `
defaults: {user: "1000:1000", env: {D: 1}, env_forwarding: {deny: [A, B_*, "*"], allow: [C, AWS_*]}}
aliases:
  php: {service: tools, path_mapping: {.: /app, ./lib: /lib}, user: host}
  node: {service: node, user: www-data}
`, nil},
		{"no aliases", `defaults: {}`, nil},
		{"service required", `aliases: {php: {}}`, []string{"aliases.php.service: is required"}},
		{"bad names", `aliases: {dockshim: {service: a}, Dockshim: {service: a}, "a/b": {service: a}}`, []string{
			`aliases.Dockshim: invalid alias name`,
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
defaults: {user: "a b", env: {"B=C": x, "B-C": ok}, env_forwarding: {deny: [1X, "A=B", "*_TOKEN"]}}
aliases: {php: {service: a, env_forwarding: {allow: [""], deny: ["A*B"]}}}
`, []string{
			`defaults.user: invalid user "a b"`,
			`defaults.env: invalid variable name "B=C"`,
			`defaults.env_forwarding.deny[1]: invalid variable name "A=B"`,
			`aliases.php.env_forwarding.allow[0]: invalid variable name ""`,
		}},
		{"path translation", `
defaults: {path_translation: {enabled: maybe, allow: [rel]}}
aliases: {php: {service: a, path_translation: {enabled: "${X:-false}", allow: [/ok, 'C:\tmp']}}}
`, []string{
			`defaults.path_translation.enabled: invalid boolean "maybe"`,
			`defaults.path_translation.allow[0]: host path "rel" must be absolute`,
		}},
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

func TestParseStructure(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []string
	}{
		{"empty", "", nil},
		{"comments only", "# nothing\n", nil},
		{"nulls", "compose:\ndefaults: {user: ~}\naliases: {php: {service: a, env: {A: null}}}", nil},
		{"merge keys", `
aliases:
  php: &php {service: a, user: "1"}
  node: {<<: *php, user: "2"}
  js: {<<: [*php], env: {<<: {A: b}}}
`, nil},
		{"unknown keys", `
global: 1
aliases:
  php: {servce: a, env_forwarding: {forward: [A]}}
`, []string{
			`global: unknown key, expected one of compose, defaults, aliases (line 2)`,
			`aliases.php.servce: unknown key, expected one of service, path_mapping, user, env, env_forwarding, path_translation (line 4)`,
			`aliases.php.env_forwarding.forward: unknown key, expected one of deny, allow (line 4)`,
		}},
		{"unknown key in merged mapping", `
aliases:
  php: &php {service: a, servce: b}
  node: {<<: *php}
`, []string{
			`aliases.php.servce: unknown key, expected one of service, path_mapping, user, env, env_forwarding, path_translation (line 3)`,
			`aliases.node.servce: unknown key, expected one of service, path_mapping, user, env, env_forwarding, path_translation (line 3)`,
		}},
		{"wrong kinds", `
compose: {files: a.yaml}
defaults: {user: [1], env: {A: {b: c}, ? [B] : x}, env_forwarding: {deny: A}}
aliases:
  php: {service: a, path_mapping: {.: [x]}, path_translation: {allow: [[x]]}}
  node: []
`, []string{
			`compose.files: expected a list, got "a.yaml" (line 2)`,
			`defaults.user: expected a single value, got a list (line 3)`,
			`defaults.env[A]: expected a single value, got a mapping (line 3)`,
			`defaults.env: expected a single value as key, got a list (line 3)`,
			`defaults.env_forwarding.deny: expected a list, got "A" (line 3)`,
			`aliases.php.path_mapping[.]: expected a single value, got a list (line 5)`,
			`aliases.php.path_translation.allow[0]: expected a single value, got a list (line 5)`,
			`aliases.node: expected a mapping, got a list (line 6)`,
		}},
		{"anchor cycles", `
aliases: &all
  php: &php {service: a, <<: *php}
  node: *all
`, []string{
			`aliases.php: anchor "php" contains itself (line 3)`,
			`aliases.node: anchor "all" contains itself (line 2)`,
		}},
		{"root", "- a", []string{`top level: expected a mapping, got a list (line 1)`}},
		{"duplicate keys", `
aliases:
  php: {service: a}
  php: {service: b}
  node: {service: a, service: b}
`, []string{
			`aliases.php: duplicate key, first at line 3 (line 4)`,
			`aliases.node.service: duplicate key, first at line 5 (line 5)`,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml), noEnv)
			if tt.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("expected ValidationError, got %v", err)
			}
			if !slices.Equal(verr.Problems, tt.want) {
				t.Fatalf("got problems:\n%s", strings.Join(verr.Problems, "\n"))
			}
		})
	}
}

func TestResolve(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, ".dockshim.yaml")
	write(t, file, `
compose: {files: [docker/compose.yaml], project_name: proj}
defaults:
  user: 1000
  env: {SOME_VAR: defaults, ONLY_DEFAULTS: true}
  env_forwarding:
    deny: [FOO, MISE_*]
aliases:
  php:
    service: tools
    path_mapping: {.: /app}
  inferred:
    service: tools
  unmapped:
    service: tools
    path_mapping: {}
  node:
    service: node
    path_mapping: {assets: /assets/build, ./lib: /app/lib}
    user: 1001
    env: {NODE_ENV: dev, SOME_VAR: alias}
    env_forwarding:
      deny: [BAZ]
      allow: [SSH_AUTH_SOCK]
`)
	p, err := Load(file, noEnv)
	if err != nil {
		t.Fatal(err)
	}
	root, _ = filepath.EvalSymlinks(root)

	if p.Root != root {
		t.Errorf("root = %q", p.Root)
	}
	if p.Compose.Files[0] != filepath.Join(root, "docker/compose.yaml") || p.Compose.ProjectName != "proj" {
		t.Errorf("compose = %+v", p.Compose)
	}

	php, node := p.Aliases["php"], p.Aliases["node"]
	if php.User != "1000" || node.User != "1001" {
		t.Errorf("users: php=%q node=%q", php.User, node.User)
	}
	// Only a missing path_mapping is inferred: an empty one maps nothing.
	if inf, un := p.Aliases["inferred"], p.Aliases["unmapped"]; php.InferPathMapping || !inf.InferPathMapping || un.InferPathMapping || len(un.PathMapping) != 0 {
		t.Errorf("infer: php=%v inferred=%v unmapped=%v", php.InferPathMapping, inf.InferPathMapping, un.InferPathMapping)
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
	if php.Env["SOME_VAR"] != "defaults" || node.Env["SOME_VAR"] != "alias" || node.Env["ONLY_DEFAULTS"] != "true" || node.Env["NODE_ENV"] != "dev" {
		t.Errorf("env: php=%v node=%v", php.Env, node.Env)
	}
	wantDeny := append(slices.Clone(envfilter.DefaultDeny), "FOO", "MISE_*", "BAZ")
	if !slices.Equal(node.EnvForwarding.Deny, wantDeny) || !slices.Equal(node.EnvForwarding.Allow, []string{"SSH_AUTH_SOCK"}) {
		t.Errorf("node env_forwarding = %+v", node.EnvForwarding)
	}
	if !slices.Contains(php.EnvForwarding.Deny, "MISE_*") || slices.Contains(php.EnvForwarding.Deny, "BAZ") {
		t.Errorf("php env_forwarding = %+v", php.EnvForwarding)
	}
}

func TestResolveDefaults(t *testing.T) {
	f, err := Parse([]byte(minimal), noEnv)
	if err != nil {
		t.Fatal(err)
	}
	p := f.Resolve(filepath.Join(t.TempDir(), ".dockshim.yaml"))
	want := strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
	if got := p.Aliases["php"].User; got != want {
		t.Fatalf("user = %q, want %q", got, want)
	}
	pt := p.Aliases["php"].PathTranslation
	if !pt.Enabled || len(pt.Allow) != 0 {
		t.Fatalf("path translation defaults = %+v", pt)
	}
}
