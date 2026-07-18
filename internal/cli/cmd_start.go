package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/registry"
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
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if dryRun {
				return printDryRun(out, name, src.Path, w, s)
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

// dryRunApp is one resolved app in a --dry-run report.
type dryRunApp struct {
	Type string `json:"type"`
	// Action is "launch" or "open".
	Action string `json:"action"`
	Cmd    string `json:"cmd,omitempty"`
	URL    string `json:"url,omitempty"`
	Cwd    string `json:"cwd,omitempty"`
	// Plugin names the plugin that resolved this app, when the type
	// is not built in.
	Plugin string `json:"plugin,omitempty"`
	Error  string `json:"error,omitempty"`
}

// printDryRun resolves every app through the registry and reports what
// would happen, without launching anything. Resolution is the part
// worth previewing: for a plugin-provided type the command is computed
// by the plugin, so printing the raw config would show nothing useful.
//
// A resolution failure is reported per app rather than aborting, so one
// broken app does not hide the rest -- the same policy the runtime uses
// when actually starting.
func printDryRun(out *output, name, srcPath string, w *config.Workspace, s *setup) error {
	pluginFor := map[string]string{}
	if s.Plugins != nil {
		pluginFor = s.Plugins.AppTypes()
	}

	apps := make([]dryRunApp, 0, len(w.Apps))
	for _, a := range w.Apps {
		entry := dryRunApp{Type: a.Type, Cwd: a.Cwd, Plugin: pluginFor[a.Type]}
		action, err := s.Reg.Resolve(a)
		if err != nil {
			entry.Error = err.Error()
			apps = append(apps, entry)
			continue
		}
		switch action.Kind {
		case registry.ActionOpenURL:
			entry.Action = "open"
			entry.URL = action.URL
		case registry.ActionLaunch:
			entry.Action = "launch"
			entry.Cmd = strings.TrimSpace(action.Launch.Cmd + " " + strings.Join(action.Launch.Args, " "))
			if action.Launch.Cwd != "" {
				entry.Cwd = action.Launch.Cwd
			}
		}
		apps = append(apps, entry)
	}

	if out.IsJSON() {
		payload := map[string]any{
			"workspace": name,
			"dry_run":   true,
			"source":    srcPath,
			"apps":      apps,
		}
		if w.Hooks != nil {
			hooks := map[string][]string{}
			for _, p := range w.Hooks.Phases() {
				if len(p.Cmds) > 0 {
					hooks[p.Name] = p.Cmds
				}
			}
			payload["hooks"] = hooks
		}
		return out.JSON(payload)
	}

	if err := out.Printf("%s (dry run)\n", name); err != nil {
		return err
	}
	for _, p := range w.Hooks.Phases() {
		for _, c := range p.Cmds {
			if err := out.Printf("  hook %-12s %s\n", p.Name, c); err != nil {
				return err
			}
		}
	}
	for _, a := range apps {
		target := a.Cmd
		if a.Action == "open" {
			target = a.URL
		}
		if a.Error != "" {
			if err := out.Printf("  fail %-12s %s\n", a.Type, a.Error); err != nil {
				return err
			}
			continue
		}
		line := fmt.Sprintf("  %-4s %-12s %s", a.Action, a.Type, target)
		if a.Plugin != "" {
			line += fmt.Sprintf("  (via plugin %s)", a.Plugin)
		}
		if err := out.Printf("%s\n", line); err != nil {
			return err
		}
	}
	return nil
}
