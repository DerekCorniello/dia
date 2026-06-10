package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/DerekCorniello/dia/internal/plugins"
	"github.com/DerekCorniello/dia/internal/state"
)

// newPluginCmd returns the parent `dia plugin` command and its
// subcommands. The plugin host is in the GUI; the CLI focuses on
// authoring and bookkeeping: new/list/install/uninstall/enable/
// disable/info. Each subcommand uses the global plugins dir from
// the XDG state dir (or --state-dir).
func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage dia plugins",
		Long:  "Scaffold, install, list, and toggle plugins. The plugin host runs in the GUI; the CLI edits on-disk state and the plugin directories.",
	}
	cmd.AddCommand(
		newPluginNewCmd(),
		newPluginListCmd(),
		newPluginInstallCmd(),
		newPluginUninstallCmd(),
		newPluginEnableCmd(),
		newPluginDisableCmd(),
		newPluginInfoCmd(),
	)
	return cmd
}

func newPluginNewCmd() *cobra.Command {
	var local bool
	cmd := &cobra.Command{
		Use:   "new <id>",
		Short: "Scaffold a new plugin",
		Long:  "Write a starter plugin.json and index.js under the global (or local, with --local) plugins dir. The plugin id must be lowercase alphanumerics/dashes, 1-40 chars.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if !isValidPluginID(id) {
				return fmt.Errorf("invalid id %q: must match ^[a-z0-9]([a-z0-9-]{1,38}[a-z0-9])?$", id)
			}
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			base := plugins.GlobalPluginsDir(s.StateDir)
			if local {
				cwd, _ := os.Getwd()
				base = plugins.LocalPluginsDir(cwd)
			}
			if err := os.MkdirAll(base, 0o755); err != nil {
				return err
			}
			dst := filepath.Join(base, id)
			if _, err := os.Stat(dst); err == nil {
				return fmt.Errorf("%w: %s", errAlreadyExists, dst)
			}
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			manifest := starterPluginManifest(id)
			if err := os.WriteFile(filepath.Join(dst, "plugin.json"), []byte(manifest), 0o644); err != nil {
				return err
			}
			entry := starterPluginEntry(id)
			if err := os.WriteFile(filepath.Join(dst, "index.js"), []byte(entry), 0o644); err != nil {
				return err
			}
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]string{"path": dst})
			}
			return out.Printf("wrote %s\n", dst)
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "scaffold into ./.dia/plugins in the current directory instead of the global plugins dir")
	return cmd
}

func newPluginListCmd() *cobra.Command {
	var local bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		Long:  "List every plugin discovered under the global (and, with --local, current directory's .dia/plugins) plugin directories.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			cwd := ""
			if local {
				var err error
				cwd, err = os.Getwd()
				if err != nil {
					return err
				}
			}
			infos, err := discoverForCLI(plugins.GlobalPluginsDir(s.StateDir), cwd)
			if err != nil {
				return err
			}
			sort.Slice(infos, func(i, j int) bool {
				if infos[i].ID != infos[j].ID {
					return infos[i].ID < infos[j].ID
				}
				return infos[i].Source < infos[j].Source
			})
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(infos)
			}
			if len(infos) == 0 {
				return out.Printf("no plugins installed\n")
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tNAME\tVERSION\tSOURCE\tSTATUS")
			for _, p := range infos {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", p.ID, p.Name, p.Version, p.Source, p.Status)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "include the current directory's .dia/plugins in the scan")
	return cmd
}

func newPluginInstallCmd() *cobra.Command {
	var local bool
	cmd := &cobra.Command{
		Use:   "install <path>",
		Short: "Install a plugin from a local path",
		Long:  "Copy <path> (which must contain plugin.json) into the global plugins dir. With --local, copy into ./.dia/plugins instead.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			host := &nullHost{}
			mgr, err := plugins.NewManager(plugins.GlobalPluginsDir(s.StateDir), host)
			if err != nil {
				return err
			}
			var dst string
			if local {
				cwd, _ := os.Getwd()
				if cwd == "" {
					return fmt.Errorf("cannot determine current directory")
				}
				dst, err = mgr.InstallLocal(args[0], cwd)
			} else {
				dst, err = mgr.Install(args[0])
			}
			if err != nil {
				return err
			}
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]string{"path": dst})
			}
			return out.Printf("installed to %s\n", dst)
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "install into ./.dia/plugins instead of the global plugins dir")
	return cmd
}

func newPluginUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall <id>",
		Short: "Remove an installed plugin",
		Long:  "Delete the plugin directory and forget its persisted state. The plugin's goja runtime is stopped if it was running in the GUI.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			host := &nullHost{}
			mgr, err := plugins.NewManager(plugins.GlobalPluginsDir(s.StateDir), host)
			if err != nil {
				return err
			}
			if err := mgr.Discover(); err != nil {
				return err
			}
			if err := mgr.Uninstall(args[0]); err != nil {
				return err
			}
			if err := s.Store.Mutate(func(d *state.Data) {
				delete(d.Plugins, args[0])
			}); err != nil {
				return err
			}
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]string{"id": args[0]})
			}
			return out.Printf("uninstalled %s\n", args[0])
		},
	}
	return cmd
}

func newPluginEnableCmd() *cobra.Command {
	var caps string
	cmd := &cobra.Command{
		Use:   "enable <id>",
		Short: "Mark a plugin enabled",
		Long:  "Persist enabled=true for the plugin and (with --caps) a comma-separated granted capability list. The GUI picks this up on next start.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			host := &nullHost{}
			mgr, err := plugins.NewManager(plugins.GlobalPluginsDir(s.StateDir), host)
			if err != nil {
				return err
			}
			if err := mgr.Discover(); err != nil {
				return err
			}
			loaded, ok := mgr.Loaded(args[0])
			if !ok {
				return &NotFoundError{What: "plugin " + args[0]}
			}
			grants := parseCaps(caps)
			if grants == nil {
				grants = plugins.DefaultReadCapabilities()
			}
			grants = plugins.GrantCapabilities(loaded.Manifest.Capabilities, grants)
			if err := s.Store.Mutate(func(d *state.Data) {
				if d.Plugins == nil {
					d.Plugins = map[string]state.PluginState{}
				}
				d.Plugins[args[0]] = state.PluginState{Enabled: true, GrantedCapabilities: grants}
			}); err != nil {
				return err
			}
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]any{"id": args[0], "grants": grants})
			}
			return out.Printf("enabled %s with %d capabilities\n", args[0], len(grants))
		},
	}
	cmd.Flags().StringVar(&caps, "caps", "", "comma-separated granted capabilities (default: all read caps)")
	return cmd
}

func newPluginDisableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable <id>",
		Short: "Mark a plugin disabled",
		Long:  "Persist enabled=false for the plugin. The GUI will not start its goja runtime on next launch.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			if err := s.Store.Mutate(func(d *state.Data) {
				if d.Plugins == nil {
					d.Plugins = map[string]state.PluginState{}
				}
				ps := d.Plugins[args[0]]
				ps.Enabled = false
				d.Plugins[args[0]] = ps
			}); err != nil {
				return err
			}
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]string{"id": args[0]})
			}
			return out.Printf("disabled %s\n", args[0])
		},
	}
	return cmd
}

func newPluginInfoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info <id>",
		Short: "Show plugin details",
		Long:  "Print the plugin's manifest, source dir, and persisted state.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			host := &nullHost{}
			mgr, err := plugins.NewManager(plugins.GlobalPluginsDir(s.StateDir), host)
			if err != nil {
				return err
			}
			if err := mgr.Discover(); err != nil {
				return err
			}
			loaded, ok := mgr.Loaded(args[0])
			if !ok {
				return &NotFoundError{What: "plugin " + args[0]}
			}
			snap := s.Store.Snapshot()
			ps := snap.Plugins[args[0]]
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]any{
					"manifest":   loaded.Manifest,
					"dir":        loaded.Dir,
					"source":     loaded.Source,
					"enabled":    ps.Enabled,
					"grants":     ps.GrantedCapabilities,
					"last_error": loaded.LastError,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "id:      %s\n", loaded.Manifest.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "name:    %s\n", loaded.Manifest.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "version: %s\n", loaded.Manifest.Version)
			fmt.Fprintf(cmd.OutOrStdout(), "source:  %s\n", loaded.Source)
			fmt.Fprintf(cmd.OutOrStdout(), "dir:     %s\n", loaded.Dir)
			fmt.Fprintf(cmd.OutOrStdout(), "ui:      %s (%s)\n", loaded.Manifest.UI.Type, loaded.Manifest.UI.Title)
			fmt.Fprintf(cmd.OutOrStdout(), "caps:    %s\n", strings.Join(loaded.Manifest.Capabilities, ", "))
			fmt.Fprintf(cmd.OutOrStdout(), "enabled: %v\n", ps.Enabled)
			if len(ps.GrantedCapabilities) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "grants:  %s\n", strings.Join(ps.GrantedCapabilities, ", "))
			}
			if loaded.LastError != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "error:   %s\n", loaded.LastError)
			}
			return nil
		},
	}
	return cmd
}

type cliPluginInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"`
	Status  string `json:"status"`
}

func discoverForCLI(globalDir, cwd string) ([]cliPluginInfo, error) {
	host := &nullHost{}
	mgr, err := plugins.NewManager(globalDir, host)
	if err != nil {
		return nil, err
	}
	if cwd != "" {
		mgr.SetLocalDir(cwd)
	}
	if err := mgr.Discover(); err != nil {
		return nil, err
	}
	out := []cliPluginInfo{}
	for _, l := range mgr.List() {
		if l.Manifest == nil {
			continue
		}
		out = append(out, cliPluginInfo{
			ID:      l.Manifest.ID,
			Name:    l.Manifest.Name,
			Version: l.Manifest.Version,
			Source:  string(l.Source),
			Status:  string(l.Status),
		})
	}
	return out, nil
}

func parseCaps(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func isValidPluginID(id string) bool {
	if len(id) < 3 || len(id) > 40 {
		return false
	}
	prevDash := false
	for i, c := range id {
		switch {
		case c >= 'a' && c <= 'z':
			prevDash = false
		case c >= '0' && c <= '9':
			prevDash = false
		case c == '-':
			if i == 0 || i == len(id)-1 || prevDash {
				return false
			}
			prevDash = true
		default:
			return false
		}
	}
	return true
}

func starterPluginManifest(id string) string {
	return fmt.Sprintf(`{
  "id": %q,
  "name": %q,
  "version": "0.1.0",
  "description": "My dia plugin",
  "author": "",
  "entry": "index.js",
  "capabilities": [
    "workspaces:read",
    "instances:read"
  ],
  "ui": {
    "type": "list",
    "title": %q,
    "refreshable": true
  }
}
`, id, id, "Hello from "+id)
}

func starterPluginEntry(id string) string {
	return fmt.Sprintf(`// %s - a starter dia plugin.
// Host API is exposed as the global "dia" object. Capabilities
// declared in plugin.json are gated; calling an ungranted method
// throws. See internal/plugins/hostapi.go for the full surface.
module.exports = {
  // Called by the host before rendering the panel. Return a value
  // shaped to match ui.type from plugin.json.
  getData: function () {
    return [
      { id: "hello", label: "Hello" },
      { id: "world", label: "World" }
    ];
  },

  // Optional: called when the user clicks an action button declared
  // in plugin.json's ui.actions. The host will only invoke actions
  // whose capability is granted to the plugin.
  onAction: function (id) {
    return { id: id, at: new Date().toISOString() };
  }
};
`, id)
}

// nullHost is a plugins.HostAPI that returns empty data and reports
// success for every method. The CLI uses it for plugin discovery
// and scaffolding: plugins are not loaded into goja here, so the
// host surface is never called.
type nullHost struct{}

func (h *nullHost) ListWorkspaces(ctx context.Context) ([]any, error) { return nil, nil }
func (h *nullHost) GetWorkspace(ctx context.Context, name string) (any, error) {
	return nil, nil
}
func (h *nullHost) StartWorkspace(ctx context.Context, name string) (any, error) {
	return nil, nil
}
func (h *nullHost) ListInstances(ctx context.Context) ([]any, error)  { return nil, nil }
func (h *nullHost) StopInstance(ctx context.Context, id string) error { return nil }
func (h *nullHost) StopAll(ctx context.Context) (int, error)          { return 0, nil }
func (h *nullHost) Doctor(ctx context.Context) ([]any, error)         { return nil, nil }
func (h *nullHost) Paths(ctx context.Context) (any, error)            { return nil, nil }
func (h *nullHost) GetTheme(ctx context.Context) (string, error)      { return "", nil }
func (h *nullHost) SetTheme(ctx context.Context, name string) error   { return nil }
func (h *nullHost) ListCustomThemes(ctx context.Context) ([]any, error) {
	return nil, nil
}
func (h *nullHost) SetCustomTheme(ctx context.Context, info any) error { return nil }
func (h *nullHost) DeleteCustomTheme(ctx context.Context, name string) error {
	return nil
}
func (h *nullHost) NewWorkspace(ctx context.Context, name string) (string, error) {
	return "", nil
}
