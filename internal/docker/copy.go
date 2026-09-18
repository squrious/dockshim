package docker

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// CopyArchive extracts a tar stream into dest in container id, keeping the archive's uid/gid.
// dir is the directory docker runs from, as for every call on a target.
func CopyArchive(r Runner, dir, id, dest string, archive io.Reader) error {
	return runCapturing(r, Cmd{Dir: dir, Args: []string{"cp", "--archive", "-", id + ":" + dest}, Stdin: archive})
}

// RemoveAll deletes path in container id, as root.
func RemoveAll(r Runner, dir, id, path string) error {
	return runCapturing(r, Cmd{Dir: dir, Args: []string{"exec", "--user", "0", id, "rm", "-rf", path}})
}

func runCapturing(r Runner, c Cmd) error {
	var stderr bytes.Buffer
	c.Stdout, c.Stderr = io.Discard, &stderr
	code, err := r.Run(c)
	if err == nil && code != 0 {
		err = fmt.Errorf("exit status %d: %s", code, strings.TrimSpace(stderr.String()))
	}
	if err != nil {
		return fmt.Errorf("docker %s: %w", strings.Join(c.Args, " "), err)
	}
	return nil
}
