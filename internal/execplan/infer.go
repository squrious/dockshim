package execplan

import (
	"cmp"
	"path/filepath"
	"slices"

	"github.com/squrious/dockshim/internal/docker"
	"github.com/squrious/dockshim/internal/hostpath"
	"github.com/squrious/dockshim/internal/pathmap"
)

// InferMappings reads the mounts of the running target and maps the bind mounts whose source is
// in the project root (ADR 0009). It also returns the sources of the bind mounts skipped for
// being outside it. Volumes and tmpfs are ignored.
func InferMappings(t docker.Target, root string) (pathmap.Map, []string, error) {
	mounts, err := t.Mounts()
	if err != nil {
		return nil, nil, err
	}
	m, outside := inferMappings(mounts, root)
	return m, outside, nil
}

func inferMappings(mounts []docker.Mount, root string) (m pathmap.Map, outside []string) {
	// Sorted, so that a directory mounted twice always maps to the same destination.
	mounts = slices.SortedFunc(slices.Values(mounts), func(a, b docker.Mount) int {
		return cmp.Compare(a.Destination, b.Destination)
	})
	m = pathmap.Map{}
	for _, mt := range mounts {
		if mt.Type != docker.MountBind {
			continue
		}
		host := hostpath.Real(filepath.Clean(mt.Source))
		if !pathmap.Within(root, host) {
			outside = append(outside, mt.Source)
			continue
		}
		m = append(m, pathmap.Mapping{Host: host, Container: mt.Destination})
	}
	return m, outside
}
