package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	DirName  = ".dockshim"
	FlatFile = ".dockshim.yaml"
)

// DirFile is the config file inside DirName, relative to a project root.
var DirFile = filepath.Join(DirName, "config.yaml")

// Candidates are relative to a project root, in lookup order.
var Candidates = []string{
	FlatFile,
	".dockshim.yml",
	DirFile,
	filepath.Join(DirName, "config.yml"),
}

var ErrNotFound = errors.New("no dockshim config found (looked for " + strings.Join(Candidates, ", ") + ")")

// FindIn returns the config file of dir, or "" if there is none.
func FindIn(dir string) (string, error) {
	var found []string
	for _, c := range Candidates {
		p := filepath.Join(dir, c)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			found = append(found, p)
		}
	}
	switch len(found) {
	case 0:
		return "", nil
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("ambiguous config in %s: %s", dir, strings.Join(found, ", "))
	}
}

// Discover walks up from each start directory in turn and returns the first config file found.
func Discover(starts ...string) (string, error) {
	for _, start := range starts {
		dir, err := filepath.Abs(start)
		if err != nil {
			return "", err
		}
		for {
			file, err := FindIn(dir)
			if err != nil || file != "" {
				return file, err
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "", ErrNotFound
}

// RootOf returns the project root a config file belongs to.
func RootOf(file string) string {
	dir := filepath.Dir(file)
	base := filepath.Base(file)
	if filepath.Base(dir) == DirName && (base == "config.yaml" || base == "config.yml") {
		return filepath.Dir(dir)
	}
	return dir
}
