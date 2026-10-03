package cli

import (
	"reflect"
	"regexp"
	"testing"

	"github.com/squrious/dockshim/internal/config"
)

// The template promises that its commented values are the defaults.
func TestInitTemplateDefaults(t *testing.T) {
	defaults := regexp.MustCompile(`(?m)^# ([a-z_]+:.*|  .*)$`)
	uncommented := defaults.ReplaceAll(initTemplate, []byte("$1"))
	if string(uncommented) == string(initTemplate) {
		t.Fatal("no default uncommented")
	}

	resolve := func(data []byte) *config.Project {
		t.Helper()
		data = append(data, "  x:\n    service: s\n"...)
		f, err := config.Parse(data, config.LookupEnviron(nil))
		if err != nil {
			t.Fatalf("%v\n%s", err, data)
		}
		if err := f.Validate(); err != nil {
			t.Fatal(err)
		}
		return f.Resolve("/project/.dockshim.yaml")
	}
	want, got := resolve(initTemplate), resolve(uncommented)
	if !reflect.DeepEqual(got.Aliases, want.Aliases) {
		t.Errorf("aliases = %+v, want %+v", got.Aliases["x"], want.Aliases["x"])
	}
	if got.Compose == nil || len(got.Compose.Files) > 0 || got.Compose.ProjectName != "" {
		t.Errorf("compose = %+v, want no options", got.Compose)
	}
}
