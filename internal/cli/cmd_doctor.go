package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"
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

			type check struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				Detail string `json:"detail,omitempty"`
			}
			var checks []check
			ok := func(name, detail string) {
				checks = append(checks, check{Name: name, Status: "ok", Detail: detail})
			}
			warn := func(name, detail string) {
				checks = append(checks, check{Name: name, Status: "warn", Detail: detail})
			}

			checks = append(checks, check{Name: "platform", Status: "ok", Detail: runtime.GOOS + "/" + runtime.GOARCH})

			stateDir := s.Store.Path()
			ok("state dir", filepath.Dir(stateDir))
			ok("state file", stateDir)

			if ghPath, err := exec.LookPath("gh"); err != nil {
				warn("gh cli", "not found; type=gh apps will fail")
			} else {
				ok("gh cli", ghPath)
			}

			openHelper := platformOpenHelper()
			if path, err := exec.LookPath(openHelper); err != nil {
				warn("url handler", openHelper+" not found; type=open and type=browser will fail")
			} else {
				ok("url handler", path)
			}

			plugins := scanPlugins()
			ok("plugin scan", fmt.Sprintf("%d dia-* found on PATH", len(plugins)))

			// Reconcile so the counts we print are current.
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
			if crashed > 0 {
				warn("state", detail)
			} else {
				ok("state", detail)
			}

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
			if len(plugins) > 0 {
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

// platformOpenHelper returns the binary dia uses to open URLs on
// the current OS. Mirrors internal/platform, kept here to avoid an
// import cycle through the platform package's helpers.
func platformOpenHelper() string {
	switch runtime.GOOS {
	case "darwin":
		return "open"
	case "windows":
		return "cmd"
	}
	return "xdg-open"
}

// scanPlugins walks the process PATH looking for executables named
// dia-*. Returns the sorted list of absolute paths.
func scanPlugins() []string {
	dirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	seen := map[string]bool{}
	var out []string
	for _, d := range dirs {
		if d == "" {
			continue
		}
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, "dia-") {
				continue
			}
			full := filepath.Join(d, name)
			info, err := e.Info()
			if err != nil {
				continue
			}
			if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
				continue
			}
			if seen[full] {
				continue
			}
			seen[full] = true
			out = append(out, full)
		}
	}
	sort.Strings(out)
	return out
}
