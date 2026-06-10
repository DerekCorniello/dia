package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/state"
)

func newStartCmd() *cobra.Command {
	var cwdFlag string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "start <name>",
		Short: "Start a workspace",
		Long:  "Start the named workspace. All apps are launched concurrently; the command returns as soon as the launch has been dispatched. Use `dia status` to see the running instance.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			w, src, err := resolveWorkspace(name)
			if err != nil {
				return err
			}
			if cwdFlag != "" {
				for i := range w.Apps {
					if w.Apps[i].Cwd == "" {
						w.Apps[i].Cwd = cwdFlag
					}
				}
			}

			out := newOutput(cmd)
			if dryRun {
				if out.IsJSON() {
					return out.JSON(map[string]any{
						"workspace": name,
						"dry_run":   true,
						"apps":      w.Apps,
						"source":    src.Path,
					})
				}
				_ = out.Printf("%s (dry run)\n", name)
				for _, a := range w.Apps {
					_ = out.Printf("  %-10s %s\n", a.Type, a.Cmd)
				}
				return nil
			}

			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			inst, err := s.Runtime.Start(w, src)
			if err != nil {
				return fmt.Errorf("start: %w", err)
			}
			if out.IsJSON() {
				return out.JSON(map[string]any{
					"id":             inst.ID,
					"workspace":      inst.WorkspaceName,
					"workspace_path": inst.WorkspacePath,
					"started_at":     inst.StartedAt,
					"status":         inst.Status,
					"apps":           inst.Apps,
				})
			}
			if err := out.Printf("started %s [%s]\n", inst.WorkspaceName, inst.ID); err != nil {
				return err
			}
			for _, a := range inst.Apps {
				switch a.Status {
				case state.StatusRunning:
					if a.PID > 0 {
						if err := out.Printf("  ok   %-10s pid=%d  %s\n", a.Type, a.PID, a.Cmd); err != nil {
							return err
						}
					} else {
						if err := out.Printf("  ok   %-10s url     %s\n", a.Type, a.Cmd); err != nil {
							return err
						}
					}
				default:
					if err := out.Printf("  fail %-10s        %s  (%s)\n", a.Type, a.Cmd, a.Err); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&cwdFlag, "cwd", "", "override the cwd of every app in the workspace")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "resolve and print what would launch without executing")
	return cmd
}
