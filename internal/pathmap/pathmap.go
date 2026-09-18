// Package pathmap translates host paths to container paths.
package pathmap

import (
	"path"
	"path/filepath"
	"strings"
)

// Mapping makes the host directory Host visible at Container in the container.
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
		rel, ok := Rel(mp.Host, hostPath)
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

// Rel returns p relative to base, when p is base or below it. Both must be absolute and cleaned.
func Rel(base, p string) (string, bool) {
	rel, err := filepath.Rel(base, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return rel, true
}

// Within reports whether p is base or below it. Both must be absolute and cleaned.
func Within(base, p string) bool {
	_, ok := Rel(base, p)
	return ok
}
