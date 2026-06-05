package cli

import (
	"github.com/spf13/cobra"
)

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open <name>",
		Short: "Reveal a workspace config in the file manager",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			out := newOutput(cmd)
			name := args[0]
			_, src, err := resolveWorkspace(name)
			if err != nil {
				return err
			}
			if err := s.PF.RevealInFileManager(src.Path); err != nil {
				return err
			}
			return out.Printf("revealed %s\n", src.Path)
		},
	}
}
