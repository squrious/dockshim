package envfilter

import (
	"slices"
	"testing"
)

func TestDefaultsDoNotDenyMise(t *testing.T) {
	r := Rules{Deny: DefaultDeny, DenyPrefixes: DefaultDenyPrefixes}
	if r.Denied("MISE_ENV") {
		t.Fatal("MISE_ vars must not be denied by default")
	}
}

func TestNames(t *testing.T) {
	r := Rules{
		Deny:         append(slices.Clone(DefaultDeny), "FOO"),
		DenyPrefixes: DefaultDenyPrefixes,
		Allow:        []string{"SSH_AUTH_SOCK"},
	}
	environ := []string{
		"PATH=/bin", "HOME=/home/me", "FOO=1", "BAR=2", "DOCKER_HOST=x",
		"SSH_AUTH_SOCK=/s", "SSH_AGENT_PID=1", "WSLENV=x", "BASH_FUNC_f%%=()",
		"1BAD=x", "EMPTY=", "BAR=dup", "_=/usr/bin/env", "lower_case=ok",
	}
	got := r.Names(environ)
	want := []string{"BAR", "SSH_AUTH_SOCK", "EMPTY", "lower_case"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
