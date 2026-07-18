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
	mu        sync.RWMutex
	globalDir string
	localDir  string
	loaded    map[string]*Loaded
	runtimes  map[string]*Runtime
	// resolvers hold the pure app-type runtimes, kept apart from
	// runtimes so resolving an app type does not require the plugin's
	// panel to be running and shares no state with it.
	resolvers map[string]*Runtime
	// appTypes maps a claimed app type to the plugin id providing it.
	appTypes map[string]string
	// appTypeConflicts records claims refused during the last rebuild.
	appTypeConflicts []AppTypeConflict
	host             HostAPI
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
		resolvers: map[string]*Runtime{},
		appTypes:  map[string]string{},
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
			m.dropResolverLocked(id)
			delete(m.loaded, id)
		}
	}
	m.rebuildAppTypesLocked()
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
			existing.GrantedCaps = GrantCapabilities(manifest.Capabilities, DefaultReadCapabilities())
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
	grants := GrantCapabilities(l.Manifest.Capabilities, DefaultReadCapabilities())
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
	// Grants decide which app types this plugin may claim, so any
	// change to them has to reach the index and invalidate a resolver
	// built under the old set.
	defer func() {
		m.dropResolverLocked(l.Manifest.ID)
		m.rebuildAppTypesLocked()
	}()
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

// SetGrants replaces a plugin's granted capabilities without touching
// its enabled state, and refreshes anything derived from them.
func (m *Manager) SetGrants(id string, grants []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	l, ok := m.loaded[id]
	if !ok {
		return fmt.Errorf("plugin %q not found", id)
	}
	if l.Manifest == nil {
		return fmt.Errorf("plugin %q has no valid manifest: %s", id, l.LastError)
	}
	l.GrantedCaps = GrantCapabilities(l.Manifest.Capabilities, grants)
	m.dropResolverLocked(id)
	m.rebuildAppTypesLocked()
	return nil
}
func (m *Manager) Call(id, method string, args []any) (any, error) {
	m.mu.RLock()
	rt, ok := m.runtimes[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("plugin %q is not enabled", id)
	}
	ctx, cancel := callCtx()
	defer cancel()
	return rt.Call(ctx, method, args)
}
func (m *Manager) List() []Loaded {
	m.mu.RLock()
	defer m.mu.RUnlock()
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
	m.mu.RLock()
	defer m.mu.RUnlock()
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

// InstallFrom installs from a parsed source, cloning first when the
// source is a git remote. cwd is only consulted for a local install
// and may be empty otherwise.
func (m *Manager) InstallFrom(src InstallSource, local bool, cwd string) (string, error) {
	dstBase := m.globalDir
	if local {
		if cwd == "" {
			return "", errors.New("cwd is required for local install")
		}
		dstBase = LocalPluginsDir(cwd)
	}
	srcDir, cleanup, err := src.materialize()
	if err != nil {
		return "", err
	}
	defer cleanup()

	dst, err := m.installAt(srcDir, dstBase)
	if err != nil {
		return dst, err
	}
	if src.Kind == InstallGit {
		if err := writeProvenance(dst, Provenance{URL: src.URL, Ref: src.Ref}); err != nil {
			// The plugin is installed and works; only `update` is lost.
			return dst, fmt.Errorf("plugin installed to %s, but recording its source failed "+
				"(dia plugin update will not work for it): %w", dst, err)
		}
	}
	return dst, nil
}

// InspectSource materializes a source (cloning it if it is a remote)
// and returns its manifest along with the directory it landed in. The
// CLI uses this to show the user which capabilities a plugin asks for
// before its code is copied into the plugins dir, then passes the same
// directory to InstallMaterialized so the repository is fetched once
// and the manifest shown is exactly the one installed.
//
// The caller must invoke the returned cleanup.
func (m *Manager) InspectSource(src InstallSource) (manifest *Manifest, dir string, cleanup func(), err error) {
	dir, cleanup, err = src.materialize()
	if err != nil {
		return nil, "", nil, err
	}
	manifest, err = LoadManifest(dir)
	if err != nil {
		cleanup()
		return nil, "", nil, err
	}
	return manifest, dir, cleanup, nil
}

// InstallMaterialized installs from an already-materialized directory,
// recording prov as its source. It pairs with InspectSource so a
// confirmed install does not clone the repository a second time.
func (m *Manager) InstallMaterialized(srcDir string, local bool, cwd string, prov *Provenance) (string, error) {
	dstBase := m.globalDir
	if local {
		if cwd == "" {
			return "", errors.New("cwd is required for local install")
		}
		dstBase = LocalPluginsDir(cwd)
	}
	dst, err := m.installAt(srcDir, dstBase)
	if err != nil {
		return dst, err
	}
	if prov != nil {
		if err := writeProvenance(dst, *prov); err != nil {
			return dst, fmt.Errorf("plugin installed to %s, but recording its source failed "+
				"(dia plugin update will not work for it): %w", dst, err)
		}
	}
	return dst, nil
}

// Update re-clones a git-installed plugin and replaces it in place.
// The old directory is only removed once the new copy has been cloned
// and its manifest validated, so a failed update leaves the working
// plugin untouched.
func (m *Manager) Update(id string) (string, error) {
	m.mu.RLock()
	l, ok := m.loaded[id]
	m.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("plugin %q not found", id)
	}
	dir := l.Dir

	prov, hasProv, err := ReadProvenance(dir)
	if err != nil {
		return "", err
	}
	if !hasProv {
		return "", fmt.Errorf("plugin %q was installed from a local path, so there is nothing to update from; "+
			"reinstall it with `dia plugin install <path>`", id)
	}

	src := InstallSource{Kind: InstallGit, URL: prov.URL, Ref: prov.Ref}
	fresh, cleanup, err := src.materialize()
	if err != nil {
		return "", err
	}
	defer cleanup()

	manifest, err := LoadManifest(fresh)
	if err != nil {
		return "", fmt.Errorf("updated plugin at %s is invalid, keeping the installed version: %w", prov.URL, err)
	}
	if manifest.ID != id {
		return "", fmt.Errorf("updated plugin declares id %q but %q is installed; refusing to replace it",
			manifest.ID, id)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if rt, ok := m.runtimes[id]; ok {
		_ = rt.Close()
		delete(m.runtimes, id)
	}
	m.dropResolverLocked(id)
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("remove old plugin dir: %w", err)
	}
	if err := copyDir(fresh, dir); err != nil {
		return "", fmt.Errorf("copy updated plugin: %w", err)
	}
	if err := writeProvenance(dir, prov); err != nil {
		return dir, fmt.Errorf("plugin updated, but recording its source failed: %w", err)
	}

	// Preserve the user's grants and enabled state across the update:
	// an update is not a reinstall, and silently dropping capabilities
	// would break a working plugin.
	granted := GrantCapabilities(manifest.Capabilities, l.GrantedCaps)
	m.loaded[id] = &Loaded{
		Manifest:    manifest,
		Dir:         dir,
		Source:      l.Source,
		GrantedCaps: granted,
		Config:      l.Config,
		Enabled:     l.Enabled,
		Status:      StatusLoaded,
	}
	m.rebuildAppTypesLocked()
	return dir, nil
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
		GrantedCaps: GrantCapabilities(manifest.Capabilities, DefaultReadCapabilities()),
		Status:      StatusLoaded,
	}
	m.rebuildAppTypesLocked()
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
	m.dropResolverLocked(id)
	if l.Dir != "" {
		if err := os.RemoveAll(l.Dir); err != nil {
			return fmt.Errorf("remove plugin dir: %w", err)
		}
	}
	delete(m.loaded, id)
	m.rebuildAppTypesLocked()
	return nil
}
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, rt := range m.runtimes {
		_ = rt.Close()
		delete(m.runtimes, id)
	}
	for id := range m.resolvers {
		m.dropResolverLocked(id)
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
func callCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), pluginCallTimeout)
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
		// A git-installed plugin arrives as a clone; its history is
		// dead weight in the plugins dir and would be the largest
		// thing in it.
		if e.IsDir() && e.Name() == ".git" {
			continue
		}
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
