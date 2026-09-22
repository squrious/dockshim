package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ToolName is the binary name: shims call it through PATH, and alias names and messages depend on it.
	ToolName = "dockshim"
	// DirName is the project directory holding the config file and, by default, bin_dir.
	DirName = ".dockshim"
	// FlatFile is the config file at the project root, the alternative to DirFile.
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

// ErrNotFound is returned by Discover when no start directory has a config file above it.
var ErrNotFound = errors.New("no dockshim config found (looked for " + strings.Join(Candidates, ", ") + ")")

// findIn returns the config file of dir, or "" if there is none.
func findIn(dir string) (string, error) {
	var found []string
	for _, c := range Candidates {
		p := filepath.Join(dir, c)
		fi, err := os.Stat(p)
		switch {
		case errors.Is(err, os.ErrNotExist):
		case err != nil:
			return "", err
		case !fi.IsDir():
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
			file, err := findIn(dir)
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
