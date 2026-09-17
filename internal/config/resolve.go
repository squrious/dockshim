package config

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"

	"github.com/squrious/dockshim/internal/envfilter"
	"github.com/squrious/dockshim/internal/pathmap"
)

const (
	UserHost         = "host"
	DefaultBinDir    = DirName + "/bin"
	DefaultMaxCopyMB = 100
)

// Project is a fully resolved configuration: paths are absolute, aliases merged with global.
type Project struct {
	Root    string                    `yaml:"root"`
	File    string                    `yaml:"file"`
	BinDir  string                    `yaml:"bin_dir"`
	Compose *Compose                  `yaml:"compose,omitempty"`
	Aliases map[string]*ResolvedAlias `yaml:"aliases"`
}

type ResolvedAlias struct {
	Name            string                  `yaml:"-"`
	Service         string                  `yaml:"service,omitempty"`
	Container       string                  `yaml:"container,omitempty"`
	User            string                  `yaml:"user"`
	PathMapping     pathmap.Map             `yaml:"path_mapping"`
	Env             envfilter.Rules         `yaml:"env"`
	Vars            map[string]string       `yaml:"vars"`
	PathTranslation ResolvedPathTranslation `yaml:"path_translation"`
}

type ResolvedPathTranslation struct {
	Enabled bool `yaml:"enabled"`
	// Exclude lists host paths never copied into the container.
	Exclude   []string `yaml:"exclude"`
	MaxCopyMB int      `yaml:"max_copy_mb"`
}

// Load parses, validates and resolves the config file, interpolating the process environment.
func Load(file string) (*Project, error) {
	file, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	f, err := Parse(data, os.LookupEnv)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if err := f.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return f.Resolve(file), nil
}

func Parse(data []byte, lookup LookupFunc) (*File, error) {
	// Node.Decode cannot reject unknown fields: check the raw document strictly first.
	var f File
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return &f, nil
		}
		return nil, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if err := interpolateNode(&doc, lookup); err != nil {
		return nil, err
	}
	f = File{}
	if err := doc.Decode(&f); err != nil {
		return nil, err
	}
	return &f, nil
}

// Resolve merges global settings into each alias and makes paths absolute.
func (f *File) Resolve(file string) *Project {
	root := realPath(RootOf(file))
	p := &Project{
		Root:    root,
		File:    file,
		BinDir:  absFrom(root, cmp.Or(f.BinDir, DefaultBinDir)),
		Aliases: map[string]*ResolvedAlias{},
	}
	if f.Compose != nil {
		c := &Compose{ProjectName: f.Compose.ProjectName}
		for _, cf := range f.Compose.Files {
			c.Files = append(c.Files, absFrom(root, cf))
		}
		p.Compose = c
	}

	for name, a := range f.Aliases {
		r := &ResolvedAlias{
			Name:      name,
			Service:   a.Service,
			Container: a.Container,
			User:      resolveUser(string(cmp.Or(a.User, f.Global.User, UserHost))),
			Env: envfilter.Rules{
				Deny:         concat(envfilter.DefaultDeny, f.Global.Env.Deny, a.Env.Deny),
				DenyPrefixes: concat(envfilter.DefaultDenyPrefixes, f.Global.Env.DenyPrefixes, a.Env.DenyPrefixes),
				Allow:        concat(f.Global.Env.Allow, a.Env.Allow),
			},
			Vars: map[string]string{},
			PathTranslation: ResolvedPathTranslation{
				Enabled:   cmp.Or(a.PathTranslation.Enabled, f.Global.PathTranslation.Enabled, "true").Bool(),
				Exclude:   concat(pathmap.DefaultCopyExclude, f.Global.PathTranslation.Exclude, a.PathTranslation.Exclude),
				MaxCopyMB: cmp.Or(a.PathTranslation.MaxCopyMB, f.Global.PathTranslation.MaxCopyMB).Int(DefaultMaxCopyMB),
			},
		}
		for _, vars := range []map[string]Scalar{f.Global.Env.Vars, a.Env.Vars} {
			for k, v := range vars {
				r.Vars[k] = string(v)
			}
		}
		for _, host := range sortedKeys(a.PathMapping) {
			r.PathMapping = append(r.PathMapping, pathmap.Mapping{
				Host:      filepath.Join(root, host),
				Container: a.PathMapping[host],
			})
		}
		p.Aliases[name] = r
	}
	return p
}

func resolveUser(u string) string {
	if u == UserHost {
		return strconv.Itoa(os.Getuid()) + ":" + strconv.Itoa(os.Getgid())
	}
	return u
}

func absFrom(root, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(root, p)
}

// realPath resolves symlinks so paths compare equal to os.Getwd results.
func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

func concat(lists ...[]string) []string {
	out := []string{}
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}
