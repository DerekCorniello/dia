// Package state persists dia's runtime state: running instances, recent
// workspaces, and favorites. State is stored as JSON in the XDG state
// dir and written atomically.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DefaultTheme is the theme name used when none is persisted.
const DefaultTheme = "dia"

// Status is the lifecycle state of an app or instance.
type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusCrashed Status = "crashed"
)

// AppProcess describes a single spawned process tracked by dia.
type AppProcess struct {
	Type   string `json:"type"`
	Cmd    string `json:"cmd"`
	PID    int    `json:"pid"`
	Status Status `json:"status"`
	Err    string `json:"err,omitempty"`
}

// Instance is a running workspace: its generated ID, the workspace it
// came from, when it started, and the processes it owns.
type Instance struct {
	ID            string       `json:"id"`
	WorkspaceName string       `json:"workspace_name"`
	WorkspacePath string       `json:"workspace_path"`
	StartedAt     time.Time    `json:"started_at"`
	Apps          []AppProcess `json:"apps"`
	Status        Status       `json:"status"`
}

// CustomTheme is a user-defined daisyUI theme. Colors are stored as
// hex strings; the frontend converts them to the OKLCH CSS variables
// daisyUI v4 expects. ColorScheme is "light" or "dark" and seeds the
// `color-scheme` CSS property on the theme block.
type CustomTheme struct {
	ColorScheme string            `json:"color_scheme"`
	Colors      map[string]string `json:"colors"`
}

// Data is the on-disk representation of dia's state.
type Data struct {
	Version      int                    `json:"version"`
	Instances    map[string]Instance    `json:"instances"`
	Recent       []string               `json:"recent"`
	Favorites    []string               `json:"favorites"`
	Theme        string                 `json:"theme"`
	Keybindings  map[string]string      `json:"keybindings,omitempty"`
	CustomThemes map[string]CustomTheme `json:"custom_themes,omitempty"`
}

// Store guards a Data value persisted to a single JSON file. The
// zero value is not usable; call Open or OpenAt.
type Store struct {
	mu   sync.Mutex
	path string
	data Data
}

// Open opens the default state file, creating an empty one if needed.
func Open() (*Store, error) {
	dir, err := ResolveStateDir()
	if err != nil {
		return nil, err
	}
	return OpenAt(filepath.Join(dir, StateFile))
}

// OpenAt opens a Store backed by the file at path. A missing file is
// treated as an empty state; a corrupt file returns an error.
func OpenAt(path string) (*Store, error) {
	s := &Store{path: path, data: Data{Version: 1, Instances: map[string]Instance{}, CustomThemes: map[string]CustomTheme{}}}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return s, nil
		}
		return nil, fmt.Errorf("read state: %w", err)
	}
	if len(data) == 0 {
		return s, nil
	}
	var loaded Data
	if err := json.Unmarshal(data, &loaded); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	if loaded.Instances == nil {
		loaded.Instances = map[string]Instance{}
	}
	if loaded.CustomThemes == nil {
		loaded.CustomThemes = map[string]CustomTheme{}
	}
	s.data = loaded
	return s, nil
}

// Path returns the absolute path to the backing file.
func (s *Store) Path() string {
	return s.path
}

// Snapshot returns a deep-enough copy of the current state for the
// caller to read without holding the lock. The Instances map and slices
// are shared with the store, so callers must not mutate them; use
// Mutate to make changes.
func (s *Store) Snapshot() Data {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}

// Mutate runs fn with exclusive access to the state, then persists the
// result. If fn returns an error the state is not written.
func (s *Store) Mutate(fn func(d *Data)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.data)
	return s.writeLocked()
}

// MutateErr runs fn with exclusive access. If fn returns an error,
// nothing is written and that error is propagated.
func (s *Store) MutateErr(fn func(d *Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(&s.data); err != nil {
		return err
	}
	return s.writeLocked()
}

// Save persists the current state. Useful after Snapshot + manual edits
// (rare; prefer Mutate).
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked()
}

func (s *Store) writeLocked() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "state-*.json.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		cleanup()
		return err
	}
	return nil
}
