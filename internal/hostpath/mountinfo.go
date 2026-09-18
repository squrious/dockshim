package hostpath

import (
	"bufio"
	"io"
	"slices"
	"strconv"
	"strings"
)

// parseDrives reads /proc/self/mountinfo and returns the Windows drives WSL makes visible,
// keyed by lowercase drive letter: "c" -> "/mnt/c".
func parseDrives(r io.Reader) map[string]string {
	if r == nil {
		return nil
	}
	drives := map[string]string{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		// id parent maj:min root mountPoint options... - fstype source superOptions
		fields := strings.Fields(sc.Text())
		sep := -1
		for i, f := range fields {
			if f == "-" {
				sep = i
				break
			}
		}
		if sep < 5 || len(fields) < sep+4 {
			continue
		}
		mountPoint, fsType, source, super := unescape(fields[4]), fields[sep+1], unescape(fields[sep+2]), fields[sep+3]
		if !isDrvfs(fsType, super) {
			continue
		}
		if letter := driveLetter(super, source); letter != "" {
			drives[letter] = mountPoint
		}
	}
	// On a read error no drive is trusted: Windows paths are then reported as unreachable.
	if sc.Err() != nil || len(drives) == 0 {
		return nil
	}
	return drives
}

// isDrvfs reports whether a mount exposes a Windows drive: drvfs directly under WSL 1,
// or a 9p/virtiofs transport announcing itself as drvfs under WSL 2.
func isDrvfs(fsType, super string) bool {
	switch fsType {
	case "drvfs":
		return true
	case "9p", "virtiofs":
		return hasOption(super, "aname=drvfs")
	}
	return false
}

// driveLetter reads the drive from the "path=C:\" super option, or from the mount source.
func driveLetter(super, source string) string {
	spec := source
	for _, opt := range strings.Split(super, ";") {
		if v, ok := strings.CutPrefix(opt, "path="); ok {
			spec = v
			break
		}
	}
	if len(spec) < 2 || spec[1] != ':' {
		return ""
	}
	return strings.ToLower(spec[:1])
}

func hasOption(super, want string) bool {
	return slices.Contains(options(super), want)
}

// options splits super options. WSL mixes both separators: "rw,aname=drvfs;path=C:\\;uid=1000".
func options(super string) []string {
	return strings.FieldsFunc(super, func(r rune) bool { return r == ',' || r == ';' })
}

// unescape resolves the octal escapes mountinfo uses for space, tab, newline and backslash.
func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if n, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
