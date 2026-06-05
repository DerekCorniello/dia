package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/version"
)

// Run executes the CLI with the given args (typically os.Args[1:]).
// Returns the process exit code: 0 for success, non-zero for any error.
// The bare `dia` invocation (no args) is handled in main.go and opens
// the GUI; Run is only called when at least one subcommand or flag is
// present.
func Run(args []string) int {
	cmd := newRootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	var showVersion bool
	cmd := &cobra.Command{
		Use:           "dia",
		Short:         "dia: a cross-platform desktop launcher for dev workspaces",
		Long:          "dia defines, starts, and stops dev workspaces from YAML configs. Run `dia` with no arguments to open the desktop launcher.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVersion {
				fmt.Fprintf(cmd.OutOrStdout(), "dia version %s\n", version.Version)
				return nil
			}
			return cmd.Help()
		},
	}
	cmd.Flags().BoolVarP(&showVersion, "version", "V", false, "print version and exit")
	return cmd
}
