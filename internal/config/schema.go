// Package config discovers, parses, validates and resolves dockshim configuration files.
package config

import (
	"strconv"

	"gopkg.in/yaml.v3"
)

// File is a config file as written, after interpolation. Load turns it into a Project.
type File struct {
	Compose  *Compose         `yaml:"compose"`
	Defaults Defaults         `yaml:"defaults"`
	Aliases  map[string]Alias `yaml:"aliases"`
}

// Compose holds the options passed to every docker compose call. Files are relative to the project root.
type Compose struct {
	Files       []string `yaml:"files"`
	ProjectName string   `yaml:"project_name"`
}

// Defaults holds the settings every alias inherits.
type Defaults struct {
	User            Scalar            `yaml:"user"`
	Env             map[string]Scalar `yaml:"env"`
	EnvForwarding   EnvForwarding     `yaml:"env_forwarding"`
	PathTranslation PathTranslation   `yaml:"path_translation"`
}

// PathTranslation configures how host paths given as arguments are made usable in the container.
type PathTranslation struct {
	Enabled Scalar   `yaml:"enabled"`
	Allow   []string `yaml:"allow"`
}

// EnvForwarding configures which host variables are forwarded. In items, * matches any run of
// characters.
type EnvForwarding struct {
	Deny  []string `yaml:"deny"`
	Allow []string `yaml:"allow"`
}

// Alias is one entry point. Its settings override or extend Defaults.
type Alias struct {
	Service         string            `yaml:"service"`
	PathMapping     map[string]string `yaml:"path_mapping"` // nil: inferred from the container's bind mounts
	User            Scalar            `yaml:"user"`
	Env             map[string]Scalar `yaml:"env"`
	EnvForwarding   EnvForwarding     `yaml:"env_forwarding"`
	PathTranslation PathTranslation   `yaml:"path_translation"`
}

// Scalar accepts any YAML scalar (string, int, bool...) as its literal text.
type Scalar string

func (s *Scalar) UnmarshalYAML(n *yaml.Node) error {
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
