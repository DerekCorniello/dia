package cli

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/state"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List discovered workspaces",
		Long:    "Print every workspace found in the global config dir, every persisted root, and the current directory (or its parents). Name collisions are shown with full paths.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := newOutput(cmd)
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			var roots []string
			if dir, err := state.ResolveStateDir(); err == nil {
				st, err := state.OpenAt(filepath.Join(dir, state.StateFile))
				if err == nil {
					roots = st.Snapshot().Roots
				}
			}
			all, err := config.Discover(config.DiscoverOptions{
				GlobalDir: config.DefaultGlobalDir(),
				Roots:     roots,
				CWD:       cwd,
			})
			if err != nil {
				return err
			}
			sort.Slice(all, func(i, j int) bool {
				if all[i].Workspace.Name != all[j].Workspace.Name {
					return all[i].Workspace.Name < all[j].Workspace.Name
				}
				return all[i].Path < all[j].Path
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
