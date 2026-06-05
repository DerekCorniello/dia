package cli

import (
	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/state"
)

func newReconcileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reconcile",
		Short: "Drop PIDs from state that are no longer running",
		Long:  "Walks the state file and marks instances as stopped when their tracked PIDs are gone. Normally called automatically on dia startup; useful after killing processes by hand.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			out := newOutput(cmd)
			before := len(s.Runtime.Instances())
			runningBefore := runningCount(s.Store)
			if err := s.Runtime.Reconcile(); err != nil {
				return err
			}
			runningAfter := runningCount(s.Store)
			stopped := runningBefore - runningAfter
			if out.IsJSON() {
				return out.JSON(map[string]int{"reconciled": stopped, "remaining": runningAfter, "total": before})
			}
			return out.Printf("reconciled %d instance(s); %d running\n", stopped, runningAfter)
		},
	}
}

func runningCount(st *state.Store) int {
	n := 0
	for _, inst := range st.Snapshot().Instances {
		if inst.Status == state.StatusRunning {
			n++
		}
	}
	return n
}
