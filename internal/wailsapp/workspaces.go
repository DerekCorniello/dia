package wailsapp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/daemon"
	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/plugins"
	"github.com/DerekCorniello/dia/internal/state"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"gopkg.in/yaml.v3"
)

// ListWorkspaces returns all discovered workspaces with a running
// flag attached. Every workspace is returned regardless of CWD:
// global workspaces, workspaces from every persisted root, and the
// project-local workspace in the CWD walk-up. Errors during discovery
// are returned to the UI. The running flag comes from the daemon's
// live instance list, not the state file.
func (a *App) ListWorkspaces() ([]WorkspaceInfo, error) {
	if a.store == nil {
		return nil, errors.New("not initialized")
	}
	cwd, _ := os.Getwd()
	snap := a.store.Snapshot()
	sources, err := config.Discover(config.DiscoverOptions{
		GlobalDir: config.DefaultGlobalDir(),
		Roots:     snap.Roots,
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
	if !s.Local {
		return "global"
	}
	dir := filepath.Dir(s.Path)
	// Show the directory the workspace YAML lives in for root-based workspaces.
	// This is more informative than "local" — it tells you where to find it.
	base := filepath.Base(dir)
	if base == ".dia" {
		base = filepath.Base(filepath.Dir(dir))
	}
	return base
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

// instances returns the daemon's live instance list, or nil when the
// daemon can't be reached. Status reads are best-effort: the UI
// should not fail simply because the daemon is not up yet.
func (a *App) instances() []state.Instance {
	dae, err := a.daemonClient()
	if err != nil {
		return nil
	}
	var insts []state.Instance
	if err := dae.Do(daemon.MethodList, nil, &insts); err != nil {
		a.logger.Warn("list instances", "error", err)
		return nil
	}
	return insts
}

// runningWorkspaces returns the set of workspace names that have at
// least one running instance.
func (a *App) runningWorkspaces() map[string]bool {
	out := map[string]bool{}
	for _, inst := range a.instances() {
		if inst.Status == state.StatusRunning {
			out[inst.WorkspaceName] = true
		}
	}
	return out
}

// GetWorkspace returns the full detail of one workspace, including
// the list of apps.
func (a *App) GetWorkspace(name string) (*WorkspaceDetail, error) {
	if a.store == nil {
		return nil, errors.New("not initialized")
	}
	var roots []string
	snap := a.store.Snapshot()
	roots = snap.Roots
	sources, err := config.Discover(config.DiscoverOptions{
		GlobalDir: config.DefaultGlobalDir(),
		Roots:     roots,
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

// StartWorkspace launches the named workspace on the daemon. The app
// enables its panel plugins first so their panels render in the GUI;
// the daemon does the same for its resolver and spawns window-type
// plugin windows. Starting an already-running name attaches to the
// existing instance (tmux server semantics).
func (a *App) StartWorkspace(name string) error {
	if a.store == nil {
		return errors.New("not initialized")
	}
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
	dae, err := a.daemonClient()
	if err != nil {
		return err
	}
	var reply daemon.StartReply
	if err := dae.Do(daemon.MethodStart, daemon.StartParams{Name: name, Path: src.Path}, &reply); err != nil {
		return err
	}
	// The daemon persists the instance and spawns plugin windows, so
	// the GUI does not; it only needs its panels enabled (done above)
	// and a push so the frontend refreshes from the live list.
	a.notifyStateChanged()
	return nil
}

// RestartWorkspace stops the running instance for name (if any) and
// starts a fresh one, via the daemon.
func (a *App) RestartWorkspace(name string) error {
	if a.store == nil {
		return errors.New("not initialized")
	}
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
	dae, err := a.daemonClient()
	if err != nil {
		return err
	}
	var reply daemon.StartReply
	if err := dae.Do(daemon.MethodRestart, daemon.StartParams{Name: name, Path: src.Path}, &reply); err != nil {
		return err
	}
	a.notifyStateChanged()
	return nil
}

// notifyStateChanged tells the frontend to refresh workspace state
// after the daemon has mutated it. If ctx is unset (a test) the call
// is a no-op.
func (a *App) notifyStateChanged() {
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "workspace:state-changed")
	}
}

// StopWorkspace stops the running instance for name via the daemon.
// Stopping a name with nothing running is a no-op, not an error.
func (a *App) StopWorkspace(name string) error {
	dae, err := a.daemonClient()
	if err != nil {
		return err
	}
	var stopped map[string]any
	if err := dae.Do(daemon.MethodStop, daemon.StopParams{Name: name}, &stopped); err != nil {
		if isNoWorkspace(err) {
			return nil
		}
		return err
	}
	a.notifyStateChanged()
	return nil
}

// isNoWorkspace reports whether a daemon error means the workspace
// does not exist / is not running. The daemon serializes
// NoWorkspaceError across the socket as a plain error string, so we
// match on the message.
func isNoWorkspace(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

// spawnPluginWindow spawns a dia --plugin-window process for the
// given plugin and workspace. Returns the PID of the spawned process.
func (a *App) spawnPluginWindow(pluginID, workspaceName, workspacePath string) (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("get executable: %w", err)
	}
	args := []string{"--plugin-window=" + pluginID, "--workspace=" + workspaceName, "--workspace-path=" + workspacePath}
	a.logger.Debug("spawn plugin window", "pluginID", pluginID, "workspaceName", workspaceName, "workspacePath", workspacePath, "args", args)
	pid, err := a.launchProcess(exe, args)
	if err != nil {
		return 0, fmt.Errorf("launch plugin window: %w", err)
	}
	a.logger.Debug("spawn plugin window: done", "pid", pid)
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

// OpenWorkspacePluginWindow spawns a window plugin bound to a
// workspace: the spawned process gets the workspace name and path so
// it loads the workspace's config for that plugin. Unlike StartWorkspace
// it does not launch the workspace's apps; it only opens the plugin
// window for the workspace it belongs to.
func (a *App) OpenWorkspacePluginWindow(name, pluginID string) (int, error) {
	if a.pmgr == nil {
		return 0, errors.New("plugin manager not initialized")
	}
	ws, src, err := a.findWorkspace(name)
	if err != nil {
		return 0, err
	}
	var refCfg map[string]any
	found := false
	for _, ref := range ws.Plugins {
		if ref.ID == pluginID {
			refCfg = ref.Config
			found = true
			break
		}
	}
	if !found {
		return 0, fmt.Errorf("workspace %q does not include plugin %q", name, pluginID)
	}
	if a.pmgr != nil {
		if err := a.enableWorkspacePlugin(pluginID, refCfg); err != nil {
			a.logger.Warn("enable workspace plugin", "id", pluginID, "error", err)
		}
	}
	return a.spawnPluginWindow(pluginID, name, src.Path)
}

// enableWorkspacePlugin loads a workspace plugin into the GUI's panel
// manager and persists its config and grants so the daemon and the
// next launch pick them up. Capabilities are never granted by being
// enabled; grants are intersected against the manifest from the
// user-approved list.
func (a *App) enableWorkspacePlugin(id string, cfg map[string]any) error {
	loaded, ok := a.pmgr.Loaded(id)
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	if loaded.Manifest == nil {
		return fmt.Errorf("plugin %q has no valid manifest: %s", id, loaded.LastError)
	}
	granted := workspacePluginGrants(a.store, loaded.Manifest, id)
	if err := a.pmgr.SetConfig(id, cfg); err != nil {
		a.logger.Warn("set plugin config", "id", id, "error", err)
	}
	if err := a.pmgr.EnableWithGrants(id, granted); err != nil {
		return err
	}
	if err := a.store.Mutate(func(d *state.Data) {
		if d.Plugins == nil {
			d.Plugins = map[string]state.PluginState{}
		}
		prev := d.Plugins[id]
		prev.GrantedCapabilities = granted
		prev.Config = cfg
		d.Plugins[id] = prev
	}); err != nil {
		a.logger.Warn("mutate enable plugin state", "error", err)
	}
	return nil
}

// workspacePluginGrants derives the granted capability set for a
// plugin from the persisted state (the user-approved list) rather
// than always starting from the read-only defaults. Intersect with
// the manifest so capabilities the manifest no longer requests are
// dropped; the store is only read and may be nil.
func workspacePluginGrants(st *state.Store, m *plugins.Manifest, id string) []string {
	granted := plugins.DefaultReadCapabilities()
	if st != nil {
		if ps, ok := st.Snapshot().Plugins[id]; ok && ps.GrantedCapabilities != nil {
			granted = ps.GrantedCapabilities
		}
	}
	return plugins.GrantCapabilities(m.Capabilities, granted)
}

// StopInstance terminates one running instance by ID. The daemon stops
// workspaces by name, so the ID is translated through its list first.
func (a *App) StopInstance(id string) error {
	dae, err := a.daemonClient()
	if err != nil {
		return err
	}
	var insts []state.Instance
	if err := dae.Do(daemon.MethodList, nil, &insts); err != nil {
		return err
	}
	name := ""
	for _, inst := range insts {
		if inst.ID == id {
			name = inst.WorkspaceName
			break
		}
	}
	if name == "" {
		return fmt.Errorf("instance %s not found", id)
	}
	var result map[string]any
	if err := dae.Do(daemon.MethodStop, daemon.StopParams{Name: name}, &result); err != nil {
		return err
	}
	a.notifyStateChanged()
	return nil
}

// StopAll terminates every running instance via the daemon. Returns
// the number stopped.
func (a *App) StopAll() (int, error) {
	dae, err := a.daemonClient()
	if err != nil {
		return 0, err
	}
	var result map[string][]string
	if err := dae.Do(daemon.MethodStopAll, nil, &result); err != nil {
		return 0, err
	}
	n := len(result["stopped"])
	if len(result["stopped"]) > 0 {
		a.notifyStateChanged()
	}
	return n, nil
}

// ListInstances returns the current set of tracked instances from the
// daemon, most recently started first.
func (a *App) ListInstances() []InstanceInfo {
	insts := a.instances()
	if insts == nil {
		return nil
	}
	out := make([]InstanceInfo, 0, len(insts))
	for i := range insts {
		inst := insts[i]
		out = append(out, *toInstanceInfo(&inst))
	}
	return out
}

// Reconcile walks the daemon state and drops dead PIDs. Returns a
// summary.
func (a *App) Reconcile() (ReconcileInfo, error) {
	dae, err := a.daemonClient()
	if err != nil {
		return ReconcileInfo{}, err
	}
	var summary struct {
		Reconciled int `json:"reconciled"`
		Remaining  int `json:"remaining"`
		Total      int `json:"total"`
	}
	if err := dae.Do(daemon.MethodReconcile, nil, &summary); err != nil {
		return ReconcileInfo{}, err
	}
	a.notifyStateChanged()
	return ReconcileInfo{
		Reconciled: summary.Reconciled,
		Remaining:  summary.Remaining,
		Total:      summary.Total,
	}, nil
}

func (a *App) findWorkspace(name string) (*config.Workspace, config.Source, error) {
	cwd, _ := os.Getwd()
	var roots []string
	if a.store != nil {
		roots = a.store.Snapshot().Roots
	}
	sources, err := config.Discover(config.DiscoverOptions{
		GlobalDir: config.DefaultGlobalDir(),
		Roots:     roots,
		CWD:       cwd,
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
	ws, src, err := a.findWorkspace(name)
	if err != nil {
		return nil, err
	}
	a.logger.Debug("get workspace editor", "name", name, "srcPath", src.Path, "srcLocal", src.Local)
	cwd, _ := os.Getwd()
	editor := &WorkspaceEditor{
		OriginalName: ws.Name,
		OriginalPath: src.Path,
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
	a.logger.Debug("save workspace editor", "originalPath", editor.OriginalPath, "originalName", editor.OriginalName, "name", editor.Name)
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
	a.logger.Debug("save workspace editor: marshaled", "yaml", string(out), "plugins", ws.Plugins)
	// Write to the same directory as the original file so that local
	// workspaces stay local and global workspaces stay global.
	dir := config.DefaultGlobalDir()
	if editor.OriginalPath != "" {
		dir = filepath.Dir(editor.OriginalPath)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	wsPath := filepath.Join(dir, editor.Name+".yaml")
	a.logger.Debug("save workspace editor: writing", "path", wsPath, "dir", dir)
	if err := os.WriteFile(wsPath, out, 0o644); err != nil {
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
	if running := a.runningWorkspaces(); running[name] {
		return fmt.Errorf("workspace %q is running; stop it first", name)
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

// NewWorkspace writes a starter YAML. When dir is empty the file goes
// in the global config dir (~/.config/dia/workspaces/). When dir is
// set, the file goes in {dir}/.dia/{name}.yaml and dir is
// auto-registered as a root so the workspace appears in every list
// regardless of CWD.
func (a *App) NewWorkspace(name string, dir string) (string, error) {
	if name == "" {
		return "", errors.New("name is required")
	}
	if err := config.ValidateName(name); err != nil {
		return "", err
	}
	var wsDir string
	var rootDir string
	if dir == "" {
		wsDir = config.DefaultGlobalDir()
	} else {
		wsDir = filepath.Join(dir, config.LocalDirName)
		rootDir = dir
	}
	if err := os.MkdirAll(wsDir, 0o755); err != nil {
		return "", err
	}
	yamlPath := filepath.Join(wsDir, name+".yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return yamlPath, fmt.Errorf("workspace %q already exists at %s", name, yamlPath)
	}
	body := fmt.Sprintf("version: %d\nname: %s\n", config.SchemaVersion, name)
	if err := os.WriteFile(yamlPath, []byte(body), 0o644); err != nil {
		return "", err
	}
	// Auto-register the parent directory as a root so the workspace
	// is visible from anywhere.
	if rootDir != "" && a.store != nil {
		_ = a.store.Mutate(func(d *state.Data) {
			for _, r := range d.Roots {
				if r == rootDir {
					return
				}
			}
			d.Roots = append(d.Roots, rootDir)
		})
	}
	return yamlPath, nil
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
