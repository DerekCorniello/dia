package cli

import (
	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/daemon"
)

func newReconcileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reconcile",
		Short: "Drop PIDs from state that are no longer running",
		Long:  "Walks the state file and marks instances as stopped when their tracked PIDs are gone. Normally called automatically on dia startup; useful after killing processes by hand.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newDialClient(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = c.Close() }()
			var counts struct {
				Reconciled int `json:"reconciled"`
				Remaining  int `json:"remaining"`
				Total      int `json:"total"`
			}
			if err := c.Do(daemon.MethodReconcile, nil, &counts); err != nil {
				return err
			}
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]int{
					"reconciled": counts.Reconciled,
					"remaining":  counts.Remaining,
					"total":      counts.Total,
				})
			}
			return out.Printf("reconciled %d instance(s); %d running\n", counts.Reconciled, counts.Remaining)
		},
	}
}

func newShutdownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shutdown",
		Short: "Stop the session daemon and all workspaces",
		Long:  "Terminate the daemon, stopping every supervised workspace, and exit. Workspaces no longer persist once the daemon is down; start one again with `dia start`.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newDialClient(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = c.Close() }()
			var reply struct {
				Stopped string `json:"stopped"`
			}
			if err := c.Do(daemon.MethodShutdown, nil, &reply); err != nil {
				return err
			}
			return newOutput(cmd).Println("daemon stopped")
		},
	}
}
