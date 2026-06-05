package cli

import (
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show running workspace instances",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			out := newOutput(cmd)
			insts := s.Runtime.Instances()
			sort.SliceStable(insts, func(i, j int) bool {
				if insts[i].Status != insts[j].Status {
					return insts[i].Status == "running"
				}
				return insts[i].StartedAt.After(insts[j].StartedAt)
			})
			if out.IsJSON() {
				return out.JSON(insts)
			}
			if len(insts) == 0 {
				return out.Println("no instances")
			}
			for _, inst := range insts {
				if err := out.Printf("%-20s %-12s %-12s %s [%s]\n",
					inst.WorkspaceName, inst.Status,
					inst.StartedAt.Format("15:04:05"), inst.WorkspacePath, inst.ID); err != nil {
					return err
				}
				for _, a := range inst.Apps {
					pidOrUrl := a.Cmd
					if a.PID > 0 {
						pidOrUrl = "pid=" + strconv.Itoa(a.PID)
					}
					if err := out.Printf("    %-10s %-10s %-8s %s\n", a.Type, a.Status, pidOrUrl, a.Cmd); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
}
