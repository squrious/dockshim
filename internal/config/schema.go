// Package config discovers, parses, validates and resolves dockshim configuration files.
package config

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v3"
)

type File struct {
	BinDir  string           `yaml:"bin_dir"`
	Compose *Compose         `yaml:"compose"`
	Global  Global           `yaml:"global"`
	Aliases map[string]Alias `yaml:"aliases"`
}

type Compose struct {
	Files       []string `yaml:"files"`
	ProjectName string   `yaml:"project_name"`
}

type Global struct {
	ShimMode        Scalar          `yaml:"shim_mode"`
	User            Scalar          `yaml:"user"`
	Env             Env             `yaml:"env"`
	PathTranslation PathTranslation `yaml:"path_translation"`
}

type PathTranslation struct {
	Enabled   Scalar   `yaml:"enabled"`
	Exclude   []string `yaml:"exclude"`
	MaxCopyMB Scalar   `yaml:"max_copy_mb"`
}

type Env struct {
	Deny         []string          `yaml:"deny"`
	DenyPrefixes []string          `yaml:"deny_prefixes"`
	Allow        []string          `yaml:"allow"`
	Vars         map[string]Scalar `yaml:"vars"`
}

type Alias struct {
	ShimMode        Scalar            `yaml:"shim_mode"`
	Service         string            `yaml:"service"`
	Container       string            `yaml:"container"`
	PathMapping     map[string]string `yaml:"path_mapping"`
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

// Int assumes a validated value, and returns def when empty.
func (s Scalar) Int(def int) int {
	if n, err := strconv.Atoi(string(s)); err == nil {
		return n
	}
	return def
}
