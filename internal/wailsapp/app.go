package wailsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/diag"
	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/plugins"
	"github.com/DerekCorniello/dia/internal/registry"
	dia "github.com/DerekCorniello/dia/internal/runtime"
	"github.com/DerekCorniello/dia/internal/state"
	"github.com/fsnotify/fsnotify"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
)

// App is the wails-bound application surface. Methods on App are
// exposed to the Svelte frontend via the generated
// wailsjs/go/wailsapp/App module. main.go binds *App directly;
// routing this through a main-package facade was tried first and
// the generator still followed the methods' return types into
// the wailsapp package.
type App struct {
	ctx context.Context

	mu     sync.Mutex
	store  *state.Store
	rt     *dia.Runtime
	reg    *registry.Registry
	pmgr   *plugins.Manager
	logger *slog.Logger
}

// wailsHost implements plugins.HostAPI by delegating to the wails
// App's bound methods. It bridges the strongly-typed wails surface
// to the loosely-typed interface plugins expect.
type wailsHost struct {
	app *App
}

func (h *wailsHost) ListWorkspaces(ctx context.Context) ([]any, error) {
	infos, err := h.app.ListWorkspaces()
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(infos))
	for _, w := range infos {
		b, _ := json.Marshal(w)
		var m any
		_ = json.Unmarshal(b, &m)
		out = append(out, m)
	}
	return out, nil
}

func (h *wailsHost) GetWorkspace(ctx context.Context, name string) (any, error) {
	d, err := h.app.GetWorkspace(name)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(d)
	var m any
	_ = json.Unmarshal(b, &m)
	return m, nil
}

func (h *wailsHost) StartWorkspace(ctx context.Context, name string) (any, error) {
	if err := h.app.StartWorkspace(name); err != nil {
		return nil, err
	}
	// Return workspace info so the plugin gets a useful result.
	ws, _, err := h.app.findWorkspace(name)
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(ws)
	var m any
	_ = json.Unmarshal(b, &m)
	return m, nil
}

func (h *wailsHost) ListInstances(ctx context.Context) ([]any, error) {
	insts := h.app.ListInstances()
	out := make([]any, 0, len(insts))
	for _, i := range insts {
		b, _ := json.Marshal(i)
		var m any
		_ = json.Unmarshal(b, &m)
		out = append(out, m)
	}
	return out, nil
}

func (h *wailsHost) StopInstance(ctx context.Context, id string) error {
	return h.app.StopInstance(id)
}

func (h *wailsHost) StopAll(ctx context.Context) (int, error) {
	return h.app.StopAll()
}

func (h *wailsHost) Doctor(ctx context.Context) ([]any, error) {
	checks := h.app.Doctor()
	out := make([]any, 0, len(checks))
	for _, c := range checks {
		out = append(out, c)
	}
	return out, nil
}

func (h *wailsHost) Paths(ctx context.Context) (any, error) {
	return h.app.Paths(), nil
}

func (h *wailsHost) GetTheme(ctx context.Context) (string, error) {
	return h.app.GetTheme(), nil
}

func (h *wailsHost) SetTheme(ctx context.Context, name string) error {
	return h.app.SetTheme(name)
}

func (h *wailsHost) ListCustomThemes(ctx context.Context) ([]any, error) {
	themes := h.app.ListCustomThemes()
	out := make([]any, 0, len(themes))
	for _, t := range themes {
		out = append(out, t)
	}
	return out, nil
}

func (h *wailsHost) SetCustomTheme(ctx context.Context, info any) error {
	b, err := json.Marshal(info)
	if err != nil {
		return err
	}
	var ci CustomThemeInfo
	if err := json.Unmarshal(b, &ci); err != nil {
		return err
	}
	return h.app.SetCustomTheme(ci)
}

func (h *wailsHost) DeleteCustomTheme(ctx context.Context, name string) error {
	return h.app.DeleteCustomTheme(name)
}

func (h *wailsHost) NewWorkspace(ctx context.Context, name string) (string, error) {
	return h.app.NewWorkspace(name, false)
}

// New returns an App with no context set; Startup fills it in and
// constructs the runtime.
func New() *App {
	return &App{}
}

// Startup is called by the wails runtime after the window is created.
// The context is required for any runtime calls (events, dialogs).
// Startup also reconciles stale state from a prior crash.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	dir, err := state.ResolveStateDir()
	if err != nil {
		a.logger.Error("resolve state dir", "error", err)
		return
	}
	st, err := state.OpenAt(filepath.Join(dir, state.StateFile))
	if err != nil {
		a.logger.Error("open state", "error", err)
		return
	}
	a.store = st
	a.reg = registry.New()
	a.rt = dia.New(dia.Options{
		Platform: platform.New(),
		Store:    st,
		Registry: a.reg,
		Logger:   a.logger,
	})
	if err := a.rt.Reconcile(); err != nil {
		a.logger.Warn("reconcile on startup", "error", err)
	}

	pmgr, err := plugins.NewManager(plugins.GlobalPluginsDir(dir), &wailsHost{app: a})
	if err != nil {
		a.logger.Error("init plugin manager", "error", err)
	} else {
		if cwd, _ := os.Getwd(); cwd != "" {
			pmgr.SetLocalDir(cwd)
		}
		if err := pmgr.Discover(); err != nil {
			a.logger.Warn("discover plugins", "error", err)
		}
		a.applyPersistedPluginState(pmgr)
		a.pmgr = pmgr
	}
}

func (a *App) StartStateWatcher() {
	if a.store == nil || a.ctx == nil {
		return
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		a.logger.Warn("create fsnotify watcher", "error", err)
		return
	}
	if err := w.Add(a.store.Path()); err != nil {
		w.Close()
		a.logger.Warn("watch state file", "error", err)
		return
	}
	go func() {
		defer w.Close()
		var debounce *time.Timer
		for {
			select {
			case <-w.Events:
				if debounce != nil {
					debounce.Stop()
				}
				debounce = time.AfterFunc(200*time.Millisecond, func() {
					wailsRuntime.EventsEmit(a.ctx, "workspace:state-changed")
				})
			case err, ok := <-w.Errors:
				if !ok {
					return
				}
				a.logger.Warn("fsnotify error", "error", err)
			}
		}
	}()
}

// ListWorkspaces returns the discovered workspaces with a running
// flag attached. Errors during discovery are returned to the UI.
func (a *App) ListWorkspaces() ([]WorkspaceInfo, error) {
	cwd, _ := os.Getwd()
	if pd := a.GetProjectDir(); pd != "" {
		cwd = pd
	}
	sources, err := config.Discover(config.DiscoverOptions{
		GlobalDir: config.DefaultGlobalDir(),
		CWD:       cwd,
	})
	if err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}
	running := a.runningWorkspaces()
	useCount := a.useCountByName()
	out := make([]WorkspaceInfo, 0, len(sources))
	for _, s := range sources {
		info := WorkspaceInfo{
			Name:        s.Workspace.Name,
			Description: s.Workspace.Description,
			Apps:        len(s.Workspace.Apps),
			Source:      sourceLabel(s),
			Path:        s.Path,
			Running:     running[s.Workspace.Name],
			UseCount:    useCount[s.Workspace.Name],
		}
		if len(s.Workspace.Plugins) > 0 {
			ids := make([]string, 0, len(s.Workspace.Plugins))
			for _, ref := range s.Workspace.Plugins {
				ids = append(ids, ref.ID)
			}
			info.Plugins = ids
		}
		out = append(out, info)
	}
	return out, nil
}

// useCountByName returns a map of workspace name -> usage count for
// every entry in state.Recent. Names with no recorded usage are
// absent from the map; callers should treat that as zero.
func (a *App) useCountByName() map[string]int {
	snap := a.store.Snapshot()
	out := make(map[string]int, len(snap.Recent))
	for _, e := range snap.Recent {
		out[e.Name] = e.Count
	}
	return out
}

func sourceLabel(s config.Source) string {
	if s.Local {
		return "local"
	}
	return "global"
}

func pluginIDs(refs []config.PluginRef) []string {
	if len(refs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		ids = append(ids, ref.ID)
	}
	return ids
}

// runningWorkspaces returns the set of workspace names that have at
// least one running instance.
func (a *App) runningWorkspaces() map[string]bool {
	out := map[string]bool{}
	for _, inst := range a.rt.Instances() {
		if inst.Status == state.StatusRunning {
			out[inst.WorkspaceName] = true
		}
	}
	return out
}

// GetWorkspace returns the full detail of one workspace, including
// the list of apps.
func (a *App) GetWorkspace(name string) (*WorkspaceDetail, error) {
	sources, err := config.Discover(config.DiscoverOptions{
		GlobalDir: config.DefaultGlobalDir(),
	})
	if err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}
	running := a.runningWorkspaces()
	for _, s := range sources {
		if s.Workspace.Name != name {
			continue
		}
		apps := make([]AppInfo, 0, len(s.Workspace.Apps))
		for _, app := range s.Workspace.Apps {
			apps = append(apps, AppInfo{
				Type: app.Type,
				Cmd:  app.Cmd,
				Args: strings.Join(app.Args, " "),
				URL:  app.Url,
			})
		}
		return &WorkspaceDetail{
			WorkspaceInfo: WorkspaceInfo{
				Name:    s.Workspace.Name,
				Apps:    len(s.Workspace.Apps),
				Source:  sourceLabel(s),
				Path:    s.Path,
				Running: running[s.Workspace.Name],
				Plugins: pluginIDs(s.Workspace.Plugins),
			},
			AppDetails: apps,
		}, nil
	}
	return nil, fmt.Errorf("workspace %q not found", name)
}

// StartWorkspace launches the named workspace. Per-app failures are
// recorded on the instance; only an all-apps-failed workspace returns
// an error. Workspace plugins are enabled before apps launch. For
// window-type plugins, a dia --plugin-window process is spawned and
// tracked alongside the apps so it is killed on stop.
func (a *App) StartWorkspace(name string) error {
	ws, src, err := a.findWorkspace(name)
	if err != nil {
		return err
	}
	if a.pmgr != nil {
		for _, ref := range ws.Plugins {
			if err := a.enableWorkspacePlugin(ref.ID, ref.Config); err != nil {
				a.logger.Warn("enable workspace plugin", "id", ref.ID, "error", err)
			}
		}
	}
	inst, err := a.rt.Start(ws, src)
	if err != nil {
		return err
	}
	if len(ws.Plugins) > 0 {
		ids := make([]string, 0, len(ws.Plugins))
		for _, ref := range ws.Plugins {
			ids = append(ids, ref.ID)
		}
		_ = a.store.Mutate(func(d *state.Data) {
			i := d.Instances[inst.ID]
			i.Plugins = ids
			d.Instances[inst.ID] = i
		})
	}
	// Spawn plugin windows for window-type plugins.
	if a.pmgr != nil {
		for _, ref := range ws.Plugins {
			if loaded, ok := a.pmgr.Loaded(ref.ID); ok && loaded.Manifest != nil && loaded.Manifest.UI.Type == "window" {
				pid, err := a.spawnPluginWindow(ref.ID, name)
				if err != nil {
					a.logger.Warn("spawn plugin window", "id", ref.ID, "error", err)
					continue
				}
				_ = a.store.Mutate(func(d *state.Data) {
					i := d.Instances[inst.ID]
					i.PluginPIDs = append(i.PluginPIDs, pid)
					d.Instances[inst.ID] = i
				})
			}
		}
	}
	return nil
}

// StopWorkspace finds the running instance for name and stops it.
func (a *App) StopWorkspace(name string) error {
	if a.rt == nil {
		return errors.New("runtime not initialized")
	}
	for _, inst := range a.rt.Instances() {
		if inst.WorkspaceName == name && inst.Status == state.StatusRunning {
			return a.StopInstance(inst.ID)
		}
	}
	return fmt.Errorf("workspace %q is not running", name)
}

// spawnPluginWindow spawns a dia --plugin-window process for the
// given plugin and workspace. Returns the PID of the spawned process.
func (a *App) spawnPluginWindow(pluginID, workspaceName string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("get executable: %w", err)
	}
	args := []string{"--plugin-window=" + pluginID, "--workspace=" + workspaceName}
	pid, err := a.launchProcess(exe, args)
	if err != nil {
		return 0, fmt.Errorf("launch plugin window: %w", err)
	}
	return pid, nil
}

// launchProcess starts a detached process and returns its PID.
func (a *App) launchProcess(cmd string, args []string) (int, error) {
	handle, err := platform.New().Launch(platform.LaunchOpts{
		Cmd:  cmd,
		Args: args,
	})
	if err != nil {
		return 0, err
	}
	return handle.PID(), nil
}

func (a *App) enableWorkspacePlugin(id string, cfg map[string]any) error {
	loaded, ok := a.pmgr.Loaded(id)
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	granted := plugins.GrantCapabilities(loaded.Manifest.Capabilities, plugins.DefaultReadCapabilities())
	if err := a.pmgr.SetConfig(id, cfg); err != nil {
		a.logger.Warn("set plugin config", "id", id, "error", err)
	}
	if err := a.pmgr.EnableWithGrants(id, granted); err != nil {
		return err
	}
	_ = a.store.Mutate(func(d *state.Data) {
		if d.Plugins == nil {
			d.Plugins = map[string]state.PluginState{}
		}
		d.Plugins[id] = state.PluginState{Enabled: true, GrantedCapabilities: granted, Config: cfg}
	})
	return nil
}

// StopInstance terminates one running instance by ID and disables
// any workspace plugins that were enabled when it started.
func (a *App) StopInstance(id string) error {
	if a.rt == nil {
		return errors.New("runtime not initialized")
	}
	var inst state.Instance
	if a.store != nil {
		snap := a.store.Snapshot()
		inst = snap.Instances[id]
	}
	if a.pmgr != nil {
		for _, pid := range inst.Plugins {
			if err := a.pmgr.Disable(pid); err != nil {
				a.logger.Warn("disable workspace plugin", "id", pid, "error", err)
			}
			_ = a.store.Mutate(func(d *state.Data) {
				if d.Plugins == nil {
					d.Plugins = map[string]state.PluginState{}
				}
				ps := d.Plugins[pid]
				ps.Enabled = false
				d.Plugins[pid] = ps
			})
		}
	}
	// Kill plugin window processes.
	pf := platform.New()
	for _, ppid := range inst.PluginPIDs {
		if ppid > 0 {
			if err := pf.Kill(ppid, true); err != nil {
				a.logger.Warn("kill plugin window", "pid", ppid, "error", err)
			}
		}
	}
	if err := a.rt.Stop(id, false); err != nil {
		return err
	}
	return nil
}

// StopAll terminates every running instance. Returns the number
// stopped.
func (a *App) StopAll() (int, error) {
	if a.rt == nil {
		return 0, errors.New("runtime not initialized")
	}
	running := 0
	for _, inst := range a.rt.Instances() {
		if inst.Status == state.StatusRunning {
			running++
		}
	}
	for _, inst := range a.rt.Instances() {
		if inst.Status != state.StatusRunning {
			continue
		}
		_ = a.StopInstance(inst.ID)
	}
	return running, nil
}

// ListInstances returns the current set of tracked instances, most
// recently started first.
func (a *App) ListInstances() []InstanceInfo {
	if a.rt == nil {
		return nil
	}
	insts := a.rt.Instances()
	out := make([]InstanceInfo, 0, len(insts))
	for _, inst := range insts {
		out = append(out, *toInstanceInfo(&inst))
	}
	return out
}

// Reconcile walks the state and drops dead PIDs. Returns a summary.
func (a *App) Reconcile() (ReconcileInfo, error) {
	if a.rt == nil {
		return ReconcileInfo{}, errors.New("runtime not initialized")
	}
	before := len(a.rt.Instances())
	if err := a.rt.Reconcile(); err != nil {
		return ReconcileInfo{}, err
	}
	after := len(a.rt.Instances())
	return ReconcileInfo{
		Reconciled: before - after,
		Remaining:  after,
		Total:      after,
	}, nil
}

// Doctor runs smoke checks and returns one row per check.
func (a *App) Doctor() []CheckInfo {
	stateDir, stateFile := "", ""
	if a.store != nil {
		stateFile = a.store.Path()
		stateDir = filepath.Dir(stateFile)
	}
	rows := diag.RunChecks(stateDir, stateFile)
	out := make([]CheckInfo, 0, len(rows))
	for _, r := range rows {
		out = append(out, CheckInfo{Name: r.Name, Status: r.Status, Detail: r.Detail})
	}
	return out
}

// Paths returns the on-disk locations dia uses, for the UI to
// show in a settings panel.
func (a *App) Paths() PathsInfo {
	out := PathsInfo{GlobalConfigDir: config.DefaultGlobalDir()}
	if dir, err := state.ResolveStateDir(); err == nil {
		out.StateDir = dir
		if a.store != nil {
			out.StateFile = a.store.Path()
		}
	}
	return out
}

// GetTheme returns the current UI theme name from persisted state.
func (a *App) GetTheme() string {
	if a.store == nil {
		return state.DefaultTheme
	}
	t := a.store.Snapshot().Theme
	if t == "" {
		return state.DefaultTheme
	}
	return t
}

// SetTheme persists the UI theme name so it survives restart.
func (a *App) SetTheme(theme string) error {
	if a.store == nil {
		return errors.New("state store not initialized")
	}
	return a.store.Mutate(func(d *state.Data) {
		d.Theme = theme
	})
}

// ListCustomThemes returns every user-defined theme. The frontend
// merges this list with the built-in daisyUI themes at render time.
func (a *App) ListCustomThemes() []CustomThemeInfo {
	if a.store == nil {
		return nil
	}
	d := a.store.Snapshot()
	out := make([]CustomThemeInfo, 0, len(d.CustomThemes))
	for name, t := range d.CustomThemes {
		colors := make(map[string]string, len(t.Colors))
		for k, v := range t.Colors {
			colors[k] = v
		}
		out = append(out, CustomThemeInfo{
			Name:        name,
			ColorScheme: t.ColorScheme,
			Colors:      colors,
		})
	}
	return out
}

// SetCustomTheme upserts a user-defined theme by name. An empty
// color_scheme or zero colors returns an error; the name must be
// non-empty and contain only ASCII letters, digits, hyphen, and
// underscore.
func (a *App) SetCustomTheme(info CustomThemeInfo) error {
	if a.store == nil {
		return errors.New("state store not initialized")
	}
	if err := validateThemeName(info.Name); err != nil {
		return err
	}
	if info.ColorScheme != "light" && info.ColorScheme != "dark" {
		return fmt.Errorf("color_scheme must be \"light\" or \"dark\", got %q", info.ColorScheme)
	}
	if len(info.Colors) == 0 {
		return errors.New("colors must not be empty")
	}
	for k, v := range info.Colors {
		if !isValidColorKey(k) {
			return fmt.Errorf("unknown color slot %q", k)
		}
		if !isValidHex(v) {
			return fmt.Errorf("color %s = %q is not a valid #rrggbb hex string", k, v)
		}
	}
	return a.store.Mutate(func(d *state.Data) {
		colors := make(map[string]string, len(info.Colors))
		for k, v := range info.Colors {
			colors[k] = v
		}
		d.CustomThemes[info.Name] = state.CustomTheme{
			ColorScheme: info.ColorScheme,
			Colors:      colors,
		}
	})
}

// DeleteCustomTheme removes a user-defined theme. Built-in themes
// are not stored in state and cannot be deleted; this is a no-op
// for names that are not in the custom map.
func (a *App) DeleteCustomTheme(name string) error {
	if a.store == nil {
		return errors.New("state store not initialized")
	}
	if err := validateThemeName(name); err != nil {
		return err
	}
	return a.store.Mutate(func(d *state.Data) {
		delete(d.CustomThemes, name)
	})
}

func validateThemeName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) > 64 {
		return errors.New("name must be 64 characters or fewer")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return fmt.Errorf("name %q must match [A-Za-z0-9_-]+", name)
		}
	}
	return nil
}

// isValidColorKey checks that a color slot is one of the daisyUI
// v4 semantic colors we surface in the editor.
func isValidColorKey(k string) bool {
	switch k {
	case "primary", "primary_content",
		"secondary", "secondary_content",
		"accent", "accent_content",
		"neutral", "neutral_content",
		"base_100", "base_200", "base_300", "base_content",
		"info", "success", "warning", "error":
		return true
	}
	return false
}

func isValidHex(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// OpenConfigFolder reveals the global config dir in the file
// manager. No-op if the dir does not exist.
func (a *App) OpenConfigFolder() error {
	dir := config.DefaultGlobalDir()
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

// OpenStateFolder reveals the state dir in the file manager.
func (a *App) OpenStateFolder() error {
	dir, err := state.ResolveStateDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(dir); err != nil {
		return err
	}
	return platform.New().RevealInFileManager(dir)
}

// OpenStateFile opens the state JSON file with the default editor.
func (a *App) OpenStateFile() error {
	path, err := state.FilePath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return platform.New().OpenFile(path)
}

// SelectProjectDir opens a directory picker dialog and persists the
// chosen path. Discovery then includes that directory for .dia.yaml
// and .dia/ workspace lookups.
func (a *App) SelectProjectDir() (string, error) {
	if a.ctx == nil {
		return "", errors.New("not initialized")
	}
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Project Directory",
	})
	if err != nil {
		return "", fmt.Errorf("select directory: %w", err)
	}
	if dir == "" {
		return "", nil
	}
	if a.store != nil {
		_ = a.store.Mutate(func(d *state.Data) {
			d.ProjectDir = dir
		})
	}
	return dir, nil
}

// GetProjectDir returns the persisted project directory.
func (a *App) GetProjectDir() string {
	if a.store == nil {
		return ""
	}
	return a.store.Snapshot().ProjectDir
}

// ClearProjectDir removes the persisted project directory.
func (a *App) ClearProjectDir() error {
	if a.store == nil {
		return errors.New("state store not initialized")
	}
	return a.store.Mutate(func(d *state.Data) {
		d.ProjectDir = ""
	})
}

// NewWorkspace writes a starter YAML. When local is true the file
// is placed in CWD/.dia/; otherwise it goes in the global config
// dir. The caller supplies the name; if a file with that name
// already exists, the operation is refused.
func (a *App) NewWorkspace(name string, local bool) (string, error) {
	if name == "" {
		return "", errors.New("name is required")
	}
	if err := config.ValidateName(name); err != nil {
		return "", err
	}
	var dir string
	if local {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("get cwd: %w", err)
		}
		dir = filepath.Join(cwd, config.LocalDirName)
	} else {
		dir = config.DefaultGlobalDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+".yaml")
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("workspace %q already exists at %s", name, path)
	}
	body := fmt.Sprintf("version: %d\nname: %s\n", config.SchemaVersion, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (a *App) findWorkspace(name string) (*config.Workspace, config.Source, error) {
	sources, err := config.Discover(config.DiscoverOptions{
		GlobalDir: config.DefaultGlobalDir(),
	})
	if err != nil {
		return nil, config.Source{}, fmt.Errorf("discover: %w", err)
	}
	for _, s := range sources {
		if s.Workspace.Name == name {
			return s.Workspace, s, nil
		}
	}
	return nil, config.Source{}, fmt.Errorf("workspace %q not found", name)
}

func toInstanceInfo(inst *state.Instance) *InstanceInfo {
	apps := make([]ProcessInfo, 0, len(inst.Apps))
	for _, a := range inst.Apps {
		apps = append(apps, ProcessInfo{
			Type:   a.Type,
			Cmd:    a.Cmd,
			PID:    a.PID,
			Status: string(a.Status),
			Err:    a.Err,
		})
	}
	out := &InstanceInfo{
		ID:            inst.ID,
		Workspace:     inst.WorkspaceName,
		WorkspacePath: inst.WorkspacePath,
		StartedAt:     inst.StartedAt.UTC().Format(time.RFC3339),
		Status:        string(inst.Status),
		Apps:          apps,
	}
	if len(inst.Plugins) > 0 {
		out.Plugins = inst.Plugins
	}
	return out
}

// GetWorkspaceEditor returns the editable view of a workspace.
func (a *App) GetWorkspaceEditor(name string) (*WorkspaceEditor, error) {
	ws, _, err := a.findWorkspace(name)
	if err != nil {
		return nil, err
	}
	cwd, _ := os.Getwd()
	editor := &WorkspaceEditor{
		OriginalName: ws.Name,
		Name:         ws.Name,
		Description:  ws.Description,
		DefaultCwd:   cwd,
		Apps:         make([]AppEditor, 0, len(ws.Apps)),
		Plugins:      make([]PluginRefEditor, 0, len(ws.Plugins)),
	}
	for _, app := range ws.Apps {
		ae := AppEditor{
			Label: app.Label,
			Cmd:   app.Cmd,
			Cwd:   app.Cwd,
			Url:   app.Url,
		}
		if len(app.Args) > 0 {
			ae.TermCmd = strings.Join(app.Args, " ")
		}
		editor.Apps = append(editor.Apps, ae)
	}
	for _, ref := range ws.Plugins {
		editor.Plugins = append(editor.Plugins, PluginRefEditor{
			ID:     ref.ID,
			Config: ref.Config,
		})
	}
	return editor, nil
}

// SaveWorkspaceEditor validates and persists the workspace from the
// form editor. If the name changed (OriginalName != Name), the old
// file is removed. App type is inferred from the fields:
//   - Cmd set → type=local
//   - Only Url set → type=open
//   - Both empty → validation error
func (a *App) SaveWorkspaceEditor(editor WorkspaceEditor) error {
	if editor.Name == "" {
		return errors.New("name is required")
	}
	if err := config.ValidateName(editor.Name); err != nil {
		return err
	}
	if len(editor.Apps) == 0 {
		return errors.New("at least one app is required")
	}
	ws := &config.Workspace{
		Version:     config.SchemaVersion,
		Name:        editor.Name,
		Description: editor.Description,
		Apps:        make([]config.App, 0, len(editor.Apps)),
	}
	for _, ae := range editor.Apps {
		appType := "local"
		if ae.Url != "" && ae.Cmd == "" {
			appType = "open"
		} else if ae.Cmd == "" {
			return fmt.Errorf("app %q: command or url is required", ae.Label)
		}
		app := config.App{
			Type:  appType,
			Label: ae.Label,
			Cmd:   ae.Cmd,
			Cwd:   ae.Cwd,
			Url:   ae.Url,
		}
		if ae.TermCmd != "" {
			app.Args = []string{"-e", ae.TermCmd}
		}
		ws.Apps = append(ws.Apps, app)
	}
	for _, ref := range editor.Plugins {
		ws.Plugins = append(ws.Plugins, config.PluginRef{
			ID:     ref.ID,
			Config: ref.Config,
		})
	}
	out, err := yaml.Marshal(ws)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	dir := config.DefaultGlobalDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, editor.Name+".yaml")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return err
	}
	if editor.OriginalName != "" && editor.OriginalName != editor.Name {
		oldPath := filepath.Join(dir, editor.OriginalName+".yaml")
		if err := os.Remove(oldPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove old: %w", err)
		}
	}
	return nil
}

// DeleteWorkspace removes the workspace YAML file. Refuses if the
// workspace has a running instance.
func (a *App) DeleteWorkspace(name string) error {
	if a.rt != nil {
		for _, inst := range a.rt.Instances() {
			if inst.WorkspaceName == name && inst.Status == state.StatusRunning {
				return fmt.Errorf("workspace %q is running; stop it first", name)
			}
		}
	}
	_, src, err := a.findWorkspace(name)
	if err != nil {
		return err
	}
	if err := os.Remove(src.Path); err != nil {
		return fmt.Errorf("delete %q: %w", name, err)
	}
	return nil
}

// ListPlugins returns every discovered plugin, with the host view
// (manifest summary, status, granted capabilities). Includes both
// enabled and disabled plugins.
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

// EnablePlugin loads and starts a plugin's goja runtime, then calls
// its onEnable hook if defined.
func (a *App) EnablePlugin(id string) error {
	if a.pmgr == nil {
		return errors.New("plugin manager not initialized")
	}
	return a.pmgr.Enable(id)
}

// DisablePlugin shuts down a plugin's goja runtime.
func (a *App) DisablePlugin(id string) error {
	if a.pmgr == nil {
		return errors.New("plugin manager not initialized")
	}
	return a.pmgr.Disable(id)
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

// RevealPath opens an arbitrary path in the file manager.
func (a *App) RevealPath(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return platform.New().RevealInFileManager(path)
}

// GetRecent returns the recently-started workspaces as
// {name, count} entries, most recent first. Names that have not
// been started since the recent list was last extended are absent.
func (a *App) GetRecent() []state.RecentEntry {
	if a.store == nil {
		return nil
	}
	snap := a.store.Snapshot()
	if len(snap.Recent) == 0 {
		return nil
	}
	out := make([]state.RecentEntry, len(snap.Recent))
	copy(out, snap.Recent)
	return out
}

// GetKeybindings returns the current keybinding overrides.
func (a *App) GetKeybindings() map[string]string {
	if a.store == nil {
		return map[string]string{}
	}
	snap := a.store.Snapshot()
	if snap.Keybindings == nil {
		return map[string]string{}
	}
	return snap.Keybindings
}

// SetKeybinding stores a keybinding override for the given action.
// Pass keys="" to remove the override (restoring the default).
func (a *App) SetKeybinding(action, keys string) error {
	if a.store == nil {
		return errors.New("store not initialized")
	}
	return a.store.Mutate(func(d *state.Data) {
		if d.Keybindings == nil {
			d.Keybindings = map[string]string{}
		}
		if keys == "" {
			delete(d.Keybindings, action)
		} else {
			d.Keybindings[action] = keys
		}
	})
}

// ResetKeybindings removes all keybinding overrides.
func (a *App) ResetKeybindings() error {
	if a.store == nil {
		return errors.New("store not initialized")
	}
	return a.store.Mutate(func(d *state.Data) {
		d.Keybindings = nil
	})
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

// applyPersistedPluginState re-enables plugins that the user enabled
// in a previous session and applies the persisted granted capability
// set (intersected with the manifest's requested set). Plugins that
// are enabled here will start their goja runtime immediately; a
// plugin whose entry script fails is left in the errored state and
// the call site (frontend) will show the LastError.
func (a *App) applyPersistedPluginState(pmgr *plugins.Manager) {
	snap := a.store.Snapshot()
	for id, ps := range snap.Plugins {
		if !ps.Enabled {
			continue
		}
		loaded, ok := pmgr.Loaded(id)
		if !ok {
			continue
		}
		granted := ps.GrantedCapabilities
		if granted == nil {
			granted = plugins.DefaultReadCapabilities()
		}
		granted = plugins.GrantCapabilities(loaded.Manifest.Capabilities, granted)
		if err := pmgr.EnableWithGrants(id, granted); err != nil {
			a.logger.Warn("enable persisted plugin", "id", id, "error", err)
		}
	}
}

// SetPluginEnabled persists the enabled flag and granted caps so the
// next startup re-enables the plugin with the same grant set.
func (a *App) SetPluginEnabled(id string, enabled bool, granted []string) error {
	if a.pmgr == nil {
		return errors.New("plugin manager not initialized")
	}
	if !enabled {
		if err := a.pmgr.Disable(id); err != nil {
			return err
		}
	} else {
		loaded, ok := a.pmgr.Loaded(id)
		if !ok {
			return fmt.Errorf("plugin %q not found", id)
		}
		final := plugins.GrantCapabilities(loaded.Manifest.Capabilities, granted)
		if err := a.pmgr.EnableWithGrants(id, final); err != nil {
			return err
		}
		granted = final
	}
	return a.store.Mutate(func(d *state.Data) {
		if d.Plugins == nil {
			d.Plugins = map[string]state.PluginState{}
		}
		d.Plugins[id] = state.PluginState{Enabled: enabled, GrantedCapabilities: granted}
	})
}

func loadedToInfo(l plugins.Loaded) PluginInfo {
	out := PluginInfo{
		Source:              string(l.Source),
		Dir:                 l.Dir,
		Enabled:             l.Enabled,
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
