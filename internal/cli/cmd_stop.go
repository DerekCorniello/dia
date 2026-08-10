package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/daemon"
)

func newStopCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "stop <name>|--all",
		Short: "Stop a workspace",
		Long:  "Stop a running workspace by name, or pass --all to stop every running instance. With --force, processes are killed immediately; otherwise dia sends SIGTERM and waits up to 5 seconds before escalating to SIGKILL.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := newOutput(cmd)
			stopAll, _ := cmd.Flags().GetBool("all")
			c, err := newDialClient(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = c.Close() }()

			if stopAll {
				var reply struct {
					Stopped []string `json:"stopped"`
				}
				if err := c.Do(daemon.MethodStopAll, map[string]any{"force": force}, &reply); err != nil {
					return err
				}
				if out.IsJSON() {
					return out.JSON(map[string]any{"stopped": reply.Stopped})
				}
				return out.Printf("stopped all (%d instances)\n", len(reply.Stopped))
			}
			if len(args) == 0 {
				return errInvalidArgs
			}
			name := args[0]
			var reply struct {
				Stopped []string `json:"stopped"`
			}
			if err := c.Do(daemon.MethodStop, daemon.StopParams{Name: name, Force: force}, &reply); err != nil {
				msg := err.Error()
				if strings.Contains(msg, "not found") {
					return &NotFoundError{What: "running workspace " + name}
				}
				return err
			}
			if out.IsJSON() {
				return out.JSON(map[string]any{"stopped": reply.Stopped})
			}
			return out.Printf("stopped %s\n", name)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "kill processes immediately (SIGKILL/taskkill /F) instead of graceful")
	cmd.Flags().Bool("all", false, "stop every running workspace")
	return cmd
}
