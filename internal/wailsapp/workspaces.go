package wailsapp

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/plugins"
	"github.com/DerekCorniello/dia/internal/state"
	"gopkg.in/yaml.v3"
)

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
		if err := a.store.Mutate(func(d *state.Data) {
			i := d.Instances[inst.ID]
			i.Plugins = ids
			d.Instances[inst.ID] = i
		}); err != nil {
			a.logger.Warn("mutate instance plugins", "error", err)
		}
	}
	// Spawn plugin windows for window-type plugins.
	if a.pmgr != nil {
		for _, ref := range ws.Plugins {
			if loaded, ok := a.pmgr.Loaded(ref.ID); ok && loaded.Manifest != nil && loaded.Manifest.UI.Type == "window" {
				pid, err := a.spawnPluginWindow(ref.ID, name, src.Path)
				if err != nil {
					a.logger.Warn("spawn plugin window", "id", ref.ID, "error", err)
					continue
				}
				if err := a.store.Mutate(func(d *state.Data) {
					i := d.Instances[inst.ID]
					i.PluginPIDs = append(i.PluginPIDs, pid)
					d.Instances[inst.ID] = i
				}); err != nil {
					a.logger.Warn("mutate instance plugin PIDs", "error", err)
				}
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
	return nil
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
	if err := a.store.Mutate(func(d *state.Data) {
		if d.Plugins == nil {
			d.Plugins = map[string]state.PluginState{}
		}
		d.Plugins[id] = state.PluginState{Enabled: true, GrantedCapabilities: granted, Config: cfg}
	}); err != nil {
		a.logger.Warn("mutate enable plugin state", "error", err)
	}
	return nil
}

// StopInstance terminates one running instance by ID.
func (a *App) StopInstance(id string) error {
	if a.rt == nil {
		return errors.New("runtime not initialized")
	}
	var inst state.Instance
	if a.store != nil {
		snap := a.store.Snapshot()
		inst = snap.Instances[id]
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
		if err := a.StopInstance(inst.ID); err != nil {
			a.logger.Warn("stop instance during StopAll", "id", inst.ID, "error", err)
		}
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

func (a *App) findWorkspace(name string) (*config.Workspace, config.Source, error) {
	cwd, _ := os.Getwd()
	if pd := a.GetProjectDir(); pd != "" {
		cwd = pd
	}
	sources, err := config.Discover(config.DiscoverOptions{
		GlobalDir: config.DefaultGlobalDir(),
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
	yamlPath := filepath.Join(dir, name+".yaml")
	if _, err := os.Stat(yamlPath); err == nil {
		return yamlPath, fmt.Errorf("workspace %q already exists at %s", name, yamlPath)
	}
	body := fmt.Sprintf("version: %d\nname: %s\n", config.SchemaVersion, name)
	if err := os.WriteFile(yamlPath, []byte(body), 0o644); err != nil {
		return "", err
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
