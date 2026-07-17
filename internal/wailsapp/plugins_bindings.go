package wailsapp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/plugins"
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
func (a *App) applyPersistedPluginState(pmgr *plugins.Manager) {
	for _, l := range pmgr.List() {
		granted := plugins.DefaultReadCapabilities()
		if l.Manifest != nil {
			granted = plugins.GrantCapabilities(l.Manifest.Capabilities, granted)
		}
		if err := pmgr.EnableWithGrants(l.Manifest.ID, granted); err != nil {
			a.logger.Warn("enable plugin", "id", l.Manifest.ID, "error", err)
		}
	}
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
