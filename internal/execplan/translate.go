package execplan

import (
	"archive/tar"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/docker"
	"github.com/squrious/dockshim/internal/hostpath"
	"github.com/squrious/dockshim/internal/pathmap"
)

const containerTmp = "/tmp"

// pathTranslator makes host paths given as arguments usable in the container.
// Paths under a mapping are rewritten to their container path. Files inside an allowed
// directory are copied into a temporary container directory for the duration of the
// command. Anything else keeps its value, and so means the container's own path.
type pathTranslator struct {
	mappings pathmap.Map
	cwd      string
	// cwdContainer is the container working directory, empty when the cwd is not mapped.
	cwdContainer string
	host         *hostpath.Resolver
	maxBytes     int64
	uid, gid     int // -1 when unknown
	warn         func(format string, args ...any)
}

type copyItem struct {
	host string // real path
	name string // path inside the temporary directory
}

// hostOptions is the host path policy an alias configures.
func hostOptions(a *config.ResolvedAlias) hostpath.Options {
	return hostpath.Options{
		Allow:          a.PathTranslation.Allow,
		FollowSymlinks: a.PathTranslation.FollowSymlinks,
	}
}

func newPathTranslator(in Input, cwdContainer string, warn func(string, ...any)) *pathTranslator {
	uid, gid := numericUser(in.Alias.User)
	pt := in.Alias.PathTranslation
	return &pathTranslator{
		mappings:     in.Alias.PathMapping,
		cwd:          in.Cwd,
		cwdContainer: cwdContainer,
		host:         in.HostPaths,
		maxBytes:     int64(pt.MaxCopyMB) << 20,
		uid:          uid,
		gid:          gid,
		warn:         warn,
	}
}

// transform returns args with host paths translated, and registers on p the steps copying files in and out.
func (t *pathTranslator) transform(p *Plan, args []string) []string {
	out := slices.Clone(args)
	base := config.ToolName + "-" + rand.Text()
	copies := map[string]string{}
	var items []copyItem
	var total int64

	// args[0] is the command itself.
	for i := 1; i < len(args); i++ {
		prefix, value := splitOption(args[i])
		tr := t.translate(value)
		if tr.warn != "" {
			t.warn("not copying %s into the container: %s", value, tr.warn)
		}
		if tr.copyFrom != "" {
			ctr, seen := copies[tr.copyFrom]
			if !seen {
				size, err := copyableSize(tr.copyFrom, t.maxBytes-total)
				if err != nil {
					t.warn("not copying %s into the container: %v", value, err)
					continue
				}
				total += size
				item := copyItem{host: tr.copyFrom, name: path.Join(strconv.Itoa(len(items)), tr.name)}
				items = append(items, item)
				ctr = path.Join(containerTmp, base, item.name)
				copies[tr.copyFrom] = ctr
			}
			tr.value = ctr
		}
		out[i] = prefix + tr.value
	}

	if len(items) > 0 {
		t.registerSteps(p, base, items)
	}
	return out
}

// translation is what becomes of one argument: a value to use, a host file to copy first
// under the name the argument gave it, and a reason why a path could not be copied.
type translation struct {
	value    string
	copyFrom string
	name     string
	warn     string
}

func (t *pathTranslator) translate(value string) translation {
	if value == "" || value == "-" {
		return translation{value: value}
	}
	// A Windows path names a file the container has no chance of finding as typed,
	// so it is worth reporting when it cannot be translated.
	win := hostpath.IsWindows(value)
	lookup := value
	if win {
		converted, ok := t.host.ToLinux(value)
		if !ok {
			return translation{value: value, warn: "is a Windows path this distribution cannot reach"}
		}
		lookup = converted
	}
	isAbs := filepath.IsAbs(lookup)
	abs := lookup
	if !isAbs {
		abs = filepath.Join(t.cwd, lookup)
	}
	abs = filepath.Clean(abs)

	for _, candidate := range []string{abs, hostpath.Real(abs)} {
		ctr, ok := t.mappings.ToContainer(candidate)
		if !ok {
			continue
		}
		// A relative path needs nothing when the working directory already resolves it there.
		if !isAbs && t.cwdContainer != "" && path.Join(t.cwdContainer, filepath.ToSlash(lookup)) == ctr {
			return translation{value: value}
		}
		return translation{value: ctr}
	}

	// Missing paths are left alone: they are usually outputs.
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return untranslated(value, win)
	}
	if !t.host.Allowed(abs, real) {
		return untranslated(value, win)
	}
	fi, err := os.Stat(real)
	switch {
	case err != nil:
		return translation{value: value}
	case fi.IsDir():
		return translation{value: value, warn: "is a directory"}
	case !fi.Mode().IsRegular():
		return translation{value: value, warn: "is not a regular file"}
	}
	return translation{value: value, copyFrom: real, name: filepath.Base(abs)}
}

// untranslated keeps the argument as typed, so it designates the container's own path.
// That is the ordinary outcome and stays silent, unless the argument was a Windows path.
func untranslated(value string, win bool) translation {
	t := translation{value: value}
	if win {
		t.warn = "is outside the allowed paths, see path_translation.allow"
	}
	return t
}

func (t *pathTranslator) registerSteps(p *Plan, base string, items []copyItem) {
	target, runner := p.Target, p.Runner
	var id string
	p.PreRun = append(p.PreRun, func() error {
		var err error
		if id, err = target.ContainerID(); err != nil {
			return err
		}
		pr, pw := io.Pipe()
		written := make(chan error, 1)
		go func() {
			err := t.writeArchive(pw, base, items)
			_ = pw.CloseWithError(err)
			written <- err
		}()
		err = docker.CopyArchive(runner, target.Dir(), id, containerTmp, pr)
		_ = pr.Close()
		if werr := <-written; werr != nil && !errors.Is(werr, io.ErrClosedPipe) {
			return fmt.Errorf("copying files into the container: %w", werr)
		}
		return err
	})
	p.PostRun = append(p.PostRun, func() error {
		if id == "" {
			return nil
		}
		return docker.RemoveAll(runner, target.Dir(), id, path.Join(containerTmp, base))
	})
}

func (t *pathTranslator) writeArchive(w io.Writer, base string, items []copyItem) error {
	tw := tar.NewWriter(w)
	if err := t.writeDir(tw, base); err != nil {
		return err
	}
	for _, item := range items {
		if err := t.writeDir(tw, path.Join(base, path.Dir(item.name))); err != nil {
			return err
		}
		if err := t.writeFile(tw, item.host, path.Join(base, item.name)); err != nil {
			return err
		}
	}
	return tw.Close()
}

func (t *pathTranslator) writeDir(tw *tar.Writer, name string) error {
	mode := int64(0o755)
	if t.uid >= 0 {
		mode = 0o700
	}
	return tw.WriteHeader(t.owned(&tar.Header{Typeflag: tar.TypeDir, Name: name + "/", Mode: mode}))
}

func (t *pathTranslator) writeFile(tw *tar.Writer, host, name string) error {
	fi, err := os.Stat(host)
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}
	hdr.Name = name
	if err := tw.WriteHeader(t.owned(hdr)); err != nil {
		return err
	}
	f, err := os.Open(host)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.CopyN(tw, f, hdr.Size)
	return err
}

// owned sets ownership to the container user. When it is unknown, files belong to root and are made world-readable.
func (t *pathTranslator) owned(h *tar.Header) *tar.Header {
	h.Uname, h.Gname = "", ""
	if t.uid >= 0 {
		h.Uid, h.Gid = t.uid, t.gid
		return h
	}
	h.Uid, h.Gid = 0, 0
	switch h.Typeflag {
	case tar.TypeDir:
		h.Mode |= 0o755
	case tar.TypeReg:
		h.Mode |= 0o644
	}
	return h
}

var errTooLarge = errors.New("too large, see path_translation.max_copy_mb")

// copyableSize returns the size of a readable file, within limit.
func copyableSize(host string, limit int64) (int64, error) {
	fi, err := os.Stat(host)
	if err != nil {
		return 0, err
	}
	if fi.Size() > limit {
		return 0, errTooLarge
	}
	f, err := os.Open(host)
	if err != nil {
		return 0, err
	}
	return fi.Size(), f.Close()
}

// splitOption separates "--opt=" from its value. Other options have no value to translate.
func splitOption(arg string) (prefix, value string) {
	if !strings.HasPrefix(arg, "-") {
		return "", arg
	}
	if i := strings.IndexByte(arg, '='); i > 0 {
		return arg[:i+1], arg[i+1:]
	}
	return arg, ""
}

// numericUser parses "uid[:gid]", returning -1s for names.
func numericUser(user string) (uid, gid int) {
	u, g, _ := strings.Cut(user, ":")
	uid, err := strconv.Atoi(u)
	if err != nil {
		return -1, -1
	}
	if g == "" {
		return uid, 0
	}
	if gid, err = strconv.Atoi(g); err != nil {
		return -1, -1
	}
	return uid, gid
}
