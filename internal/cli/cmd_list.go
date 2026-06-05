package cli

import (
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/config"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List discovered workspaces",
		Long:    "Print every workspace found in the global config dir and in the current directory (or its parents). Project-local workspaces shadow global ones of the same name.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := newOutput(cmd)
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			all, err := config.Discover(config.DiscoverOptions{
				GlobalDir: config.DefaultGlobalDir(),
				CWD:       cwd,
			})
			if err != nil {
				return err
			}
			sort.Slice(all, func(i, j int) bool {
				if all[i].Workspace.Name != all[j].Workspace.Name {
					return all[i].Workspace.Name < all[j].Workspace.Name
				}
				return all[i].Local == all[j].Local
			})
			if out.IsJSON() {
				type row struct {
					Name    string `json:"name"`
					Apps    int    `json:"apps"`
					Source  string `json:"source"`
					Path    string `json:"path"`
					Version int    `json:"version"`
				}
				rows := make([]row, 0, len(all))
				for _, w := range all {
					kind := "global"
					if w.Local {
						kind = "local"
					}
					rows = append(rows, row{
						Name:    w.Workspace.Name,
						Apps:    len(w.Workspace.Apps),
						Source:  kind,
						Path:    w.Path,
						Version: w.Workspace.Version,
					})
				}
				return out.JSON(rows)
			}
			if len(all) == 0 {
				return out.Println("no workspaces found")
			}
			for _, w := range all {
				kind := "global"
				if w.Local {
					kind = "local"
				}
				if err := out.Printf("%-20s %3d app(s)  %-6s  %s\n",
					w.Workspace.Name, len(w.Workspace.Apps), kind, w.Path); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
