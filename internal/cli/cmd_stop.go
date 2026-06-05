package cli

import (
	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/state"
)

func newStopCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "stop <name>|--all",
		Short: "Stop a workspace",
		Long:  "Stop a running workspace by name, or pass --all to stop every running instance. With --force, processes are killed immediately; otherwise dia sends SIGTERM and waits up to 5 seconds before escalating to SIGKILL.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			out := newOutput(cmd)
			stopAll, _ := cmd.Flags().GetBool("all")

			if stopAll {
				if err := s.Runtime.StopAll(force); err != nil {
					return err
				}
				if !out.IsJSON() {
					return out.Println("stopped all")
				}
				return out.JSON(map[string]any{"stopped": "all"})
			}
			if len(args) == 0 {
				return errInvalidArgs
			}
			name := args[0]
			ids := instancesByName(s.Store, name)
			if len(ids) == 0 {
				return &NotFoundError{What: "running workspace " + name}
			}
			for _, id := range ids {
				if err := s.Runtime.Stop(id, force); err != nil {
					return err
				}
			}
			if out.IsJSON() {
				return out.JSON(map[string]any{"stopped": ids})
			}
			return out.Printf("stopped %s\n", name)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "kill processes immediately (SIGKILL/taskkill /F) instead of graceful")
	cmd.Flags().Bool("all", false, "stop every running workspace")
	return cmd
}

func instancesByName(st *state.Store, name string) []string {
	var out []string
	for id, inst := range st.Snapshot().Instances {
		if inst.WorkspaceName == name && inst.Status == state.StatusRunning {
			out = append(out, id)
		}
	}
	return out
}
