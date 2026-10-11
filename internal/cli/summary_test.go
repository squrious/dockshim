package cli

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/envfilter"
	"github.com/squrious/dockshim/internal/pathmap"
	"github.com/squrious/dockshim/internal/style"
)

func TestMappingLines(t *testing.T) {
	inferred := &config.ResolvedAlias{Service: "tools", InferPathMapping: true}
	m := pathmap.Map{{Host: "/proj", Container: "/app"}, {Host: "/proj/src", Container: "/code"}}
	tests := []struct {
		name  string
		alias *config.ResolvedAlias
		inf   *inference
		want  []string
	}{
		{"configured", &config.ResolvedAlias{PathMapping: m}, nil, []string{". → /app", "src → /code"}},
		{"configured empty", &config.ResolvedAlias{}, nil, []string{"none"}},
		{"unknown", inferred, nil, []string{"inferred from the container's bind mounts"}},
		{"running", inferred, &inference{running: true, mappings: m, outside: []string{"/var/run/docker.sock"}}, []string{
			"inferred from the running container", ". → /app", "src → /code", "not mapped, outside the project: /var/run/docker.sock",
		}},
		{"compose", inferred, &inference{mappings: m[:1]}, []string{"inferred from the compose config, tools isn't running", ". → /app"}},
		{"nothing inside", inferred, &inference{running: true}, []string{"inferred from the running container", "no bind mount inside the project"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mappingLines(style.Style{}, "/proj", tt.alias, func(*config.ResolvedAlias) *inference { return tt.inf })
			if !slices.Equal(got, tt.want) {
				t.Fatalf("got %q", got)
			}
		})
	}
}

func TestEnvLines(t *testing.T) {
	tests := []struct {
		name  string
		alias *config.ResolvedAlias
		env   []string
		fwd   []string
	}{
		{"none", &config.ResolvedAlias{EnvForwarding: envfilter.Rules{Deny: envfilter.DefaultDeny}}, nil, nil},
		{"all", &config.ResolvedAlias{
			Env:           map[string]string{"B": "secret", "A": "1"},
			EnvForwarding: envfilter.Rules{Deny: append(slices.Clone(envfilter.DefaultDeny), "FOO", "MISE_*"), Allow: []string{"SSH_AUTH_SOCK"}},
		}, []string{"A, B"}, []string{"allow SSH_AUTH_SOCK", "deny FOO, MISE_*"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := envLines(tt.alias); !slices.Equal(got, tt.env) {
				t.Errorf("env = %q, want %q", got, tt.env)
			}
			if got := forwardingLines(tt.alias); !slices.Equal(got, tt.fwd) {
				t.Errorf("env_forwarding = %q, want %q", got, tt.fwd)
			}
		})
	}
}

// Styled keys keep their columns aligned: they are padded before styling.
func TestPrintSummaryStyled(t *testing.T) {
	p := &config.Project{Root: "/proj", File: "/proj/.dockshim.yaml", Aliases: map[string]*config.ResolvedAlias{
		"php": {Service: "tools", User: "1000", PathMapping: pathmap.Map{{Host: "/proj", Container: "/app"}}},
	}}
	var plain, styled strings.Builder
	must(t, printSummary(&plain, style.Style{}, p, nil))
	must(t, printSummary(&styled, style.Style{Enabled: true}, p, nil))
	if !strings.Contains(styled.String(), "\x1b[1mphp\x1b[0m\n") || !strings.Contains(styled.String(), "  \x1b[36mservice\x1b[0m") {
		t.Fatalf("styled = %q", styled.String())
	}
	if got := ansiRe.ReplaceAllString(styled.String(), ""); got != plain.String() {
		t.Fatalf("stripped = %q, want %q", got, plain.String())
	}
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)
