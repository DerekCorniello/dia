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
	"strconv"
	"sync"
	"time"
)

// DefaultTheme is the theme name used when none is persisted.
const DefaultTheme = "dia"

// SchemaVersion is the newest state-file schema this build can read and
// write. New fields should be added with a migration before this changes.
const SchemaVersion = 1

// Status is the lifecycle state of an app or instance.
type Status string

const (
	StatusRunning  Status = "running"
	StatusStopped  Status = "stopped"
	StatusCrashed  Status = "crashed"
	StatusDegraded Status = "degraded"
)

// Live reports whether the status counts as active for lifecycle
// purposes: running and degraded instances are supervised, stoppable,
// and attachable. Stopped and crashed are terminal. Every lifecycle
// decision must use this instead of comparing against StatusRunning,
// or degraded workspaces become unmanageable (unstoppable, unwatched).
func (s Status) Live() bool {
	return s == StatusRunning || s == StatusDegraded
}

// AppProcess describes a single spawned process tracked by dia.
//
// Browser is set only for "browser" apps launched under an owned
// profile (the dedicated-profile strategy). For those, teardown goes
// through the browser.Surface using Browser rather than killing PID
// directly, because the meaningful thing to clean up is the owned
// browser instance and its ephemeral profile, not a bare PID.
type AppProcess struct {
	Type string `json:"type"`
	Cmd  string `json:"cmd"`
	PID  int    `json:"pid"`
	// Identity is an OS process-start token captured at launch. It prevents
	// a recycled PID from being mistaken for dia's child during reconcile.
	Identity string `json:"identity,omitempty"`
	Status   Status `json:"status"`
	Err      string `json:"err,omitempty"`
	// Note is a non-fatal caveat on a running app (e.g. a browser that
	// fell back to a plain launch dia cannot close on stop). It makes
	// degraded launches visible instead of silently downgraded.
	Note    string         `json:"note,omitempty"`
	Browser *BrowserHandle `json:"browser,omitempty"`
}

// BrowserHandle identifies a browser URL group dia opened under an
// owned profile, so it can be closed later -- even across a dia
// restart. It is self-describing: Strategy records which
// browser.Surface minted it, so teardown always uses the mechanism
// that actually opened the window.
type BrowserHandle struct {
	// Strategy is the browser.Surface identifier (e.g.
	// "dedicated-profile").
	Strategy string `json:"strategy"`
	// PID is the owned browser process. Because the launch used a
	// unique profile, this is a fresh instance dia owns and may kill.
	PID      int    `json:"pid"`
	Identity string `json:"identity,omitempty"`
	// ProfileDir is the ephemeral clone that backs this launch. It is
	// removed on close.
	ProfileDir string `json:"profile_dir"`
	// SeedDir is the writeback target: the dia-managed seed profile the
	// runtime profile is copied back to on close.
	SeedDir string `json:"seed_dir,omitempty"`
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
	Plugins       []string     `json:"plugins,omitempty"`
	PluginPIDs    []int        `json:"plugin_pids,omitempty"`
}

// CustomTheme is a user-defined daisyUI theme. Colors are stored as
// hex strings; the frontend converts them to the OKLCH CSS variables
// daisyUI v4 expects. ColorScheme is "light" or "dark" and seeds the
// `color-scheme` CSS property on the theme block.
type CustomTheme struct {
	ColorScheme string            `json:"color_scheme"`
	Colors      map[string]string `json:"colors"`
}

// PluginState persists a plugin's per-user settings across restarts.
// The GrantedCapabilities list is the user-approved subset of the
// manifest's requested capabilities. A plugin whose manifest changes
// its requested set will see the stored grants intersect with the
// new set on next load.
type PluginState struct {
	GrantedCapabilities []string       `json:"granted_capabilities,omitempty"`
	Config              map[string]any `json:"config,omitempty"`
}

// RecentEntry records a workspace's recency and usage. Count is
// incremented each time a workspace is started; the position in the
// Recent slice is the most-recent-first order.
type RecentEntry struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Data is the on-disk representation of dia's state.
type Data struct {
	Version      int                    `json:"version"`
	Instances    map[string]Instance    `json:"instances"`
	Recent       []RecentEntry          `json:"recent"`
	Favorites    []string               `json:"favorites"`
	Theme        string                 `json:"theme"`
	Keybindings  map[string]string      `json:"keybindings,omitempty"`
	CustomThemes map[string]CustomTheme `json:"custom_themes,omitempty"`
	Plugins      map[string]PluginState `json:"plugins,omitempty"`
	ProjectDir   string                 `json:"project_dir,omitempty"`
	// Roots are directories to scan for local workspace YAMLs.
	// Workspaces found here appear in every list regardless of CWD.
	Roots []string `json:"roots,omitempty"`
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
	// Every map has to be non-nil here, not just on the load path
	// below: a caller that writes into one on a brand new state file
	// (no state.json yet) would otherwise panic assigning to a nil map.
	s := &Store{path: path, data: Data{
		Version:      SchemaVersion,
		Instances:    map[string]Instance{},
		CustomThemes: map[string]CustomTheme{},
		Plugins:      map[string]PluginState{},
	}}
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("state path is a symlink: %s", path)
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("stat state: %w", err)
	}
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
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, fmt.Errorf("protect state: %w", err)
	}
	var raw rawData
	if err := json.Unmarshal(data, &raw); err != nil {
		backup := path + ".bak"
		if _, backupErr := os.Stat(backup); backupErr == nil {
			return nil, fmt.Errorf("parse state: %w (backup available at %s)", err, backup)
		}
		return nil, fmt.Errorf("parse state: %w", err)
	}
	if raw.Version > SchemaVersion {
		return nil, fmt.Errorf("state schema version %d is newer than supported version %d", raw.Version, SchemaVersion)
	}
	loaded := Data{
		Version:      SchemaVersion,
		Instances:    raw.Instances,
		Favorites:    raw.Favorites,
		Theme:        raw.Theme,
		Keybindings:  raw.Keybindings,
		CustomThemes: raw.CustomThemes,
		Plugins:      raw.Plugins,
		ProjectDir:   raw.ProjectDir,
		Roots:        raw.Roots,
	}
	if loaded.Instances == nil {
		loaded.Instances = map[string]Instance{}
	}
	if loaded.CustomThemes == nil {
		loaded.CustomThemes = map[string]CustomTheme{}
	}
	if loaded.Plugins == nil {
		loaded.Plugins = map[string]PluginState{}
	}
	loaded.Recent = migrateRecent(raw.Recent)
	s.data = loaded
	return s, nil
}

// rawData mirrors Data but with the Recent field kept as raw JSON so
// migration can inspect the on-disk format before decoding it.
type rawData struct {
	Version      int                    `json:"version"`
	Instances    map[string]Instance    `json:"instances"`
	Recent       []json.RawMessage      `json:"recent"`
	Favorites    []string               `json:"favorites"`
	Theme        string                 `json:"theme"`
	Keybindings  map[string]string      `json:"keybindings,omitempty"`
	CustomThemes map[string]CustomTheme `json:"custom_themes,omitempty"`
	Plugins      map[string]PluginState `json:"plugins,omitempty"`
	ProjectDir   string                 `json:"project_dir,omitempty"`
	Roots        []string               `json:"roots,omitempty"`
}

// migrateRecent normalizes a freshly-loaded Recent slice. The legacy
// on-disk format was []string; we coerce that to []RecentEntry with
// Count 0 so older state files load without error.
func migrateRecent(in []json.RawMessage) []RecentEntry {
	if len(in) == 0 {
		return nil
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return nil
	}
	var first any
	if err := json.Unmarshal(in[0], &first); err != nil {
		return nil
	}
	if _, ok := first.(string); ok {
		var legacy []string
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return nil
		}
		out := make([]RecentEntry, 0, len(legacy))
		for _, n := range legacy {
			out = append(out, RecentEntry{Name: n, Count: 0})
		}
		return out
	}
	var entries []RecentEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}
	return entries
}

// Path returns the absolute path to the backing file.
func (s *Store) Path() string {
	return s.path
}

// Reload re-reads the backing file, replacing the in-memory state with
// whatever is on disk. The daemon holds the store open for its whole
// life and applies persisted grants and roots on each start, so it must
// reload after another client (the GUI, a plugin grant, a `dia new
// --dir` root) has written the file behind its back. Slow only for the
// caller's snapshot; Mutate flushes it back.
func (s *Store) Reload() error {
	fresh, err := OpenAt(s.path)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.data = fresh.data
	s.mu.Unlock()
	return nil
}

// Snapshot returns an independent copy of the current state for the
// caller to read without holding the lock. Callers may safely retain or
// modify the returned value; use Mutate to persist changes.
func (s *Store) Snapshot() Data {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneData(s.data)
}

func cloneData(in Data) Data {
	out := in
	out.Instances = make(map[string]Instance, len(in.Instances))
	for id, inst := range in.Instances {
		cp := inst
		cp.Apps = append([]AppProcess(nil), inst.Apps...)
		for i := range cp.Apps {
			if inst.Apps[i].Browser != nil {
				h := *inst.Apps[i].Browser
				cp.Apps[i].Browser = &h
			}
		}
		cp.Plugins = append([]string(nil), inst.Plugins...)
		cp.PluginPIDs = append([]int(nil), inst.PluginPIDs...)
		out.Instances[id] = cp
	}
	out.Recent = append([]RecentEntry(nil), in.Recent...)
	out.Favorites = append([]string(nil), in.Favorites...)
	out.Roots = append([]string(nil), in.Roots...)
	if in.Keybindings != nil {
		out.Keybindings = mapsClone(in.Keybindings)
	}
	if in.CustomThemes != nil {
		out.CustomThemes = make(map[string]CustomTheme, len(in.CustomThemes))
		for name, theme := range in.CustomThemes {
			cp := theme
			if theme.Colors != nil {
				cp.Colors = mapsClone(theme.Colors)
			}
			out.CustomThemes[name] = cp
		}
	}
	if in.Plugins != nil {
		out.Plugins = make(map[string]PluginState, len(in.Plugins))
		for id, plugin := range in.Plugins {
			cp := plugin
			cp.GrantedCapabilities = append([]string(nil), plugin.GrantedCapabilities...)
			if plugin.Config != nil {
				cp.Config = cloneAnyMap(plugin.Config)
			}
			out.Plugins[id] = cp
		}
	}
	return out
}

func mapsClone[V any](in map[string]V) map[string]V {
	out := make(map[string]V, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAny(v)
	}
	return out
}

func cloneAny(v any) any {
	switch value := v.(type) {
	case map[string]any:
		return cloneAnyMap(value)
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = cloneAny(item)
		}
		return out
	case []string:
		return append([]string(nil), value...)
	default:
		return v
	}
}

// Mutate runs fn with exclusive access to the state, then persists the
// result. If fn returns an error the state is not written. On write
// failure the in-memory and on-disk states may diverge; restarting dia
// recovers the last persisted state.
func (s *Store) Mutate(fn func(d *Data)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	release, err := acquireStateLock(s.path + ".lock")
	if err != nil {
		return err
	}
	defer release()
	if err := s.reloadLocked(); err != nil {
		return err
	}
	fn(&s.data)
	return s.writeLocked()
}

// MutateIfChanged runs fn with exclusive access to the state and
// persists the result only if fn reports a change, by returning true.
// Prefer this over Mutate for a hot loop that usually has nothing to
// write, such as a periodic liveness poll: Mutate's unconditional
// write turns "nothing changed" into a full marshal, temp file, and
// atomic rename on every call.
func (s *Store) MutateIfChanged(fn func(d *Data) bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	release, err := acquireStateLock(s.path + ".lock")
	if err != nil {
		return err
	}
	defer release()
	if err := s.reloadLocked(); err != nil {
		return err
	}
	if !fn(&s.data) {
		return nil
	}
	return s.writeLocked()
}

// MutateErr runs fn with exclusive access. If fn returns an error,
// nothing is written and that error is propagated.
func (s *Store) MutateErr(fn func(d *Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	release, err := acquireStateLock(s.path + ".lock")
	if err != nil {
		return err
	}
	defer release()
	if err := s.reloadLocked(); err != nil {
		return err
	}
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
	release, err := acquireStateLock(s.path + ".lock")
	if err != nil {
		return err
	}
	defer release()
	return s.writeLocked()
}

func (s *Store) reloadLocked() error {
	fresh, err := OpenAt(s.path)
	if err != nil {
		return err
	}
	s.data = fresh.data
	return nil
}

func acquireStateLock(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create state lock directory: %w", err)
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, _ = f.WriteString(strconv.Itoa(os.Getpid()))
			_ = f.Sync()
			return func() {
				_ = f.Close()
				_ = os.Remove(path)
			}, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire state lock: %w", err)
		}
		if owner, readErr := os.ReadFile(path); readErr == nil {
			if pid, parseErr := strconv.Atoi(string(owner)); parseErr == nil && pid > 0 && !lockOwnerAlive(pid) {
				_ = os.Remove(path)
				continue
			}
		}
		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > time.Hour {
			_ = os.Remove(path)
			continue
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("acquire state lock: timed out waiting for %s", path)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (s *Store) writeLocked() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
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
	if previous, err := os.ReadFile(s.path); err == nil {
		if err := atomicBackup(s.path+".bak", previous); err != nil {
			cleanup()
			return fmt.Errorf("backup state: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		cleanup()
		return fmt.Errorf("read previous state: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		cleanup()
		return err
	}
	if err := os.Chmod(s.path, 0o600); err != nil {
		return err
	}
	if err := syncDir(dir); err != nil {
		return err
	}
	return nil
}

func atomicBackup(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "state-backup-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
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
	if err := os.Rename(tmpName, path); err != nil {
		cleanup()
		return err
	}
	return nil
}
