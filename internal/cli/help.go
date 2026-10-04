package cli

import (
	"cmp"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/squrious/dockshim/internal/style"
)

// setHelp renders help and usage of root and its subcommands with s. The layout follows cobra's
// default templates, whose functions can't depend on an Env.
func setHelp(root *cobra.Command, s style.Style) {
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		w := cmd.OutOrStdout()
		if desc := strings.TrimSpace(cmp.Or(cmd.Long, cmd.Short)); desc != "" {
			_, _ = fmt.Fprintf(w, "%s\n\n", desc)
		}
		_ = writeUsage(w, s, cmd)
	})
	root.SetUsageFunc(func(cmd *cobra.Command) error {
		return writeUsage(cmd.OutOrStderr(), s, cmd)
	})
}

func writeUsage(w io.Writer, s style.Style, cmd *cobra.Command) error {
	var b strings.Builder
	section := func(title string) {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(s.Title(title) + "\n")
	}

	section("Usage:")
	if cmd.Runnable() {
		b.WriteString("  " + cmd.UseLine() + "\n")
	}
	if cmd.HasAvailableSubCommands() {
		b.WriteString("  " + cmd.CommandPath() + " [command]\n")
	}
	if len(cmd.Aliases) > 0 {
		section("Aliases:")
		b.WriteString("  " + cmd.NameAndAliases() + "\n")
	}
	if cmd.HasExample() {
		section("Examples:")
		b.WriteString(strings.TrimRight(cmd.Example, "\n") + "\n")
	}
	if cmd.HasAvailableSubCommands() {
		section("Commands:")
		var subs []*cobra.Command
		width := 0
		for _, c := range cmd.Commands() {
			if c.IsAvailableCommand() || c.Name() == "help" {
				subs = append(subs, c)
				width = max(width, len(c.Name()))
			}
		}
		for _, c := range subs {
			fmt.Fprintf(&b, "  %s  %s\n", s.Key(fmt.Sprintf("%-*s", width, c.Name())), c.Short)
		}
	}
	if cmd.HasAvailableLocalFlags() {
		section("Flags:")
		b.WriteString(strings.TrimRight(cmd.LocalFlags().FlagUsages(), "\n") + "\n")
	}
	if cmd.HasAvailableInheritedFlags() {
		section("Global Flags:")
		b.WriteString(strings.TrimRight(cmd.InheritedFlags().FlagUsages(), "\n") + "\n")
	}
	if cmd.HasAvailableSubCommands() {
		fmt.Fprintf(&b, "\nUse \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath())
	}
	_, err := io.WriteString(w, b.String())
	return err
}
