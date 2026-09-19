// Package config discovers, parses, validates and resolves dockshim configuration files.
package config

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

// File is a config file as written, after interpolation. Load turns it into a Project.
type File struct {
	BinDir  string           `yaml:"bin_dir"`
	Compose *Compose         `yaml:"compose"`
	Global  Global           `yaml:"global"`
	Aliases map[string]Alias `yaml:"aliases"`
}

// Compose holds the options passed to every docker compose call. Files are relative to the project root.
type Compose struct {
	Files       []string `yaml:"files"`
	ProjectName string   `yaml:"project_name"`
}

// Global holds the defaults every alias inherits.
type Global struct {
	ShimMode        Scalar          `yaml:"shim_mode"`
	User            Scalar          `yaml:"user"`
	Env             Env             `yaml:"env"`
	PathTranslation PathTranslation `yaml:"path_translation"`
}

// PathTranslation configures how host paths given as arguments are made usable in the container.
type PathTranslation struct {
	Enabled        Scalar   `yaml:"enabled"`
	Allow          []string `yaml:"allow"`
	FollowSymlinks Scalar   `yaml:"follow_symlinks"`
	MaxCopyMB      Scalar   `yaml:"max_copy_mb"`
}

// Env configures which host variables are forwarded, and which are set.
type Env struct {
	Deny         []string          `yaml:"deny"`
	DenyPrefixes []string          `yaml:"deny_prefixes"`
	Allow        []string          `yaml:"allow"`
	Vars         map[string]Scalar `yaml:"vars"`
}

// Alias is one entry point. Its settings override or extend Global.
type Alias struct {
	ShimMode        Scalar            `yaml:"shim_mode"`
	Service         string            `yaml:"service"`
	PathMapping     map[string]string `yaml:"path_mapping"` // nil: inferred from the container's bind mounts
	User            Scalar            `yaml:"user"`
	Env             Env               `yaml:"env"`
	PathTranslation PathTranslation   `yaml:"path_translation"`
}

// Scalar accepts any YAML scalar (string, int, bool...) as its literal text.
type Scalar string

func (s *Scalar) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("line %d: expected a scalar value", n.Line)
	}
	if n.Tag == "!!null" {
		*s = ""
		return nil
	}
	*s = Scalar(n.Value)
	return nil
}

// Bool assumes a validated value.
func (s Scalar) Bool() bool {
	b, _ := strconv.ParseBool(string(s))
	return b
}

// Int assumes a validated value, and returns def when it is empty or not an integer.
func (s Scalar) Int(def int) int {
	if n, err := strconv.Atoi(string(s)); err == nil {
		return n
	}
	return def
}
