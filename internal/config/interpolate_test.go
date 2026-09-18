package config

import (
	"strings"
	"testing"
)

func env(vars map[string]string) LookupFunc {
	return func(name string) (string, bool) {
		v, ok := vars[name]
		return v, ok
	}
}

func TestInterpolate(t *testing.T) {
	lookup := env(map[string]string{"A": "a", "EMPTY": "", "B": "b"})
	tests := []struct{ in, want, err string }{
		{in: "plain", want: "plain"},
		{in: "$A-${A}", want: "a-a"},
		{in: "x${A}y$B.z", want: "xayb.z"},
		{in: "$$A $$ cost $5 end$", want: "$A $ cost $5 end$"},
		{in: "${EMPTY}", want: ""},
		{in: "${UNSET:-def}|${EMPTY:-def}|${A:-def}", want: "def|def|a"},
		{in: "${UNSET-def}|${EMPTY-def}", want: "def|"},
		{in: "${UNSET:-${B}/x}", want: "b/x"},
		{in: "${UNSET:-}", want: ""},
		{in: "${A:?boom}${EMPTY?boom}", want: "a"},
		{in: "${UNSET}", err: "variable UNSET is not set"},
		{in: "$UNSET", err: "variable UNSET is not set"},
		{in: "${UNSET:-$OTHER}", err: "variable OTHER is not set"},
		{in: "${EMPTY:?must be set}", err: "variable EMPTY: must be set"},
		{in: "${UNSET?}", err: "variable UNSET: is required"},
		{in: "${A", err: "unclosed ${"},
		{in: "${}", err: "invalid variable expression"},
		{in: "${1A}", err: "invalid variable expression"},
		{in: "${A+x}", err: "invalid variable expression"},
	}
	for _, tt := range tests {
		got, err := Interpolate(tt.in, lookup)
		if tt.err != "" {
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Errorf("Interpolate(%q) error = %v, want %q", tt.in, err, tt.err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("Interpolate(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestLookupEnviron(t *testing.T) {
	lookup := LookupEnviron([]string{"A=1", "AB=2", "EMPTY=", "A=last"})
	for name, want := range map[string]struct {
		v  string
		ok bool
	}{"A": {"last", true}, "AB": {"2", true}, "EMPTY": {"", true}, "B": {"", false}} {
		if v, ok := lookup(name); v != want.v || ok != want.ok {
			t.Errorf("lookup(%q) = %q, %v; want %q, %v", name, v, ok, want.v, want.ok)
		}
	}
}

func TestParseInterpolates(t *testing.T) {
	lookup := env(map[string]string{"UID": "1000", "SVC": "tools", "TRICKY": "a: [b", "DIR": "src"})
	f, err := Parse([]byte(`
global:
  user: ${UID}
  env:
    deny: [$UID]
    vars:
      TRICKY: $TRICKY
      LITERAL: $$HOME
      NUM: 42
aliases:
  php:
    service: ${SVC}
    path_mapping:
      ${DIR}: /app/${DIR}
`), lookup)
	if err != nil {
		t.Fatal(err)
	}
	php := f.Aliases["php"]
	switch {
	case f.Global.User != "1000", f.Global.Env.Deny[0] != "1000":
		t.Errorf("global = %+v", f.Global)
	case f.Global.Env.Vars["TRICKY"] != "a: [b", f.Global.Env.Vars["LITERAL"] != "$HOME", f.Global.Env.Vars["NUM"] != "42":
		t.Errorf("vars = %v", f.Global.Env.Vars)
	case php.Service != "tools", php.PathMapping["${DIR}"] != "/app/src":
		t.Errorf("php = %+v", php)
	}

	_, err = Parse([]byte("aliases:\n  php:\n    service: ${NOPE}\n"), lookup)
	if err == nil || !strings.Contains(err.Error(), "line 3: variable NOPE is not set") {
		t.Fatalf("got %v", err)
	}
	_, err = Parse([]byte("aliases:\n  php:\n    servce: ${NOPE}\n"), lookup)
	if err == nil || !strings.Contains(err.Error(), "line 3: field servce not found") {
		t.Fatalf("unknown fields must be reported with original lines first, got %v", err)
	}
}
