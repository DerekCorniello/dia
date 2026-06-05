package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/diag"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Smoke checks for the dia installation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			out := newOutput(cmd)

			stateFile := s.Store.Path()
			stateDir := ""
			if stateFile != "" {
				stateDir = filepath.Dir(stateFile)
			}

			checks := diag.RunChecks(stateDir, stateFile)

			// Reconcile so the running/stopped/crashed counts we
			// print reflect the current set of alive PIDs.
			_ = s.Runtime.Reconcile()
			insts := s.Runtime.Instances()
			running, stopped, crashed := 0, 0, 0
			for _, inst := range insts {
				switch inst.Status {
				case "running":
					running++
				case "stopped":
					stopped++
				case "crashed":
					crashed++
				}
			}
			detail := fmt.Sprintf("running=%d stopped=%d crashed=%d", running, stopped, crashed)
			stateStatus := "ok"
			if crashed > 0 {
				stateStatus = "warn"
			}
			checks = append(checks, diag.Check{Name: "state", Status: stateStatus, Detail: detail})

			if out.IsJSON() {
				return out.JSON(checks)
			}
			for _, c := range checks {
				mark := "[" + c.Status + "]"
				if c.Detail != "" {
					if err := out.Printf("%-6s %-15s %s\n", mark, c.Name, c.Detail); err != nil {
						return err
					}
				} else {
					if err := out.Printf("%-6s %s\n", mark, c.Name); err != nil {
						return err
					}
				}
			}
			if plugins := diag.ScanPlugins(); len(plugins) > 0 {
				if err := out.Println("\ndiscovered plugins:"); err != nil {
					return err
				}
				for _, p := range plugins {
					if err := out.Printf("  %s\n", p); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
}
