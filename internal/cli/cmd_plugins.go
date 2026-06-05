package cli

import (
	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/diag"
)

func newPluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "List discovered dia-* plugins on PATH",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := newOutput(cmd)
			plugins := diag.ScanPlugins()
			if out.IsJSON() {
				return out.JSON(plugins)
			}
			if len(plugins) == 0 {
				return out.Println("no dia-* plugins found on PATH")
			}
			for _, p := range plugins {
				if err := out.Println(p); err != nil {
					return err
				}
			}
			return nil
		},
	}
	return cmd
}
