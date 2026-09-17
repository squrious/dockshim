package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// LookupFunc returns the value of an environment variable and whether it is set.
type LookupFunc func(name string) (string, bool)

// interpolateNode expands variables in every scalar value (not mapping keys) below n.
func interpolateNode(n *yaml.Node, lookup LookupFunc) error {
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, c := range n.Content {
			if err := interpolateNode(c, lookup); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		for i := 1; i < len(n.Content); i += 2 {
			if err := interpolateNode(n.Content[i], lookup); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		if !strings.Contains(n.Value, "$") {
			return nil
		}
		v, err := Interpolate(n.Value, lookup)
		if err != nil {
			return fmt.Errorf("line %d: %w", n.Line, err)
		}
		n.Value, n.Tag = v, "!!str"
	}
	return nil
}

// Interpolate expands $VAR, ${VAR}, ${VAR:-default}, ${VAR-default}, ${VAR:?error} and ${VAR?error}.
// $$ is a literal $. A variable that is unset and has no default is an error.
func Interpolate(s string, lookup LookupFunc) (string, error) {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] != '$' || i+1 == len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}
		switch next := s[i+1]; {
		case next == '$':
			b.WriteByte('$')
			i += 2
		case next == '{':
			end := closingBrace(s, i+2)
			if end < 0 {
				return "", fmt.Errorf("unclosed ${ in %q", s)
			}
			v, err := expand(s[i+2:end], lookup)
			if err != nil {
				return "", err
			}
			b.WriteString(v)
			i = end + 1
		case isNameStart(next):
			j := i + 1
			for j < len(s) && isNameChar(s[j]) {
				j++
			}
			v, err := expand(s[i+1:j], lookup)
			if err != nil {
				return "", err
			}
			b.WriteString(v)
			i = j
		default:
			b.WriteByte('$')
			i++
		}
	}
	return b.String(), nil
}

func closingBrace(s string, from int) int {
	depth := 1
	for i := from; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return -1
}

func expand(expr string, lookup LookupFunc) (string, error) {
	n := 0
	for n < len(expr) && isNameChar(expr[n]) {
		n++
	}
	name, rest := expr[:n], expr[n:]
	if name == "" || !isNameStart(name[0]) {
		return "", fmt.Errorf("invalid variable expression ${%s}", expr)
	}
	val, set := lookup(name)

	var op, arg string
	for _, candidate := range []string{":-", ":?", "-", "?"} {
		if a, ok := strings.CutPrefix(rest, candidate); ok {
			op, arg = candidate, a
			break
		}
	}
	switch {
	case rest == "":
		if !set {
			return "", fmt.Errorf("variable %s is not set (use ${%s:-} to allow it)", name, name)
		}
		return val, nil
	case op == "":
		return "", fmt.Errorf("invalid variable expression ${%s}", expr)
	}

	missing := !set || (op[0] == ':' && val == "")
	if !missing {
		return val, nil
	}
	if strings.HasSuffix(op, "?") {
		if arg == "" {
			arg = "is required"
		}
		return "", fmt.Errorf("variable %s: %s", name, arg)
	}
	return Interpolate(arg, lookup)
}

func isNameStart(c byte) bool {
	return c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

func isNameChar(c byte) bool {
	return isNameStart(c) || ('0' <= c && c <= '9')
}
