package config

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// checkStructure reports the keys and value kinds of doc that do not fit File, by field path, so
// that errors name config keys rather than Go types. Values themselves are left to Validate.
func checkStructure(doc *yaml.Node) []string {
	c := checker{open: map[*yaml.Node]bool{}}
	c.check(doc, reflect.TypeFor[File](), "")
	return c.problems
}

type checker struct {
	problems []string
	// The mappings being checked: reaching one again through an alias is a cycle.
	open map[*yaml.Node]bool
}

func (c *checker) add(field string, n *yaml.Node, format string, args ...any) {
	c.problems = append(c.problems, fmt.Sprintf("%s: %s (line %d)", cmp.Or(field, "top level"), fmt.Sprintf(format, args...), n.Line))
}

func (c *checker) check(n *yaml.Node, t reflect.Type, field string) {
	for n.Kind == yaml.DocumentNode || n.Kind == yaml.AliasNode {
		if n.Kind == yaml.AliasNode {
			n = n.Alias
		} else if len(n.Content) == 0 {
			return
		} else {
			n = n.Content[0]
		}
	}
	if n.Kind == yaml.ScalarNode && n.ShortTag() == "!!null" {
		return
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Struct, reflect.Map:
		if c.expect(n, yaml.MappingNode, field) {
			c.checkMapping(n, t, field)
		}
	case reflect.Slice:
		if c.expect(n, yaml.SequenceNode, field) {
			for i, e := range n.Content {
				c.check(e, t.Elem(), fmt.Sprintf("%s[%d]", field, i))
			}
		}
	case reflect.String:
		c.expect(n, yaml.ScalarNode, field)
	}
}

func (c *checker) checkMapping(n *yaml.Node, t reflect.Type, field string) {
	if c.open[n] {
		c.add(field, n, "anchor %q contains itself", n.Anchor)
		return
	}
	c.open[n] = true
	defer delete(c.open, n)

	seen := map[string]int{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i], n.Content[i+1]
		// Merged keys may be overridden: check the merged mappings on their own.
		if k.ShortTag() == "!!merge" {
			if v.Kind == yaml.SequenceNode {
				for _, m := range v.Content {
					c.check(m, t, field)
				}
			} else {
				c.check(v, t, field)
			}
			continue
		}
		if k.Kind != yaml.ScalarNode {
			c.add(field, k, "expected a single value as key, got %s", describe(k))
			continue
		}

		sub := k.Value
		switch {
		case t.Kind() == reflect.Map && t.Elem().Kind() != reflect.Struct:
			sub = field + "[" + k.Value + "]"
		case field != "":
			sub = field + "." + k.Value
		}
		if first, ok := seen[k.Value]; ok {
			c.add(sub, k, "duplicate key, first at line %d", first)
			continue
		}
		seen[k.Value] = k.Line

		if t.Kind() == reflect.Map {
			c.check(v, t.Elem(), sub)
			continue
		}
		keys := structKeys(t)
		idx := slices.Index(keys, k.Value)
		if idx < 0 {
			c.add(sub, k, "unknown key, expected one of %s", strings.Join(keys, ", "))
			continue
		}
		c.check(v, t.Field(idx).Type, sub)
	}
}

// structKeys returns the YAML key of each field of t, in field order.
func structKeys(t reflect.Type) []string {
	keys := make([]string, t.NumField())
	for i := range keys {
		keys[i], _, _ = strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
	}
	return keys
}

func (c *checker) expect(n *yaml.Node, kind yaml.Kind, field string) bool {
	if n.Kind == kind {
		return true
	}
	c.add(field, n, "expected %s, got %s", kindNames[kind], describe(n))
	return false
}

var kindNames = map[yaml.Kind]string{
	yaml.MappingNode:  "a mapping",
	yaml.SequenceNode: "a list",
	yaml.ScalarNode:   "a single value",
}

func describe(n *yaml.Node) string {
	if n.Kind == yaml.ScalarNode {
		return fmt.Sprintf("%q", n.Value)
	}
	return kindNames[n.Kind]
}
