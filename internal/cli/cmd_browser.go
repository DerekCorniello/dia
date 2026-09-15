package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/browser"
	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/state"
)

func newBrowserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "browser",
		Short: "Manage dia's owned browser profiles",
		Long: "A workspace `browser` app that names a specific browser is launched " +
			"against a dia-managed copy of your real profile, so dia can close exactly " +
			"the window it opened. The copy is seeded once from your real profile and " +
			"then keeps whatever you do in dia windows. Use `refresh` to re-pull your " +
			"current real-browser logins into it.",
	}
	cmd.AddCommand(newBrowserListCmd())
	cmd.AddCommand(newBrowserRefreshCmd())
	cmd.AddCommand(newBrowserStatusCmd())
	return cmd
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 4 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

func newBrowserStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Aliases: []string{"st"},
		Short:   "Show disk usage of dia-managed browser profiles",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := resolveStateDir(cmd)
			if err != nil {
				return err
			}
			surf, err := browser.NewDedicatedProfile(browser.Options{
				Platform: platform.New(),
				StateDir: dir,
			})
			if err != nil {
				return err
			}
			stats, err := surf.Status()
			if err != nil {
				return err
			}
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]any{"browsers": stats})
			}
			if len(stats) == 0 {
				return out.Println("no dia-managed browser profiles")
			}
			for _, st := range stats {
				if err := out.Printf("%s: seed %s, %d clone(s) %s, %d active session(s)\n",
					st.Browser, formatBytes(st.SeedBytes), st.Clones, formatBytes(st.CloneBytes), st.Active); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newBrowserListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List dia-managed browser profiles",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := resolveStateDir(cmd)
			if err != nil {
				return err
			}
			surf, err := browser.NewDedicatedProfile(browser.Options{
				Platform: platform.New(),
				StateDir: dir,
			})
			if err != nil {
				return err
			}
			targets, err := surf.SeededBrowsers()
			if err != nil {
				return err
			}
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]any{"browsers": targets})
			}
			if len(targets) == 0 {
				return out.Println("no dia-managed browser profiles")
			}
			for _, b := range targets {
				if err := out.Printf("%s\n", b); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newBrowserRefreshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh [browser]",
		Short: "Re-pull dia's browser profile from your real profile",
		Long: "Re-clone the dia-managed profile from your current real browser profile, " +
			"discarding whatever the dia windows had accumulated. This is the deliberate " +
			"way to pick up logins you made in your normal browser -- dia never merges " +
			"the two automatically, because a live browser profile cannot be merged " +
			"safely. With no argument, every seeded browser is refreshed.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := resolveStateDir(cmd)
			if err != nil {
				return err
			}
			surf, err := browser.NewDedicatedProfile(browser.Options{
				Platform: platform.New(),
				StateDir: dir,
			})
			if err != nil {
				return err
			}

			var targets []string
			if len(args) == 1 {
				targets = []string{args[0]}
			} else {
				targets, err = surf.SeededBrowsers()
				if err != nil {
					return err
				}
				if len(targets) == 0 {
					return newOutput(cmd).Println("no dia browser profiles to refresh")
				}
			}

			refreshed := make([]string, 0, len(targets))
			for _, bin := range targets {
				if err := surf.Refresh(bin); err != nil {
					return fmt.Errorf("refresh %s: %w", bin, err)
				}
				refreshed = append(refreshed, bin)
			}

			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]any{"refreshed": refreshed})
			}
			for _, bin := range refreshed {
				_ = out.Printf("refreshed %s\n", bin)
			}
			return nil
		},
	}
}

// resolveStateDir returns the state directory for a command, honoring
// the --state-dir override (which, matching newSetup, is used directly
// as the state dir rather than as the XDG state home).
func resolveStateDir(cmd *cobra.Command) (string, error) {
	if sd := flagsFromCmd(cmd).StateDir; sd != "" {
		return sd, nil
	}
	return state.ResolveStateDir()
}
