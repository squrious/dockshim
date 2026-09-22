package config

import (
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/squrious/dockshim/internal/envfilter"
	"github.com/squrious/dockshim/internal/hostpath"
)

// ValidationError lists every problem found in a config file, each prefixed with its field path.
type ValidationError struct {
	Problems []string
}

func (e *ValidationError) Error() string {
	return "invalid config:\n  " + strings.Join(e.Problems, "\n  ")
}

var (
	aliasNameRe = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._+-]*$`)
	userRe      = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*(:[A-Za-z0-9_][A-Za-z0-9_.-]*)?$`)
)

// Validate checks the file as written, before interpolated values are resolved into a Project.
// It reports all problems at once, as a *ValidationError.
func (f *File) Validate() error {
	var problems []string
	add := func(field, format string, args ...any) {
		problems = append(problems, field+": "+fmt.Sprintf(format, args...))
	}

	if f.Compose != nil {
		for i, file := range f.Compose.Files {
			if file == "" {
				add(fmt.Sprintf("compose.files[%d]", i), "must not be empty")
			}
		}
	}
	validateUser(add, "global.user", f.Global.User)
	validateEnv(add, "global.env", f.Global.Env)
	validatePathTranslation(add, "global.path_translation", f.Global.PathTranslation)

	for _, name := range sortedKeys(f.Aliases) {
		a := f.Aliases[name]
		field := "aliases." + name
		// Case-insensitive: on such filesystems, a shim named like the tool would shadow it and exec itself.
		if !aliasNameRe.MatchString(name) || strings.EqualFold(name, ToolName) {
			add(field, "invalid alias name (must be a plain file name, other than %q)", ToolName)
		}
		if a.Service == "" {
			add(field+".service", "is required")
		}

		seen := map[string]string{}
		for _, host := range sortedKeys(a.PathMapping) {
			ctr := a.PathMapping[host]
			mf := field + ".path_mapping[" + host + "]"
			clean := filepath.Clean(host)
			switch {
			case filepath.IsAbs(clean), clean == "..", strings.HasPrefix(clean, ".."+string(filepath.Separator)):
				add(mf, "host path must be inside the project directory")
			case seen[clean] != "":
				add(mf, "duplicates %q", seen[clean])
			}
			seen[clean] = host
			if !path.IsAbs(ctr) {
				add(mf, "container path %q must be absolute", ctr)
			}
		}
		validateUser(add, field+".user", a.User)
		validateEnv(add, field+".env", a.Env)
		validatePathTranslation(add, field+".path_translation", a.PathTranslation)
	}
	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

func validateUser(add func(string, string, ...any), field string, u Scalar) {
	if u != "" && !userRe.MatchString(string(u)) {
		add(field, "invalid user %q (expected host, uid[:gid] or name[:group])", u)
	}
}

func validateEnv(add func(string, string, ...any), field string, e Env) {
	for _, list := range []struct {
		name  string
		items []string
	}{{"deny", e.Deny}, {"deny_prefixes", e.DenyPrefixes}, {"allow", e.Allow}} {
		for i, n := range list.items {
			if !envfilter.ValidName(n) {
				add(fmt.Sprintf("%s.%s[%d]", field, list.name, i), "invalid variable name %q", n)
			}
		}
	}
	for _, n := range sortedKeys(e.Vars) {
		if !envfilter.ValidName(n) {
			add(field+".vars", "invalid variable name %q", n)
		}
	}
}

func validatePathTranslation(add func(string, string, ...any), field string, pt PathTranslation) {
	if pt.Enabled != "" {
		if _, err := strconv.ParseBool(string(pt.Enabled)); err != nil {
			add(field+".enabled", "invalid boolean %q", pt.Enabled)
		}
	}
	if pt.FollowSymlinks != "" {
		if _, err := strconv.ParseBool(string(pt.FollowSymlinks)); err != nil {
			add(field+".follow_symlinks", "invalid boolean %q", pt.FollowSymlinks)
		}
	}
	if pt.MaxCopyMB != "" {
		if n, err := strconv.Atoi(string(pt.MaxCopyMB)); err != nil || n <= 0 {
			add(field+".max_copy_mb", "must be a positive integer, got %q", pt.MaxCopyMB)
		}
	}
	for i, p := range pt.Allow {
		if !filepath.IsAbs(p) && !hostpath.IsWindows(p) {
			add(fmt.Sprintf("%s.allow[%d]", field, i), "host path %q must be absolute", p)
		}
	}
}

func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}
