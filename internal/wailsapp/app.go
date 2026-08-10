package wailsapp

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DerekCorniello/dia/internal/daemon"
	"github.com/DerekCorniello/dia/internal/plugins"
	"github.com/DerekCorniello/dia/internal/registry"
	"github.com/DerekCorniello/dia/internal/state"
	"github.com/fsnotify/fsnotify"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the wails-bound application surface. Methods on App are
// exposed to the Svelte frontend via the generated
// wailsjs/go/wailsapp/App module. main.go binds *App directly;
// routing this through a main-package facade binding the generator
// still followed the methods' return types into the wailsapp package.
//
// App owns what has to stay in-process: the plugin panels, grants,
// themes, and workspace files. Session lifecycle (start/stop/restart/
// reconcile) belongs to the daemon; StartWorkspace et al. are thin
// clients over its socket, exactly like the CLI.
type App struct {
	ctx context.Context

	store  *state.Store
	reg    *registry.Registry
	pmgr   *plugins.Manager
	logger *slog.Logger

	dialOnce sync.Once
	dae      *daemon.Client
	dialErr  error
}

// New returns an App with no context set; Startup fills it in and
// constructs the runtime.
func New() *App {
	return &App{}
}

// Startup is called by the wails runtime after the window is created.
// The context is required for any runtime calls (events, dialogs).
// Startup wires up the plugin and state handles; the session daemon
// is dialed lazily on first use so tests that only exercise the
// plugin/theme surface never spawn the daemon.
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))

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
		a.registerPluginAppTypes(pmgr)
		a.pmgr = pmgr
	}
}

// StateDir returns the state directory this app is bound to, or "" when
// not initialized.
func (a *App) StateDir() string {
	if a.store == nil {
		return ""
	}
	return filepath.Dir(a.store.Path())
}

// daemonClient dials the single daemon for this app, spawning it once
// via the current executable. The connection lives for the whole app
// lifetime (Close is never called); the daemon is what keeps
// workspaces alive when the GUI quits.
func (a *App) daemonClient() (*daemon.Client, error) {
	if a.store == nil {
		return nil, errors.New("not initialized")
	}
	a.dialOnce.Do(func() {
		a.dae, a.dialErr = daemon.Ensure(daemon.EnsureOpts{StateDir: a.StateDir()})
	})
	if a.dialErr != nil {
		return nil, a.dialErr
	}
	return a.dae, nil
}

// registerPluginAppTypes adds every app type claimed by a plugin to
// the registry, so a workspace can use one the same way it uses a
// built-in type. Conflicts are reported rather than resolved silently:
// a workspace launching something other than what its author meant is
// worth a log line.
func (a *App) registerPluginAppTypes(pmgr *plugins.Manager) {
	for _, err := range registry.SyncPluginTypes(a.reg, pmgr) {
		a.logger.Warn("register plugin app type", "error", err)
	}
	for _, c := range pmgr.AppTypeConflicts() {
		a.logger.Warn("app type claimed by more than one plugin",
			"type", c.Type, "using", c.Winner, "ignored", c.Loser)
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
	statePath := a.store.Path()
	stateName := filepath.Base(statePath)
	stateDir := filepath.Dir(statePath)
	if err := w.Add(stateDir); err != nil {
		w.Close()
		a.logger.Warn("watch state dir", "error", err)
		return
	}
	go func() {
		defer w.Close()
		var debounce *time.Timer
		for {
			select {
			case ev := <-w.Events:
				if filepath.Base(ev.Name) != stateName {
					continue
				}
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
