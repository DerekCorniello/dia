package cli

import (
	"sort"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/daemon"
	"github.com/DerekCorniello/dia/internal/state"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show running workspace instances",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newDialClient(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = c.Close() }()
			var insts []state.Instance
			if err := c.Do(daemon.MethodList, nil, &insts); err != nil {
				return err
			}
			sort.SliceStable(insts, func(i, j int) bool {
				if insts[i].Status != insts[j].Status {
					return insts[i].Status == "running"
				}
				return insts[i].StartedAt.After(insts[j].StartedAt)
			})

			out := newOutput(cmd)
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
					var pidOrUrl, cmdCol string
					if a.PID > 0 {
						pidOrUrl = "pid=" + strconv.Itoa(a.PID)
						cmdCol = a.Cmd
					} else {
						// URL apps have no PID; runtime stores the
						// URL in Cmd, so show it as the "pid/url"
						// column and leave the command column empty.
						pidOrUrl = a.Cmd
					}
					if err := out.Printf("    %-10s %-10s %-8s %s\n", a.Type, a.Status, pidOrUrl, cmdCol); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
}
