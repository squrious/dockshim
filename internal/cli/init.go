package cli

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/squrious/dockshim/internal/config"
)

//go:embed init.yaml
var initTemplate []byte

func newInitCmd(e *Env) *cobra.Command {
	var dir string
	var flat, force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create an initial configuration file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := e.cwd()
			if err != nil {
				return err
			}
			root := cwd
			if dir != "" {
				root = dir
				if !filepath.IsAbs(dir) {
					root = filepath.Join(cwd, dir)
				}
			}
			if fi, err := os.Stat(root); err != nil {
				return err
			} else if !fi.IsDir() {
				return errors.New(root + " is not a directory")
			}

			target := filepath.Join(root, config.DirFile)
			if flat {
				target = filepath.Join(root, config.FlatFile)
			}
			for _, c := range config.Candidates {
				existing := filepath.Join(root, c)
				if _, err := os.Stat(existing); err != nil {
					continue
				}
				switch {
				case existing != target:
					return fmt.Errorf("%s already exists, a second config file would be ambiguous", existing)
				case !force:
					return fmt.Errorf("%s already exists, use --force to overwrite", existing)
				}
			}

			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(target, initTemplate, 0o644); err != nil {
				return err
			}
			cmd.Printf("created %s\n", target)

			if !flat {
				gitignore := filepath.Join(root, config.DirName, ".gitignore")
				if _, err := os.Stat(gitignore); errors.Is(err, os.ErrNotExist) {
					if err := os.WriteFile(gitignore, []byte("/bin/\n"), 0o644); err != nil {
						return err
					}
					cmd.Printf("created %s\n", gitignore)
				}
			}
			cmd.Println("\nNext: declare aliases, then run `dockshim install`.")
			return nil
		},
	}
	cmd.Flags().StringVarP(&dir, "dir", "d", "", "project directory (default: current directory)")
	cmd.Flags().BoolVar(&flat, "flat", false, "write "+config.FlatFile+" instead of "+config.DirFile)
	cmd.Flags().BoolVarP(&force, "force", "f", false, "overwrite an existing config file")
	return cmd
}
