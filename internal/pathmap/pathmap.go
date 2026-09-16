// Package pathmap translates host paths to container paths.
package pathmap

import (
	"path"
	"path/filepath"
	"strings"
)

type Mapping struct {
	Host      string `yaml:"host"`
	Container string `yaml:"container"`
}

// Map holds mappings with absolute, cleaned host paths.
type Map []Mapping

// ToContainer translates hostPath using the mapping with the longest matching host prefix.
func (m Map) ToContainer(hostPath string) (string, bool) {
	best := -1
	var bestRel string
	for i, mp := range m {
		rel, ok := within(mp.Host, hostPath)
		if !ok {
			continue
		}
		if best < 0 || len(mp.Host) > len(m[best].Host) {
			best, bestRel = i, rel
		}
	}
	if best < 0 {
		return "", false
	}
	return path.Join(m[best].Container, filepath.ToSlash(bestRel)), true
}

// within reports whether p is base or below it, and returns p relative to base.
func within(base, p string) (string, bool) {
	rel, err := filepath.Rel(base, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// Within reports whether p is base or below it. Both must be absolute and cleaned.
func Within(base, p string) bool {
	_, ok := within(base, p)
	return ok
}
