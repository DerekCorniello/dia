package wailsapp

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/plugins"
	"github.com/DerekCorniello/dia/internal/registry"
	dia "github.com/DerekCorniello/dia/internal/runtime"
	"github.com/DerekCorniello/dia/internal/state"
	"github.com/fsnotify/fsnotify"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the wails-bound application surface. Methods on App are
// exposed to the Svelte frontend via the generated
// wailsjs/go/wailsapp/App module. main.go binds *App directly;
// routing this through a main-package facade was tried first and
// the generator still followed the methods' return types into
// the wailsapp package.
type App struct {
	ctx context.Context

	store  *state.Store
	rt     *dia.Runtime
	reg    *registry.Registry
	pmgr   *plugins.Manager
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
