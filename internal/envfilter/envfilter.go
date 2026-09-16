// Package envfilter decides which host environment variables are forwarded to containers.
package envfilter

import (
	"regexp"
	"slices"
	"strings"
)

// Values of these are host paths or host identity: meaningless, or wrong, inside a container.
var (
	DefaultDeny = []string{
		"PATH", "HOME", "PWD", "OLDPWD", "TMPDIR", "TMP", "TEMP",
		"USER", "LOGNAME", "HOSTNAME", "SHELL", "SHLVL", "_", "LS_COLORS",
	}
	DefaultDenyPrefixes = []string{
		"COMPOSE_", "DOCKER_", "SSH_", "XDG_", "WSL", "BASH_FUNC_",
	}
)

var nameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ValidName(name string) bool {
	return nameRe.MatchString(name)
}

type Rules struct {
	Deny         []string `yaml:"deny"`
	DenyPrefixes []string `yaml:"deny_prefixes"`
	Allow        []string `yaml:"allow"`
}

func (r Rules) Denied(name string) bool {
	if slices.Contains(r.Allow, name) {
		return false
	}
	if slices.Contains(r.Deny, name) {
		return true
	}
	for _, p := range r.DenyPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// Names returns the names of the entries of environ ("NAME=value") that should be forwarded.
func (r Rules) Names(environ []string) []string {
	var names []string
	seen := map[string]bool{}
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if seen[name] || !ValidName(name) || r.Denied(name) {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}
