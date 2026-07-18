package wailsapp

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/diag"
	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/state"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

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
		if err := a.store.Mutate(func(d *state.Data) {
			d.ProjectDir = dir
		}); err != nil {
			a.logger.Warn("mutate project dir", "error", err)
		}
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

// RevealPath opens an arbitrary path in the file manager.
func (a *App) RevealPath(path string) error {
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return platform.New().RevealInFileManager(path)
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
	// A plugin that lost an app-type claim is otherwise invisible: the
	// workspace starts and launches the winner's command instead.
	if a.pmgr != nil {
		for _, c := range a.pmgr.AppTypeConflicts() {
			out = append(out, CheckInfo{
				Name:   "app type conflict",
				Status: "warn",
				Detail: fmt.Sprintf("%q claimed by %s and %s; using %s", c.Type, c.Winner, c.Loser, c.Winner),
			})
		}
	}
	return out
}
