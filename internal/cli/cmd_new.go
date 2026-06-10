package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/config"
)

func newNewCmd() *cobra.Command {
	var local bool
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a starter workspace",
		Long:  "Write a starter YAML to the global config dir. With --local, write to ./.dia.yaml in the current directory instead.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := newOutput(cmd)
			name := args[0]

			var wsPath string
			if local {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				wsPath = filepath.Join(cwd, ".dia.yaml")
			} else {
				dir := config.DefaultGlobalDir()
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("mkdir %s: %w", dir, err)
				}
				wsPath = filepath.Join(dir, name+".yaml")
			}

			if _, err := os.Stat(wsPath); err == nil {
				return fmt.Errorf("%w: %s", errAlreadyExists, wsPath)
			}
			content := starterWorkspace(name, local)
			if err := os.WriteFile(wsPath, []byte(content), 0o644); err != nil {
				return fmt.Errorf("write %s: %w", wsPath, err)
			}
			if out.IsJSON() {
				return out.JSON(map[string]string{"path": wsPath})
			}
			return out.Printf("wrote %s\n", wsPath)
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "write to ./.dia.yaml in the current directory instead of the global config dir")
	return cmd
}

// errAlreadyExists is mapped to ExitAlreadyExists by exitCodeFor.
var errAlreadyExists = fmt.Errorf("file already exists")

func starterWorkspace(name string, local bool) string {
	home, _ := os.UserHomeDir()
	cwd := filepath.Join(home, "projects", name)
	if local {
		if d, err := os.Getwd(); err == nil {
			cwd = d
		}
	}
	return fmt.Sprintf(`# Workspace: %s
# Created with `+"`dia new`"+`. Edit apps to your liking, then `+"`dia start %s`"+`.
version: 1
name: %s
apps:
  # Open the project in your editor. `+"`cwd`"+` supports ~ and $VAR.
  - type: editor
    cmd: code .
    cwd: %s

  # Run a long-lived process. dia tracks its PID and cleans it up
  # when you stop the workspace.
  - type: terminal
    cmd: echo "replace with your dev command" && sleep 3600
    cwd: %s

  # Open a URL in the default browser.
  - type: browser
    url: http://localhost:8080
`, name, name, name, cwd, cwd)
}
