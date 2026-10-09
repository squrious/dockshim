package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/squrious/dockshim/internal/hostpath"
)

const (
	// ToolName is the binary name: shims call it through PATH, and alias names and messages depend on it.
	ToolName = "dockshim"
	// FileName is the config file, at the project root. `init` writes it.
	FileName = ".dockshim.yaml"
	// DirName is the project directory dockshim owns: it holds generated files only, and git-ignores itself.
	DirName = ".dockshim"
	// RelBinDir holds the shims, relative to the project root. A dedicated directory keeps other
	// generated files out of PATH.
	RelBinDir = DirName + "/bin"
)

// Candidates are the config file names, in lookup order.
var Candidates = []string{FileName, ".dockshim.yml"}

// ErrNotFound is returned when there is no config file: above the start directory for Discover,
// in the project root for ForShim.
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

// Discover walks up from start and returns the first config file found.
func Discover(start string) (string, error) {
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
			return "", ErrNotFound
		}
		dir = parent
	}
}

// ForShim returns the config file of the project that holds shim, an absolute path, in its
// RelBinDir. Nothing above that project is looked at: a project without a config must not run
// another one's aliases.
func ForShim(shim string) (string, error) {
	// A link to a shim, or a linked directory in PATH, belongs to the project of the real shim.
	bin := filepath.Dir(hostpath.Real(shim))
	root, ok := strings.CutSuffix(bin, string(filepath.Separator)+filepath.FromSlash(RelBinDir))
	if !ok {
		return "", fmt.Errorf("%s is not in a %s directory", shim, RelBinDir)
	}
	if root == "" {
		root = string(filepath.Separator)
	}
	file, err := findIn(root)
	if err != nil {
		return "", err
	}
	if file == "" {
		return "", fmt.Errorf("%s: %w", root, ErrNotFound)
	}
	return file, nil
}
