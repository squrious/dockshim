package cli

import (
	"slices"
	"testing"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/pathmap"
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
			got := mappingLines("/proj", tt.alias, func(*config.ResolvedAlias) *inference { return tt.inf })
			if !slices.Equal(got, tt.want) {
				t.Fatalf("got %q", got)
			}
		})
	}
}
