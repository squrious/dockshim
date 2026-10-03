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
	return &cobra.Command{
		Use:   "init",
		Short: "Create an initial configuration file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := e.cwd()
			if err != nil {
				return err
			}
			for _, c := range config.Candidates {
				existing := filepath.Join(root, c)
				if _, err := os.Stat(existing); err == nil {
					return fmt.Errorf("%s already exists", existing)
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}
			}

			target := filepath.Join(root, config.FileName)
			if err := os.WriteFile(target, initTemplate, 0o644); err != nil {
				return err
			}
			cmd.Printf("created %s\n", target)
			cmd.Println("\nNext: declare aliases, then run `dockshim install`.")
			return nil
		},
	}
}
