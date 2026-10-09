package cli

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/execplan"
	"github.com/squrious/dockshim/internal/pathmap"
	"github.com/squrious/dockshim/internal/shim"
)

// Version is set at build time with -ldflags "-X github.com/squrious/dockshim/internal/cli.Version=...".
var Version = ""

type exitCodeError int

func (e exitCodeError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }

func runManager(args []string, e *Env) int {
	root := newRoot(e)
	root.SetArgs(args)
	root.SetIn(e.Stdin)
	root.SetOut(e.Stdout)
	root.SetErr(e.Stderr)
	err := root.Execute()
	var code exitCodeError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &code):
		return int(code)
	default:
		e.errorf("%v", err)
		return 1
	}
}

// load finds the config of a manager command, from the current directory upwards.
func (e *Env) load() (*config.Project, error) {
	cwd, err := e.cwd()
	if err != nil {
		return nil, err
	}
	file, err := config.Discover(cwd)
	if err != nil {
		return nil, err
	}
	return e.loadConfig(file)
}

func (e *Env) loadConfig(file string) (*config.Project, error) {
	return config.Load(file, config.LookupEnviron(e.Environ))
}

func newRoot(e *Env) *cobra.Command {
	root := &cobra.Command{
		Use:           config.ToolName,
		Short:         "Run commands in Docker containers as if they were installed on the host",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newConfigCmd(e),
		&cobra.Command{
			Use:   "install",
			Short: "Create an entry point for each alias in " + config.RelBinDir + ", and remove stale ones",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				proj, err := e.load()
				if err != nil {
					return err
				}
				for _, dir := range []string{config.DirName, config.RelBinDir} {
					if err := ownedDir(filepath.Join(proj.Root, dir)); err != nil {
						return err
					}
				}
				if err := ignoreDir(filepath.Join(proj.Root, config.DirName)); err != nil {
					return err
				}
				res, err := shim.Install(proj.BinDir(), slices.Sorted(maps.Keys(proj.Aliases)))
				printList(cmd, e.OutStyle.Success("created:"), res.Created)
				printList(cmd, e.OutStyle.Warning("removed:"), res.Removed)
				if err != nil {
					return err
				}
				if !inPath(e.Environ, proj.BinDir()) {
					cmd.Printf("\n%s is not in PATH: add it to run the aliases by name.\n", proj.BinDir())
				}
				if !commandInPath(e.Environ, config.ToolName) {
					e.warnf("%s is not in PATH, the shims won't find it", config.ToolName)
				}
				return nil
			},
		},
		newRunCmd(e),
		newInitCmd(e),
		&cobra.Command{
			Use:   "version",
			Short: "Print the dockshim version",
			Args:  cobra.NoArgs,
			Run: func(cmd *cobra.Command, args []string) {
				cmd.Println(version())
			},
		},
	)
	setHelp(root, e.OutStyle)
	return root
}

func newConfigCmd(e *Env) *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Print the effective configuration",
		Long: "Print the effective configuration: service, user and path mappings of each alias, and the\n" +
			"settings that differ from the defaults. Inferred mappings are read from the running container,\n" +
			"or from the compose config while the service is down.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := e.load()
			if err != nil {
				return err
			}
			cache := map[string]*inference{}
			err = printSummary(cmd.OutOrStdout(), e.OutStyle, proj, func(a *config.ResolvedAlias) *inference {
				if inf, ok := cache[a.Service]; ok {
					return inf
				}
				cache[a.Service] = inferMounts(e, proj, a)
				return cache[a.Service]
			})
			if err != nil {
				return err
			}
			// Last, so that they stay in sight below the summary.
			warnUnknownServices(e, proj)
			warnOutdatedShims(e, proj)
			return nil
		},
	}
}

// inference is what path mapping inference gives for a service, read from the running container
// or, while it is down, from the compose config.
type inference struct {
	running  bool
	mappings pathmap.Map
	outside  []string
}

// inferMounts never starts the service, and returns nil when docker can't tell.
func inferMounts(e *Env, proj *config.Project, a *config.ResolvedAlias) *inference {
	if e.Runner == nil {
		return nil
	}
	target := execplan.NewTarget(proj, a.Service, e.Runner)
	running := target.IsRunning()
	mount := target.ConfiguredMounts
	if running {
		mount = target.Mounts
	}
	mounts, err := mount()
	if err != nil {
		return nil
	}
	m, outside := execplan.InferMappings(mounts, proj.Root)
	return &inference{running: running, mappings: m, outside: outside}
}

// warnUnknownServices tells which aliases target a service the compose project lacks. It says
// nothing when docker can't tell.
func warnUnknownServices(e *Env, proj *config.Project) {
	if e.Runner == nil {
		return
	}
	services, err := execplan.NewTarget(proj, "", e.Runner).Services()
	if err != nil {
		return
	}
	for _, name := range slices.Sorted(maps.Keys(proj.Aliases)) {
		if s := proj.Aliases[name].Service; !slices.Contains(services, s) {
			e.warnf("alias %q: service %q is not in the compose project", name, s)
		}
	}
}

// newRunCmd is the command the entry points call.
func newRunCmd(e *Env) *cobra.Command {
	var shimPath string
	cmd := &cobra.Command{
		Use:                   "run --shim <path> <alias> [args...]",
		Short:                 "Run an alias, called by its entry point in " + config.RelBinDir,
		Hidden:                true,
		DisableFlagsInUseLine: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if shimPath == "" {
				alias := "<alias>"
				if len(args) > 0 {
					alias = args[0]
				}
				return fmt.Errorf("run only serves the entry points: call %s/%s instead", config.RelBinDir, alias)
			}
			return cobra.MinimumNArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := e.cwd()
			if err != nil {
				return err
			}
			if !filepath.IsAbs(shimPath) {
				// Not filepath.Join: cleaning would drop "link/.." before the link is resolved.
				shimPath = cwd + string(filepath.Separator) + shimPath
			}
			file, err := config.ForShim(shimPath)
			if err != nil {
				return err
			}
			proj, err := e.loadConfig(file)
			if err != nil {
				return err
			}
			if code := execAlias(e, proj, args[0], cwd, args[1:]); code != 0 {
				return exitCodeError(code)
			}
			return nil
		},
	}
	// The project is the one holding the shim, whatever the working directory.
	cmd.Flags().StringVar(&shimPath, "shim", "", "`path` of the entry point this call comes from (required)")
	// Everything after the alias name belongs to the aliased command.
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func printList(cmd *cobra.Command, label string, items []string) {
	if len(items) > 0 {
		cmd.Printf("%s %s\n", label, strings.Join(items, ", "))
	}
}

// ownedDir fails when p exists but is not a real directory. install removes whatever it doesn't
// expect in the directories it owns: through a symlink, that would be someone else's files.
func ownedDir(p string) error {
	fi, err := os.Lstat(p)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return nil
	case err != nil:
		return err
	case !fi.IsDir():
		return fmt.Errorf("%s must be a directory: %s owns it", p, config.ToolName)
	}
	return nil
}

// ignoreDir creates dir with a .gitignore ignoring everything in it, itself included.
// An existing .gitignore is left alone. install writes it rather than init, so that it is never
// committed and a fresh clone gets it from its first install.
func ignoreDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	p := filepath.Join(dir, ".gitignore")
	if _, err := os.Lstat(p); !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(p, []byte("*\n"), 0o644)
}

// pathDirs returns the directories of the first PATH in environ, as getenv would.
func pathDirs(environ []string) []string {
	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, "PATH="); ok {
			return filepath.SplitList(v)
		}
	}
	return nil
}

func inPath(environ []string, dir string) bool {
	return slices.Contains(pathDirs(environ), dir)
}

// commandInPath reports whether an executable file named name is in the PATH of environ.
func commandInPath(environ []string, name string) bool {
	for _, dir := range pathDirs(environ) {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o111 != 0 {
			return true
		}
	}
	return false
}

func version() string {
	if Version != "" {
		return Version
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "dev"
}
