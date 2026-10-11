package envfilter

import (
	"slices"
	"testing"
)

func TestDefaultsDoNotDenyMise(t *testing.T) {
	r := Rules{Deny: DefaultDeny}
	if r.Denied("MISE_ENV") {
		t.Fatal("MISE_ vars must not be denied by default")
	}
}

func TestValidName(t *testing.T) {
	for n, want := range map[string]bool{
		"FOO": true, "lower": true, "1X": true, "A-B": true, "A.B": true, "P(x86)": true, "*_TOKEN": true, "A B": true,
		"": false, "A=B": false, "=": false, "A\x00B": false,
	} {
		if got := ValidName(n); got != want {
			t.Errorf("ValidName(%q) = %v, want %v", n, got, want)
		}
	}
}

func TestDenied(t *testing.T) {
	tests := []struct {
		name  string
		rules Rules
		env   string
		want  bool
	}{
		{"exact deny", Rules{Deny: []string{"FOO"}}, "FOO", true},
		{"exact does not match a longer name", Rules{Deny: []string{"FOO"}}, "FOOBAR", false},
		{"prefix deny", Rules{Deny: []string{"X_*"}}, "X_Y", true},
		{"prefix matches itself", Rules{Deny: []string{"X_*"}}, "X_", true},
		{"prefix does not match another", Rules{Deny: []string{"X_*"}}, "XY", false},
		{"star denies all", Rules{Deny: []string{"*"}}, "ANY", true},
		{"double star denies all", Rules{Deny: []string{"**"}}, "ANY", true},
		{"suffix deny", Rules{Deny: []string{"*_TOKEN"}}, "GITHUB_TOKEN", true},
		{"suffix does not match a prefix", Rules{Deny: []string{"*_TOKEN"}}, "TOKEN_X", false},
		{"middle deny", Rules{Deny: []string{"A_*_B"}}, "A_X_B", true},
		{"middle matches nothing between", Rules{Deny: []string{"A_*_B"}}, "A__B", true},
		{"prefix and suffix don't overlap", Rules{Deny: []string{"AB*BA"}}, "ABA", false},
		{"several stars in order", Rules{Deny: []string{"*A*B*"}}, "xAyBz", true},
		{"several stars out of order", Rules{Deny: []string{"*A*B*"}}, "xByAz", false},
		{"star matches a literal star", Rules{Deny: []string{"A*B"}}, "A*B", true},
		{"star matches other characters", Rules{Deny: []string{"A*B"}}, "AxB", true},
		{"stars must match in order", Rules{Deny: []string{"A*A*A"}}, "AA", false},
		{"surrounding stars need each A", Rules{Deny: []string{"*A*A*"}}, "A", false},
		{"surrounding stars match two As", Rules{Deny: []string{"*A*A*"}}, "AA", true},
		{"adjacent stars act as one", Rules{Deny: []string{"A**B"}}, "AB", true},
		{"allow wins over exact deny", Rules{Deny: []string{"FOO"}, Allow: []string{"FOO"}}, "FOO", false},
		{"allow wins over prefix deny", Rules{Deny: []string{"SSH_*"}, Allow: []string{"SSH_AUTH_SOCK"}}, "SSH_AUTH_SOCK", false},
		{"allow prefix", Rules{Deny: []string{"*"}, Allow: []string{"AWS_*"}}, "AWS_REGION", false},
		{"allowlist mode denies the rest", Rules{Deny: []string{"*"}, Allow: []string{"AWS_*"}}, "FOO", true},
		{"nothing denied", Rules{}, "FOO", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.rules.Denied(tt.env); got != tt.want {
				t.Fatalf("Denied(%q) = %v, want %v", tt.env, got, tt.want)
			}
		})
	}
}

func TestNames(t *testing.T) {
	r := Rules{
		Deny:  append(slices.Clone(DefaultDeny), "FOO"),
		Allow: []string{"SSH_AUTH_SOCK"},
	}
	environ := []string{
		"PATH=/bin", "HOME=/home/me", "FOO=1", "BAR=2", "DOCKER_HOST=x",
		"SSH_AUTH_SOCK=/s", "SSH_AGENT_PID=1", "WSLENV=x", "BASH_FUNC_f%%=()",
		"1X=x", "A.B=x", "EMPTY=", "BAR=dup", "_=/usr/bin/env", "lower_case=ok", "=C:=C:\\",
	}
	got := r.Names(environ)
	want := []string{"BAR", "SSH_AUTH_SOCK", "1X", "A.B", "EMPTY", "lower_case"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
