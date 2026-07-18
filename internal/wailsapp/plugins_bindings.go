package wailsapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/plugins"
	"github.com/DerekCorniello/dia/internal/state"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// ListPlugins returns every discovered plugin with the host view
// (manifest summary, status, granted capabilities).
func (a *App) ListPlugins() []PluginInfo {
	if a.pmgr == nil {
		return nil
	}
	loaded := a.pmgr.List()
	out := make([]PluginInfo, 0, len(loaded))
	for _, l := range loaded {
		out = append(out, loadedToInfo(l))
	}
	return out
}

// PluginCall invokes an exported method on a plugin's module.exports.
// Args are JSON-marshalled to any and passed through to the plugin.
// The plugin must be enabled.
func (a *App) PluginCall(id, method string, argsJSON string) (string, error) {
	if a.pmgr == nil {
		return "", errors.New("plugin manager not initialized")
	}
	var args []any
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", fmt.Errorf("parse args: %w", err)
		}
	}
	v, err := a.pmgr.Call(id, method, args)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// PluginPaths returns the on-disk locations of the global and local
// plugin directories, for the settings panel.
func (a *App) PluginPaths() PluginPathsInfo {
	out := PluginPathsInfo{}
	if a.pmgr == nil {
		return out
	}
	out.GlobalDir = a.pmgr.GlobalDir()
	out.LocalDir = a.pmgr.LocalDir()
	return out
}

// InstallPluginFromFolder opens a directory picker and installs the
// selected plugin globally. Returns the plugin ID on success.
func (a *App) InstallPluginFromFolder() (string, error) {
	if a.pmgr == nil {
		return "", errors.New("plugin manager not initialized")
	}
	if a.ctx == nil {
		return "", errors.New("not initialized")
	}
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Plugin Directory",
	})
	if err != nil {
		return "", fmt.Errorf("select directory: %w", err)
	}
	if dir == "" {
		return "", nil
	}
	dst, err := a.pmgr.Install(dir)
	if err != nil {
		return "", fmt.Errorf("install plugin: %w", err)
	}
	return dst, nil
}

// OpenPluginFolder reveals the global plugins directory in the file
// manager.
func (a *App) OpenPluginFolder() error {
	if a.pmgr == nil {
		return errors.New("plugin manager not initialized")
	}
	dir := a.pmgr.GlobalDir()
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				return mkErr
			}
		} else {
			return err
		}
	}
	return platform.New().RevealInFileManager(dir)
}

// OpenPluginWindow spawns a second dia process with
// --plugin-window=<id> and returns its PID. The spawned process
// loads the plugin's panel/ folder in a new OS-level window.
// Returns an error if the plugin is missing, is not a window
// plugin (ui.type != "window"), or the OS refuses the exec. The
// main dia process keeps running; closing the parent does not
// close the plugin window.
func (a *App) OpenPluginWindow(id string) (int, error) {
	if id == "" {
		return 0, errors.New("plugin id is required")
	}
	loaded, ok := a.pmgr.Loaded(id)
	if !ok {
		return 0, fmt.Errorf("plugin %q not found", id)
	}
	if loaded.Manifest == nil {
		return 0, fmt.Errorf("plugin %q has no valid manifest", id)
	}
	if loaded.Manifest.UI.Type != "window" {
		return 0, fmt.Errorf("plugin %q is not a window plugin (ui.type=%q)", id, loaded.Manifest.UI.Type)
	}
	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("locate executable: %w", err)
	}
	pf := platform.New()
	handle, err := pf.Launch(platform.LaunchOpts{
		Cmd:  exe,
		Args: []string{"--plugin-window=" + id},
		Env:  os.Environ(),
	})
	if err != nil {
		return 0, fmt.Errorf("spawn plugin window: %w", err)
	}
	return handle.PID(), nil
}

// applyPersistedPluginState starts the goja runtime for every
// discovered plugin so all plugins are immediately usable. A plugin
// whose entry script fails is left in the errored state and the
// frontend will show the LastError.
//
// Capabilities come from the persisted grant list the user approved in
// a previous session, falling back to the read-only defaults for a
// plugin that has never been granted anything. This function used to
// ignore the store entirely and hardcode the read-only set, which
// meant a capability granted from the CLI silently did nothing in the
// GUI.
func (a *App) applyPersistedPluginState(pmgr *plugins.Manager) {
	persisted := map[string]state.PluginState{}
	if a.store != nil {
		persisted = a.store.Snapshot().Plugins
	}
	for _, l := range pmgr.List() {
		if l.Manifest == nil {
			continue
		}
		id := l.Manifest.ID
		granted := plugins.DefaultReadCapabilities()
		if ps, ok := persisted[id]; ok && ps.GrantedCapabilities != nil {
			granted = ps.GrantedCapabilities
		}
		granted = plugins.GrantCapabilities(l.Manifest.Capabilities, granted)
		if err := pmgr.EnableWithGrants(id, granted); err != nil {
			a.logger.Warn("enable plugin", "id", id, "error", err)
		}
	}
}

// GetPluginCapabilities returns what a plugin asks for and what it has
// been granted, so the frontend can render the grant controls without
// re-deriving the capability model.
func (a *App) GetPluginCapabilities(id string) (PluginCapabilityInfo, error) {
	out := PluginCapabilityInfo{Requested: []CapabilityInfo{}, Granted: []string{}}
	if a.pmgr == nil {
		return out, errors.New("plugin manager unavailable")
	}
	l, ok := a.pmgr.Get(id)
	if !ok || l.Manifest == nil {
		return out, fmt.Errorf("plugin %q not found", id)
	}
	for _, c := range l.Manifest.Capabilities {
		out.Requested = append(out.Requested, CapabilityInfo{
			Name:     c,
			Mutating: plugins.IsMutatingCapability(c),
			Granted:  plugins.HasCapability(l.GrantedCaps, c),
		})
	}
	if l.GrantedCaps != nil {
		out.Granted = l.GrantedCaps
	}
	return out, nil
}

// SetPluginCapabilities persists a new grant list for a plugin and
// applies it immediately: the plugin's runtime is rebuilt with the new
// grants and the app-type registry is re-synced, so granting or
// revoking apps:resolve takes effect without a restart.
//
// Capabilities the manifest never requested are dropped rather than
// rejected, so the frontend can send whatever the user toggled.
func (a *App) SetPluginCapabilities(id string, caps []string) error {
	if a.pmgr == nil {
		return errors.New("plugin manager unavailable")
	}
	l, ok := a.pmgr.Get(id)
	if !ok || l.Manifest == nil {
		return fmt.Errorf("plugin %q not found", id)
	}
	granted := plugins.GrantCapabilities(l.Manifest.Capabilities, caps)

	if a.store != nil {
		if err := a.store.Mutate(func(d *state.Data) {
			prev := d.Plugins[id]
			prev.Enabled = true
			prev.GrantedCapabilities = granted
			d.Plugins[id] = prev
		}); err != nil {
			return fmt.Errorf("persist plugin capabilities: %w", err)
		}
	}

	// Disable then re-enable: enableLocked only rebuilds the runtime
	// for a plugin that is not already enabled, and the runtime has to
	// be rebuilt for a changed grant set to reach the bridge.
	if err := a.pmgr.Disable(id); err != nil {
		a.logger.Warn("disable plugin before regrant", "id", id, "error", err)
	}
	if err := a.pmgr.EnableWithGrants(id, granted); err != nil {
		return fmt.Errorf("apply plugin capabilities: %w", err)
	}
	a.registerPluginAppTypes(a.pmgr)
	return nil
}

func loadedToInfo(l plugins.Loaded) PluginInfo {
	out := PluginInfo{
		Source:              string(l.Source),
		Dir:                 l.Dir,
		Enabled:             true,
		Status:              string(l.Status),
		LastError:           l.LastError,
		GrantedCapabilities: l.GrantedCaps,
		Actions:             []PluginActionDef{},
	}
	if l.GrantedCaps == nil {
		out.GrantedCapabilities = []string{}
	}
	if l.Manifest != nil {
		out.ID = l.Manifest.ID
		out.Name = l.Manifest.Name
		out.Version = l.Manifest.Version
		out.Description = l.Manifest.Description
		out.LongDescription = l.Manifest.LongDescription
		out.Author = l.Manifest.Author
		out.UI = PluginUIInfo{
			Type:        l.Manifest.UI.Type,
			Title:       l.Manifest.UI.Title,
			Refreshable: l.Manifest.UI.Refreshable,
		}
		if len(l.Manifest.UI.Columns) > 0 {
			out.UI.Columns = make([]PluginUIColumn, 0, len(l.Manifest.UI.Columns))
			for _, c := range l.Manifest.UI.Columns {
				out.UI.Columns = append(out.UI.Columns, PluginUIColumn{Key: c.Key, Label: c.Label, Format: c.Format})
			}
		}
		out.Actions = make([]PluginActionDef, 0, len(l.Manifest.UI.Actions))
		for _, ad := range l.Manifest.UI.Actions {
			out.Actions = append(out.Actions, PluginActionDef{ID: ad.ID, Label: ad.Label, Confirm: ad.Confirm, Capability: ad.Capability})
		}
		if l.Manifest.ConfigSchema != nil {
			out.ConfigSchema = l.Manifest.ConfigSchema
		}
	}
	return out
}
