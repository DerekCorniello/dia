package cli

import (
	"bufio"
	"context"
	"errors"
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
// authoring and bookkeeping: new/list/install/uninstall/info. Each
// subcommand uses the global plugins dir from the XDG state dir (or
// --state-dir).
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
		newPluginUpdateCmd(),
		newPluginUninstallCmd(),
		newPluginInfoCmd(),
		newPluginEnableCmd(),
		newPluginDisableCmd(),
	)
	return cmd
}

func newPluginNewCmd() *cobra.Command {
	var local bool
	var kind string
	cmd := &cobra.Command{
		Use:   "new <id>",
		Short: "Scaffold a new plugin",
		Long: "Write a starter plugin.json and index.js under the global (or local, with --local) " +
			"plugins dir. The plugin id must be lowercase alphanumerics/dashes, 1-40 chars.\n\n" +
			"--type=panel (the default) scaffolds a plugin that renders a list panel in the GUI.\n" +
			"--type=app scaffolds a plugin that provides a workspace app type: it exports " +
			"resolveApp(app) and claims a `type:` name workspaces can use. That needs the " +
			"mutating apps:resolve capability, so grant it with `dia plugin enable <id> " +
			"--caps apps:resolve` before the type will resolve.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if !isValidPluginID(id) {
				return fmt.Errorf("invalid id %q: must match ^[a-z0-9]([a-z0-9-]{1,38}[a-z0-9])?$", id)
			}
			if kind != "panel" && kind != "app" {
				return fmt.Errorf("invalid --type %q: must be panel or app", kind)
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
			manifest, entry := starterPluginManifest(id), starterPluginEntry(id)
			if kind == "app" {
				manifest, entry = starterAppTypeManifest(id), starterAppTypeEntry(id)
			}
			if err := os.WriteFile(filepath.Join(dst, "plugin.json"), []byte(manifest), 0o644); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(dst, "index.js"), []byte(entry), 0o644); err != nil {
				return err
			}
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]string{"path": dst, "type": kind})
			}
			if err := out.Printf("wrote %s\n", dst); err != nil {
				return err
			}
			if kind == "app" {
				// The type stays unresolvable until the capability is
				// granted, and there is nothing in the scaffold to hint
				// at that, so say it here.
				return out.Printf("grant it with: dia plugin enable %s --caps apps:resolve\n", id)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "scaffold into ./.dia/plugins in the current directory instead of the global plugins dir")
	cmd.Flags().StringVar(&kind, "type", "panel", "what to scaffold: panel (a GUI panel) or app (a workspace app type)")
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
	var ref string
	var assumeYes bool
	cmd := &cobra.Command{
		Use:   "install <path|url>",
		Short: "Install a plugin from a local path or a git repository",
		Long: "Install a plugin (a directory containing plugin.json) into the global plugins dir, " +
			"or into ./.dia/plugins with --local.\n\n" +
			"<path|url> is either an existing local directory or a git remote: " +
			"https://host/owner/repo, git@host:owner/repo, or host.tld/owner/repo. " +
			"Git sources are shallow-cloned with the system git, and --ref selects a branch or tag.\n\n" +
			"Installing a plugin puts code on your machine that dia will run. The plugin's " +
			"requested capabilities are shown before anything is copied, and confirmation is " +
			"required when any of them are mutating.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			mgr, err := plugins.NewManager(plugins.GlobalPluginsDir(s.StateDir), &nullHost{})
			if err != nil {
				return err
			}
			src, err := plugins.ParseInstallSource(args[0], ref)
			if err != nil {
				return err
			}
			cwd := ""
			if local {
				if cwd, _ = os.Getwd(); cwd == "" {
					return fmt.Errorf("cannot determine current directory")
				}
			}

			// Materialize once: the manifest shown to the user must be
			// the one that gets installed, and cloning a second time
			// could fetch different code.
			manifest, dir, cleanup, err := mgr.InspectSource(src)
			if err != nil {
				return err
			}
			defer cleanup()

			out := newOutput(cmd)
			if err := confirmInstall(cmd, out, manifest, assumeYes); err != nil {
				return err
			}

			var prov *plugins.Provenance
			if src.Kind == plugins.InstallGit {
				prov = &plugins.Provenance{URL: src.URL, Ref: src.Ref}
			}
			dst, err := mgr.InstallMaterialized(dir, local, cwd, prov)
			if err != nil {
				return err
			}
			if out.IsJSON() {
				return out.JSON(map[string]string{"path": dst, "id": manifest.ID})
			}
			return out.Printf("installed %s to %s\n", manifest.ID, dst)
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "install into ./.dia/plugins instead of the global plugins dir")
	cmd.Flags().StringVar(&ref, "ref", "", "branch or tag to clone (git sources only)")
	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "skip the capability confirmation prompt")
	return cmd
}

func newPluginUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Re-clone a git-installed plugin",
		Long: "Fetch the plugin's recorded git source again and replace the installed copy. " +
			"Granted capabilities and enabled state are preserved. Plugins installed from a " +
			"local path have no recorded source and cannot be updated this way.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			mgr, err := plugins.NewManager(plugins.GlobalPluginsDir(s.StateDir), &nullHost{})
			if err != nil {
				return err
			}
			if cwd, _ := os.Getwd(); cwd != "" {
				mgr.SetLocalDir(cwd)
			}
			if err := mgr.Discover(); err != nil {
				return err
			}
			dst, err := mgr.Update(args[0])
			if err != nil {
				return err
			}
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]string{"path": dst, "id": args[0]})
			}
			return out.Printf("updated %s at %s\n", args[0], dst)
		},
	}
	return cmd
}

// newPluginEnableCmd persists a plugin's enabled flag and granted
// capabilities. This is the only way to grant a mutating capability
// from the CLI: discovery and install grant read-only capabilities
// only, no matter what the manifest requests.
func newPluginEnableCmd() *cobra.Command {
	var caps string
	cmd := &cobra.Command{
		Use:   "enable <id>",
		Short: "Enable a plugin and set its granted capabilities",
		Long: "Mark a plugin enabled in the persisted state. --caps takes a comma-separated " +
			"capability list; anything the manifest does not request is dropped. Without " +
			"--caps the plugin keeps the read-only defaults.\n\n" +
			"Granting a mutating capability lets the plugin change your system: cmd:exec runs " +
			"commands, and apps:resolve lets it decide what a workspace app type launches.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			mgr, err := discoveredManager(s)
			if err != nil {
				return err
			}
			l, ok := mgr.Get(id)
			if !ok || l.Manifest == nil {
				return &NotFoundError{What: "plugin " + id}
			}

			granted := plugins.DefaultReadCapabilities()
			if caps != "" {
				granted = splitCaps(caps)
			}
			granted = plugins.GrantCapabilities(l.Manifest.Capabilities, granted)

			if err := s.Store.Mutate(func(d *state.Data) {
				prev := d.Plugins[id]
				d.Plugins[id] = state.PluginState{
					Enabled:             true,
					GrantedCapabilities: granted,
					Config:              prev.Config,
				}
			}); err != nil {
				return err
			}

			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]any{"id": id, "enabled": true, "granted": granted})
			}
			if len(granted) == 0 {
				return out.Printf("enabled %s with no capabilities\n", id)
			}
			return out.Printf("enabled %s with %s\n", id, strings.Join(granted, ", "))
		},
	}
	cmd.Flags().StringVar(&caps, "caps", "", "comma-separated capabilities to grant")
	return cmd
}

func newPluginDisableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "disable <id>",
		Short: "Disable a plugin",
		Long:  "Mark a plugin disabled in the persisted state. Granted capabilities are kept so re-enabling does not re-prompt.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			s, err := newSetup(flagsFromCmd(cmd).StateDir, cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			mgr, err := discoveredManager(s)
			if err != nil {
				return err
			}
			if _, ok := mgr.Get(id); !ok {
				return &NotFoundError{What: "plugin " + id}
			}
			if err := s.Store.Mutate(func(d *state.Data) {
				prev := d.Plugins[id]
				prev.Enabled = false
				d.Plugins[id] = prev
			}); err != nil {
				return err
			}
			out := newOutput(cmd)
			if out.IsJSON() {
				return out.JSON(map[string]any{"id": id, "enabled": false})
			}
			return out.Printf("disabled %s\n", id)
		},
	}
	return cmd
}

// discoveredManager returns the setup's plugin manager, falling back to
// a freshly discovered one when the setup could not build it.
func discoveredManager(s *setup) (*plugins.Manager, error) {
	if s.Plugins != nil {
		return s.Plugins, nil
	}
	mgr, err := plugins.NewManager(plugins.GlobalPluginsDir(s.StateDir), &nullHost{})
	if err != nil {
		return nil, err
	}
	if cwd, _ := os.Getwd(); cwd != "" {
		mgr.SetLocalDir(cwd)
	}
	if err := mgr.Discover(); err != nil {
		return nil, err
	}
	return mgr, nil
}

func splitCaps(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// confirmInstall shows what the plugin will be allowed to do and, when
// any requested capability is mutating, requires an explicit yes.
// Installing runs someone else's code; the capability list is the only
// signal the user gets about what that code can reach.
func confirmInstall(cmd *cobra.Command, out *output, m *plugins.Manifest, assumeYes bool) error {
	mutating := make([]string, 0, len(m.Capabilities))
	for _, c := range m.Capabilities {
		if plugins.IsMutatingCapability(c) {
			mutating = append(mutating, c)
		}
	}
	if out.IsJSON() || assumeYes {
		if len(mutating) > 0 && !assumeYes {
			return fmt.Errorf("plugin %q requests mutating capabilities (%s); "+
				"re-run with --yes to install it non-interactively",
				m.ID, strings.Join(mutating, ", "))
		}
		return nil
	}

	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "%s %s", m.Name, m.Version)
	if m.Author != "" {
		fmt.Fprintf(w, " by %s", m.Author)
	}
	fmt.Fprintln(w)
	if m.Description != "" {
		fmt.Fprintf(w, "  %s\n", m.Description)
	}
	if len(m.Capabilities) == 0 {
		fmt.Fprintln(w, "  requests no capabilities")
	} else {
		fmt.Fprintln(w, "  requests:")
		for _, c := range m.Capabilities {
			mark := ""
			if plugins.IsMutatingCapability(c) {
				mark = "  (mutating)"
			}
			fmt.Fprintf(w, "    %s%s\n", c, mark)
		}
	}
	if len(mutating) == 0 {
		return nil
	}
	fmt.Fprintf(w, "\nThis plugin asks for capabilities that change your system. Install it? [y/N] ")
	return readYesNo(cmd)
}

// readYesNo reads a single confirmation line from the command's input.
func readYesNo(cmd *cobra.Command) error {
	reader := bufio.NewReader(cmd.InOrStdin())
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return errors.New("install cancelled")
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return nil
	default:
		return errors.New("install cancelled")
	}
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
			fmt.Fprintf(cmd.OutOrStdout(), "id:      %s\n", loaded.ID)
			fmt.Fprintf(cmd.OutOrStdout(), "source:  %s\n", loaded.Source)
			fmt.Fprintf(cmd.OutOrStdout(), "dir:     %s\n", loaded.Dir)
			// `plugin info` is how you diagnose a plugin that is not
			// working, so it has to survive a broken manifest and say
			// what is wrong with it.
			if loaded.Manifest == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "status:  %s\n", loaded.Status)
				fmt.Fprintf(cmd.OutOrStdout(), "error:   %s\n", loaded.LastError)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "name:    %s\n", loaded.Manifest.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "version: %s\n", loaded.Manifest.Version)
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
		info := cliPluginInfo{
			ID:     l.ID,
			Source: string(l.Source),
			Status: string(l.Status),
		}
		// A plugin whose manifest is broken still gets listed: hiding
		// it leaves the user with a directory that does nothing and no
		// indication why.
		if l.Manifest != nil {
			info.Name = l.Manifest.Name
			info.Version = l.Manifest.Version
		} else {
			info.Name = "(invalid manifest)"
		}
		out = append(out, info)
	}
	return out, nil
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
func (h *nullHost) Exec(ctx context.Context, cmd string, args []string) (string, error) {
	return "", nil
}
func (h *nullHost) Fetch(ctx context.Context, url string, opts map[string]any) (any, error) {
	return nil, nil
}
func (h *nullHost) NewWorkspace(ctx context.Context, name string) (string, error) {
	return "", nil
}

// starterAppTypeManifest scaffolds a plugin that provides a workspace
// app type rather than a panel. The claimed type is the plugin id, so
// the scaffold is usable the moment apps:resolve is granted.
func starterAppTypeManifest(id string) string {
	return fmt.Sprintf(`{
  "id": %q,
  "name": %q,
  "version": "0.1.0",
  "description": "Provides the %s workspace app type",
  "author": "",
  "entry": "index.js",
  "capabilities": [
    "apps:resolve"
  ],
  "app_types": [
    %q
  ],
  "ui": {
    "type": "kv",
    "title": %q
  }
}
`, id, id, id, id, id)
}

func starterAppTypeEntry(id string) string {
	return fmt.Sprintf(`// %s - a dia app-type plugin.
//
// This plugin claims the %q workspace app type. Use it like any
// built-in type:
//
//   apps:
//     - type: %s
//       cwd: ~/projects/thing
//
// resolveApp runs in a restricted runtime. The only host calls
// available are dia.getConfig() and dia.pluginDir() -- no workspaces,
// no exec, no fetch. It must be a pure function of the app it is
// handed, which is what keeps "dia start --dry-run" honest.
module.exports = {
  // Called once per app of a type this plugin claims. "app" is the
  // workspace entry as written in YAML: type, label, cmd, args, cwd,
  // env, url.
  //
  // Return { cmd, args?, cwd?, env? } to launch something, or
  // { url } to open a URL. Exactly one of cmd/url; any other key is
  // an error.
  resolveApp: function (app) {
    return {
      cmd: "echo",
      args: ["replace me"].concat(app.args || []),
      cwd: app.cwd,
      env: app.env,
    };
  },

  // The panel is incidental for an app-type plugin, but a manifest
  // needs a ui block and the kv panel calls getData to render.
  getData: function () {
    return { provides: %q };
  },
};
`, id, id, id, id)
}
