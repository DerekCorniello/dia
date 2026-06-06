package wailsapp

import (
	"context"
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
	"github.com/DerekCorniello/dia/internal/registry"
	dia "github.com/DerekCorniello/dia/internal/runtime"
	"github.com/DerekCorniello/dia/internal/state"
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
	plug   *registry.PluginResolver
	logger *slog.Logger
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
	a.plug = registry.NewPluginResolver()
	a.rt = dia.New(dia.Options{
		Platform: platform.New(),
		Store:    st,
		Registry: a.reg,
		Plugins:  a.plug,
		Logger:   a.logger,
	})
	if err := a.rt.Reconcile(); err != nil {
		a.logger.Warn("reconcile on startup", "error", err)
	}

	// TODO(cross-process-state): wire a fsnotify watcher on
	// a.store.Path() and re-Snapshot the runtime when the file
	// changes outside this process (e.g. `dia start foo` from a
	// shell). Deferred to v1.1 to avoid the fsnotify dependency in
	// v1 and to keep this method's contract trivial.
}

// ListWorkspaces returns the discovered workspaces with a running
// flag attached. Errors during discovery are returned to the UI.
func (a *App) ListWorkspaces() ([]WorkspaceInfo, error) {
	sources, err := config.Discover(config.DiscoverOptions{
		GlobalDir: config.DefaultGlobalDir(),
	})
	if err != nil {
		return nil, fmt.Errorf("discover: %w", err)
	}
	running := a.runningWorkspaces()
	out := make([]WorkspaceInfo, 0, len(sources))
	for _, s := range sources {
		out = append(out, WorkspaceInfo{
			Name:    s.Workspace.Name,
			Apps:    len(s.Workspace.Apps),
			Source:  sourceLabel(s),
			Path:    s.Path,
			Running: running[s.Workspace.Name],
		})
	}
	return out, nil
}

func sourceLabel(s config.Source) string {
	if s.Local {
		return "local"
	}
	return "global"
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
			},
			AppDetails: apps,
		}, nil
	}
	return nil, fmt.Errorf("workspace %q not found", name)
}

// StartWorkspace launches the named workspace and returns the
// resulting instance. Per-app failures are recorded on the instance;
// only an all-apps-failed workspace returns an error.
func (a *App) StartWorkspace(name string) (*InstanceInfo, error) {
	ws, src, err := a.findWorkspace(name)
	if err != nil {
		return nil, err
	}
	inst, err := a.rt.Start(ws, src)
	if err != nil {
		return nil, err
	}
	return toInstanceInfo(inst), nil
}

// StopInstance terminates one running instance by ID.
func (a *App) StopInstance(id string) error {
	if a.rt == nil {
		return errors.New("runtime not initialized")
	}
	return a.rt.Stop(id, false)
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
	if err := a.rt.StopAll(false); err != nil {
		return running, err
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

// Plugins returns the absolute paths of every dia-* executable on
// the process PATH.
func (a *App) Plugins() []string {
	return diag.ScanPlugins()
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

// NewWorkspace writes a starter YAML to the global config dir. The
// caller (UI) supplies the name; if a file with that name already
// exists, the operation is refused and the existing path is returned
// alongside the error so the user can pick a new name.
func (a *App) NewWorkspace(name string) (string, error) {
	if name == "" {
		return "", errors.New("name is required")
	}
	if err := config.ValidateName(name); err != nil {
		return "", err
	}
	dir := config.DefaultGlobalDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name+".yaml")
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("workspace %q already exists at %s", name, path)
	}
	body := starterYAML(name)
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
	return &InstanceInfo{
		ID:            inst.ID,
		Workspace:     inst.WorkspaceName,
		WorkspacePath: inst.WorkspacePath,
		StartedAt:     inst.StartedAt.UTC().Format(time.RFC3339),
		Status:        string(inst.Status),
		Apps:          apps,
	}
}

func starterYAML(name string) string {
	return fmt.Sprintf(`version: 1
name: %s
apps:
  - type: editor
    cmd: code
  - type: terminal
    cmd: bash
    args: ["-l"]
  - type: browser
    url: https://example.com
`, name)
}
