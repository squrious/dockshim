// Package config discovers, parses, validates and resolves dockshim configuration files.
package config

import (
	"fmt"

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
	User Scalar `yaml:"user"`
	Env  Env    `yaml:"env"`
}

type Env struct {
	Deny         []string          `yaml:"deny"`
	DenyPrefixes []string          `yaml:"deny_prefixes"`
	Allow        []string          `yaml:"allow"`
	Vars         map[string]Scalar `yaml:"vars"`
}

type Alias struct {
	Service     string            `yaml:"service"`
	Container   string            `yaml:"container"`
	PathMapping map[string]string `yaml:"path_mapping"`
	User        Scalar            `yaml:"user"`
	Env         Env               `yaml:"env"`
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
