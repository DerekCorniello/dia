package plugins

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Status string

const (
	StatusActive  Status = "active"
	StatusErrored Status = "errored"
	StatusLoaded  Status = "loaded"
)

type Source string

const (
	SourceGlobal Source = "global"
	SourceLocal  Source = "local"
)

type Loaded struct {
	Manifest    *Manifest
	Dir         string
	Source      Source
	GrantedCaps []string
	Config      map[string]any
	Enabled     bool
	Status      Status
	LastError   string
	LastRefresh time.Time
}
type Manager struct {
	mu        sync.Mutex
	globalDir string
	localDir  string
	loaded    map[string]*Loaded
	runtimes  map[string]*Runtime
	host      HostAPI
}

func NewManager(globalDir string, host HostAPI) (*Manager, error) {
	if globalDir == "" {
		return nil, errors.New("global plugins dir is empty")
	}
	if host == nil {
		return nil, errors.New("host is nil")
	}
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir global plugins dir: %w", err)
	}
	return &Manager{
		globalDir: globalDir,
		loaded:    map[string]*Loaded{},
		runtimes:  map[string]*Runtime{},
		host:      host,
	}, nil
}
func (m *Manager) SetLocalDir(cwd string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cwd == "" {
		m.localDir = ""
		return
	}
	m.localDir = LocalPluginsDir(cwd)
	if err := os.MkdirAll(m.localDir, 0o755); err != nil {
		m.localDir = ""
	}
}
func (m *Manager) GlobalDir() string { return m.globalDir }
func (m *Manager) LocalDir() string  { return m.localDir }
func (m *Manager) Discover() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := map[string]bool{}
	if m.localDir != "" {
		if err := m.scanDir(m.localDir, SourceLocal, seen); err != nil {
			return err
		}
	}
	if m.globalDir != "" {
		if err := m.scanDir(m.globalDir, SourceGlobal, seen); err != nil {
			return err
		}
	}
	for id := range m.loaded {
		if !seen[id] {
			if rt, ok := m.runtimes[id]; ok {
				_ = rt.Close()
				delete(m.runtimes, id)
			}
			delete(m.loaded, id)
		}
	}
	return nil
}
func (m *Manager) scanDir(dir string, source Source, seen map[string]bool) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read plugins dir %s: %w", dir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		if !validID(id) {
			continue
		}
		seen[id] = true
		full := filepath.Join(dir, id)
		if existing, ok := m.loaded[id]; ok && existing.Source != source {
			continue
		}
		manifest, err := LoadManifest(full)
		if err != nil {
			if existing, ok := m.loaded[id]; ok && existing.Source == source {
				existing.Status = StatusErrored
				existing.LastError = err.Error()
			} else if !ok {
				m.loaded[id] = &Loaded{
					Dir:       full,
					Source:    source,
					Status:    StatusErrored,
					LastError: err.Error(),
				}
			}
			continue
		}
		existing, ok := m.loaded[id]
		if !ok {
			existing = &Loaded{Dir: full, Source: source}
		}
		existing.Manifest = manifest
		existing.Dir = full
		existing.Source = source
		if existing.GrantedCaps == nil {
			existing.GrantedCaps = MergeCapabilities(DefaultReadCapabilities(), manifest.Capabilities)
		}
		if existing.Status == "" || existing.Status == StatusErrored {
			existing.Status = StatusLoaded
		}
		m.loaded[id] = existing
	}
	return nil
}
func (m *Manager) Enable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.loaded[id]
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	if l.Manifest == nil {
		return fmt.Errorf("plugin %q has no valid manifest: %s", id, l.LastError)
	}
	grants := MergeCapabilities(DefaultReadCapabilities(), l.Manifest.Capabilities)
	return m.enableLocked(l, grants)
}

// EnableWithGrants enables a plugin and restricts the granted
// capability set to grants (intersected with the manifest's
// requested set). Use this to apply a persisted user-approved
// capability list from a previous session.
func (m *Manager) EnableWithGrants(id string, grants []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.loaded[id]
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	if l.Manifest == nil {
		return fmt.Errorf("plugin %q has no valid manifest: %s", id, l.LastError)
	}
	return m.enableLocked(l, GrantCapabilities(l.Manifest.Capabilities, grants))
}

func (m *Manager) enableLocked(l *Loaded, grants []string) error {
	if l.Enabled {
		l.GrantedCaps = grants
		return nil
	}
	rt, err := NewRuntime(l.Manifest, l.Dir, m.host, grants, l.Config)
	if err != nil {
		l.Status = StatusErrored
		l.LastError = err.Error()
		return err
	}
	if err := rt.Load(); err != nil {
		l.Status = StatusErrored
		l.LastError = err.Error()
		return err
	}
	m.runtimes[l.Manifest.ID] = rt
	l.GrantedCaps = grants
	l.Enabled = true
	l.Status = StatusActive
	l.LastError = ""
	return nil
}
func (m *Manager) Disable(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.loaded[id]
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	rt, ok := m.runtimes[id]
	if !ok {
		l.Enabled = false
		return nil
	}
	if err := rt.Close(); err != nil {
		l.LastError = err.Error()
	}
	delete(m.runtimes, id)
	l.Enabled = false
	if l.Status == StatusActive {
		l.Status = StatusLoaded
	}
	return nil
}
func (m *Manager) Call(id, method string, args []any) (any, error) {
	m.mu.Lock()
	rt, ok := m.runtimes[id]
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("plugin %q is not enabled", id)
	}
	return rt.Call(callCtx(), method, args)
}
func (m *Manager) List() []Loaded {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Loaded, 0, len(m.loaded))
	for _, l := range m.loaded {
		snap := *l
		out = append(out, snap)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Manifest.ID != out[j].Manifest.ID {
			return out[i].Manifest.ID < out[j].Manifest.ID
		}
		return string(out[i].Source) < string(out[j].Source)
	})
	return out
}
func (m *Manager) Get(id string) (Loaded, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.loaded[id]
	if !ok {
		return Loaded{}, false
	}
	return *l, true
}

// Loaded returns a deep-enough snapshot of a single plugin's state
// (manifest, dir, source, granted caps, enabled flag, status, last
// error). The bool reports whether the id was discovered. Callers
// must not mutate the returned struct.
func (m *Manager) Loaded(id string) (Loaded, bool) { return m.Get(id) }

// SetConfig stores a config map on the loaded plugin. The config
// is passed to the goja runtime as dia.getConfig(). Safe to call
// before or after Enable.
func (m *Manager) SetConfig(id string, cfg map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.loaded[id]
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	l.Config = cfg
	return nil
}
func (m *Manager) Install(srcDir string) (string, error) {
	return m.installAt(srcDir, m.globalDir)
}
func (m *Manager) InstallLocal(srcDir, cwd string) (string, error) {
	if cwd == "" {
		return "", errors.New("cwd is required for local install")
	}
	return m.installAt(srcDir, LocalPluginsDir(cwd))
}
func (m *Manager) installAt(srcDir, dstBase string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	manifest, err := LoadManifest(srcDir)
	if err != nil {
		return "", err
	}
	dst := filepath.Join(dstBase, manifest.ID)
	if _, err := os.Stat(dst); err == nil {
		return dst, fmt.Errorf("plugin %q already installed at %s", manifest.ID, dst)
	}
	if err := copyDir(srcDir, dst); err != nil {
		return "", fmt.Errorf("copy plugin: %w", err)
	}
	m.loaded[manifest.ID] = &Loaded{
		Manifest:    manifest,
		Dir:         dst,
		Source:      SourceOfDir(dstBase),
		GrantedCaps: MergeCapabilities(DefaultReadCapabilities(), manifest.Capabilities),
		Status:      StatusLoaded,
	}
	return dst, nil
}
func (m *Manager) Uninstall(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.loaded[id]
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	if rt, ok := m.runtimes[id]; ok {
		_ = rt.Close()
		delete(m.runtimes, id)
	}
	if l.Dir != "" {
		if err := os.RemoveAll(l.Dir); err != nil {
			return fmt.Errorf("remove plugin dir: %w", err)
		}
	}
	delete(m.loaded, id)
	return nil
}
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, rt := range m.runtimes {
		_ = rt.Close()
		delete(m.runtimes, id)
	}
}
func GlobalPluginsDir(stateDir string) string {
	return filepath.Join(stateDir, pluginsDirName)
}
func LocalPluginsDir(cwd string) string {
	return filepath.Join(cwd, ".dia", "plugins")
}
func SourceOfDir(dir string) Source {
	if strings.Contains(dir, string(filepath.Separator)+".dia"+string(filepath.Separator)) {
		return SourceLocal
	}
	return SourceGlobal
}
func callCtx() context.Context {
	return context.Background()
}
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(s, d); err != nil {
				return err
			}
			continue
		}
		data, err := os.ReadFile(s)
		if err != nil {
			return err
		}
		if err := os.WriteFile(d, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}
func isPluginFile(name string) bool {
	return !strings.HasPrefix(name, ".")
}
