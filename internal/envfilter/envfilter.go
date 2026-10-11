// Package envfilter decides which host environment variables are forwarded to containers.
package envfilter

import (
	"slices"
	"strings"
)

// DefaultDeny lists the variables whose values are host paths or host identity: meaningless, or
// wrong, inside a container.
var DefaultDeny = []string{
	"PATH", "HOME", "PWD", "OLDPWD", "TMPDIR", "TMP", "TEMP",
	"USER", "LOGNAME", "HOSTNAME", "SHELL", "SHLVL", "_", "LS_COLORS",
	"COMPOSE_*", "DOCKER_*", "SSH_*", "XDG_*", "WSL*", "BASH_FUNC_*",
}

// ValidName reports whether name can be passed to docker exec --env. Docker accepts any other
// name, so a shell identifier is not required.
func ValidName(name string) bool {
	return name != "" && !strings.ContainsAny(name, "=\x00")
}

// match reports whether name matches pattern, where * matches any run of characters. A literal *
// in a name is matched by a * in the pattern, along with anything else.
func match(pattern, name string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == name
	}
	first, last := parts[0], parts[len(parts)-1]
	if len(name) < len(first)+len(last) || !strings.HasPrefix(name, first) || !strings.HasSuffix(name, last) {
		return false
	}
	// Leftmost matches of the middle parts leave the most room for the next ones.
	name = name[len(first) : len(name)-len(last)]
	for _, p := range parts[1 : len(parts)-1] {
		i := strings.Index(name, p)
		if i < 0 {
			return false
		}
		name = name[i+len(p):]
	}
	return true
}

// Rules select the host variables forwarded to the container. Their items are names, in which *
// matches any run of characters.
type Rules struct {
	Deny  []string
	Allow []string
}

// Denied reports whether name must not be forwarded. Allow wins over Deny.
func (r Rules) Denied(name string) bool {
	return !slices.ContainsFunc(r.Allow, func(p string) bool { return match(p, name) }) &&
		slices.ContainsFunc(r.Deny, func(p string) bool { return match(p, name) })
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
