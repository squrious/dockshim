package cli

import (
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/squrious/dockshim/internal/config"
	"github.com/squrious/dockshim/internal/execplan"
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

func newRoot(e *Env) *cobra.Command {
	var configFile string
	load := func(shimPath string) (*config.Project, error) {
		if configFile != "" {
			return config.Load(configFile, config.LookupEnviron(e.Environ))
		}
		cwd, err := e.cwd()
		if err != nil {
			return nil, err
		}
		return e.discover(cwd, shimPath)
	}

	root := &cobra.Command{
		Use:           config.ToolName,
		Short:         "Run commands in Docker containers as if they were installed on the host",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file (default: discovered from the current directory upwards)")

	root.AddCommand(
		newConfigCmd(e, load),
		&cobra.Command{
			Use:   "validate",
			Short: "Validate the configuration",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				proj, err := load("")
				if err != nil {
					return err
				}
				cmd.Printf("%s is valid\n", proj.File)
				return nil
			},
		},
		&cobra.Command{
			Use:   "install",
			Short: "Create an entry point for each alias in the bin directory, and remove stale ones",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, args []string) error {
				proj, err := load("")
				if err != nil {
					return err
				}
				if e.Executable == "" {
					return errors.New("cannot determine the dockshim executable path")
				}
				shims := make([]shim.Shim, 0, len(proj.Aliases))
				for _, name := range slices.Sorted(maps.Keys(proj.Aliases)) {
					shims = append(shims, shim.Shim{Name: name, Mode: proj.Aliases[name].ShimMode})
				}
				res, err := shim.Install(proj.BinDir, e.Executable, shims)
				printList(cmd, "created", res.Created)
				printList(cmd, "removed", res.Removed)
				if len(res.Skipped) > 0 {
					e.errorf("skipped, not a dockshim shim: %s", strings.Join(res.Skipped, ", "))
				}
				if err != nil {
					return err
				}
				if !inPath(e.Environ, proj.BinDir) {
					rel, _ := filepath.Rel(proj.Root, proj.BinDir)
					cmd.Printf("\n%s is not in PATH. For instance:\n  mise.toml: [env] _.path = [\"{{config_root}}/%s\"]\n  .envrc:    PATH_add %s\n", proj.BinDir, rel, rel)
				}
				return nil
			},
		},
		newRunCmd(e, load),
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
	return root
}

func newConfigCmd(e *Env, load func(shimPath string) (*config.Project, error)) *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "config [alias]",
		Short: "Print the resolved configuration",
		Long: "Print the resolved configuration: service, user and path mappings of each alias, and the\n" +
			"settings that differ from the defaults. --full prints everything, as YAML.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := load("")
			if err != nil {
				return err
			}
			if len(args) == 1 {
				a, ok := proj.Aliases[args[0]]
				if !ok {
					return fmt.Errorf("alias %q is not defined in %s", args[0], proj.File)
				}
				proj.Aliases = map[string]*config.ResolvedAlias{args[0]: a}
			}
			if !full {
				return printSummary(cmd.OutOrStdout(), proj, func(a *config.ResolvedAlias) []string {
					return outsideMounts(e, proj, a)
				})
			}
			enc := yaml.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent(2)
			if err := enc.Encode(proj); err != nil {
				return err
			}
			return enc.Close()
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "print every setting, defaults included, as YAML")
	return cmd
}

// outsideMounts lists the bind mounts inference skips, when the alias service is running.
// It never starts the service, and stays silent when docker can't tell.
func outsideMounts(e *Env, proj *config.Project, a *config.ResolvedAlias) []string {
	if e.Runner == nil {
		return nil
	}
	target := execplan.NewTarget(proj, a, e.Runner)
	if !target.IsRunning() {
		return nil
	}
	_, outside, _ := execplan.InferMappings(target, proj.Root)
	return outside
}

func newRunCmd(e *Env, load func(shimPath string) (*config.Project, error)) *cobra.Command {
	var shimPath string
	cmd := &cobra.Command{
		Use:   "run <alias> [args...]",
		Short: "Run an alias, as if invoked through its shim",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			proj, err := load(shimPath)
			if err != nil {
				return err
			}
			cwd, err := e.cwd()
			if err != nil {
				return err
			}
			if code := execAlias(e, proj, args[0], shimPath, cwd, args[1:]); code != 0 {
				return exitCodeError(code)
			}
			return nil
		},
	}
	// Wrapper scripts pass their own path, so discovery is anchored like argv[0] dispatch.
	cmd.Flags().StringVar(&shimPath, "shim", "", "path of the wrapper script this call comes from")
	_ = cmd.Flags().MarkHidden("shim")
	// Everything after the alias name belongs to the aliased command.
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func printList(cmd *cobra.Command, label string, items []string) {
	if len(items) > 0 {
		cmd.Printf("%s: %s\n", label, strings.Join(items, ", "))
	}
}

func inPath(environ []string, dir string) bool {
	for _, kv := range environ {
		if v, ok := strings.CutPrefix(kv, "PATH="); ok {
			return slices.Contains(filepath.SplitList(v), dir)
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
