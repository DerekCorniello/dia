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

// AddRoot persists a directory to the roots list and rescans it for
// workspace YAMLs. Duplicates and non-directories are silently ignored.
func (a *App) AddRoot(dir string) error {
	if a.store == nil {
		return errors.New("state store not initialized")
	}
	return a.store.Mutate(func(d *state.Data) {
		for _, r := range d.Roots {
			if r == dir {
				return
			}
		}
		d.Roots = append(d.Roots, dir)
	})
}

// RemoveRoot removes a directory from the roots list.
func (a *App) RemoveRoot(dir string) error {
	if a.store == nil {
		return errors.New("state store not initialized")
	}
	return a.store.Mutate(func(d *state.Data) {
		out := make([]string, 0, len(d.Roots))
		for _, r := range d.Roots {
			if r != dir {
				out = append(out, r)
			}
		}
		d.Roots = out
	})
}

// ListRoots returns the persisted project roots.
func (a *App) ListRoots() []string {
	if a.store == nil {
		return nil
	}
	snap := a.store.Snapshot()
	out := make([]string, len(snap.Roots))
	copy(out, snap.Roots)
	return out
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
