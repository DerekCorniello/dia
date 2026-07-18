package plugins

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/dop251/goja"
)

// resolveTimeout bounds a single resolveApp call. Resolution happens
// on the path to launching a workspace, so it has to be short: a
// plugin that hangs here would hang `dia start`.
const resolveTimeout = 5 * time.Second

// AppDescriptor is what a plugin's resolveApp returns: either a
// command to launch or a URL to open, never both. It mirrors the
// registry's Action without coupling this package to it.
type AppDescriptor struct {
	Cmd  string            `json:"cmd,omitempty"`
	Args []string          `json:"args,omitempty"`
	Cwd  string            `json:"cwd,omitempty"`
	Env  map[string]string `json:"env,omitempty"`
	URL  string            `json:"url,omitempty"`
}

// Validate enforces the one-of rule. A descriptor that sets both, or
// neither, is a plugin bug, and saying so precisely beats launching
// something the author did not intend.
func (d AppDescriptor) Validate() error {
	hasCmd := d.Cmd != ""
	hasURL := d.URL != ""
	switch {
	case hasCmd && hasURL:
		return errors.New(`resolveApp returned both "cmd" and "url"; it must return exactly one`)
	case !hasCmd && !hasURL:
		return errors.New(`resolveApp returned neither "cmd" nor "url"; it must return exactly one`)
	}
	if hasURL && (len(d.Args) > 0 || d.Cwd != "" || len(d.Env) > 0) {
		return errors.New(`resolveApp returned "url" alongside cmd-only fields (args/cwd/env)`)
	}
	return nil
}

// NewResolverRuntime builds a goja runtime for resolving app types.
//
// The resolver is deliberately pure: the dia object exposes only the
// plugin's own config and directory, with no access to workspaces,
// exec, or fetch. Three reasons. It keeps `dia start --dry-run`
// deterministic and side-effect free. It means the CLI does not need a
// HostAPI implementation just to launch a workspace. And it keeps a
// resolver from making a network call in the middle of a start, where
// the user has no way to see or interrupt it.
func NewResolverRuntime(manifest *Manifest, pluginDir string, cfg map[string]any) (*Runtime, error) {
	if manifest == nil {
		return nil, errors.New("manifest is nil")
	}
	rt := goja.New()
	entry := filepath.Clean(filepath.Join(pluginDir, manifest.Entry))
	// An empty grant set: every capability-gated method is unreachable,
	// and the resolver object below does not expose them anyway.
	bridge := NewBridge(rt, manifest.Capabilities, nil, nil, cfg)
	bridge.pluginDir = pluginDir
	for k, v := range map[string]any{
		"dia":         bridge.ResolverDiaObject(),
		"require":     bridge.NewRequire(pluginDir),
		"__pluginDir": pluginDir,
	} {
		if err := rt.Set(k, v); err != nil {
			return nil, fmt.Errorf("set %q on resolver runtime: %w", k, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Runtime{
		rt:       rt,
		manifest: manifest,
		entry:    entry,
		bridge:   bridge,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// ResolverDiaObject is the dia global a resolver sees. It is the
// read-your-own-config subset: nothing here can reach the host.
func (b *Bridge) ResolverDiaObject() map[string]any {
	return map[string]any{
		"getConfig": b.getConfig,
		"pluginDir": b.getPluginDir,
	}
}

// ResolveApp asks the plugin that claims typeName to turn an app
// config into a descriptor. app is the marshaled config.App, and the
// result is the marshaled descriptor; passing plain maps keeps this
// package independent of internal/registry and internal/config.
func (m *Manager) ResolveApp(typeName string, app map[string]any) (map[string]any, error) {
	m.mu.RLock()
	id, ok := m.appTypes[typeName]
	var l *Loaded
	if ok {
		l = m.loaded[id]
	}
	m.mu.RUnlock()
	if !ok || l == nil {
		return nil, fmt.Errorf("no plugin provides app type %q", typeName)
	}
	if !HasCapability(l.GrantedCaps, CapAppsResolve) {
		return nil, fmt.Errorf("plugin %q provides app type %q but does not have the %q capability granted",
			id, typeName, CapAppsResolve)
	}

	rt, err := m.resolverFor(id, l)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()
	out, err := rt.Call(ctx, "resolveApp", []any{app})
	if err != nil {
		return nil, fmt.Errorf("plugin %q resolveApp(%s): %w", id, typeName, err)
	}
	if out == nil {
		return nil, fmt.Errorf("plugin %q resolveApp(%s) returned nothing", id, typeName)
	}
	desc, ok := out.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("plugin %q resolveApp(%s) returned %T, want an object", id, typeName, out)
	}
	return desc, nil
}

// resolverFor returns the plugin's resolver runtime, loading it on
// first use and caching it. Resolvers are cached separately from panel
// runtimes: a resolver must work even when the plugin's panel is not
// running, and it must not share mutable state with one.
func (m *Manager) resolverFor(id string, l *Loaded) (*Runtime, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rt, ok := m.resolvers[id]; ok {
		return rt, nil
	}
	rt, err := NewResolverRuntime(l.Manifest, l.Dir, l.Config)
	if err != nil {
		return nil, fmt.Errorf("plugin %q: %w", id, err)
	}
	if err := rt.Load(); err != nil {
		return nil, fmt.Errorf("plugin %q: %w", id, err)
	}
	m.resolvers[id] = rt
	return rt, nil
}

// DecodeDescriptor converts a resolveApp result into a typed
// descriptor and validates it.
func DecodeDescriptor(m map[string]any) (AppDescriptor, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return AppDescriptor{}, err
	}
	var d AppDescriptor
	dec := json.NewDecoder(bytes.NewReader(data))
	// Reject unknown keys: a typo like "command" would otherwise be
	// silently dropped and surface as "returned neither cmd nor url".
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return AppDescriptor{}, fmt.Errorf("resolveApp returned an unusable object: %w", err)
	}
	if err := d.Validate(); err != nil {
		return AppDescriptor{}, err
	}
	return d, nil
}

// AppTypes returns the app types currently claimed, mapped to the
// plugin id providing each.
func (m *Manager) AppTypes() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]string, len(m.appTypes))
	for t, id := range m.appTypes {
		out[t] = id
	}
	return out
}

// ApplyPersistedGrants applies a saved per-plugin capability list to
// every discovered plugin, without loading any panel runtime. The CLI
// needs the grants (they decide which app types are claimed) but has
// no use for the panels, and loading a plugin's UI code to run
// `dia start` would be both slow and a needless place to fail.
//
// Unknown ids are ignored: a plugin can be uninstalled while its
// persisted state lingers.
func (m *Manager) ApplyPersistedGrants(grants map[string][]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, caps := range grants {
		l, ok := m.loaded[id]
		if !ok || l.Manifest == nil {
			continue
		}
		l.GrantedCaps = GrantCapabilities(l.Manifest.Capabilities, caps)
		m.dropResolverLocked(id)
	}
	m.rebuildAppTypesLocked()
}

// AppTypeConflict describes two plugins claiming the same app type.
type AppTypeConflict struct {
	Type string
	// Winner is the plugin whose claim was kept.
	Winner string
	// Loser is the plugin whose claim was refused.
	Loser string
}

// AppTypeConflicts returns the claims that were refused on the last
// index rebuild. `dia doctor` surfaces these: silently letting one
// plugin win would make a workspace launch something other than what
// its author intended, with no indication why.
func (m *Manager) AppTypeConflicts() []AppTypeConflict {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AppTypeConflict, len(m.appTypeConflicts))
	copy(out, m.appTypeConflicts)
	return out
}

// rebuildAppTypesLocked recomputes the app-type index. A plugin claims
// its types when it is installed and has been granted apps:resolve;
// the panel-level Enabled flag is deliberately not consulted, since a
// workspace can reference an app type without the plugin's panel being
// open.
//
// Claims are resolved in a fixed order (local before global, then by
// id) so the outcome does not depend on directory iteration order, and
// the first claim wins. The caller must hold m.mu.
func (m *Manager) rebuildAppTypesLocked() {
	ids := make([]string, 0, len(m.loaded))
	for id := range m.loaded {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		li, lj := m.loaded[ids[i]], m.loaded[ids[j]]
		if li.Source != lj.Source {
			// A project-local plugin is the more specific choice.
			return li.Source == SourceLocal
		}
		return ids[i] < ids[j]
	})

	m.appTypes = map[string]string{}
	m.appTypeConflicts = nil
	for _, id := range ids {
		l := m.loaded[id]
		if l.Manifest == nil || len(l.Manifest.AppTypes) == 0 {
			continue
		}
		if !HasCapability(l.GrantedCaps, CapAppsResolve) {
			continue
		}
		for _, t := range l.Manifest.AppTypes {
			if winner, taken := m.appTypes[t]; taken {
				m.appTypeConflicts = append(m.appTypeConflicts, AppTypeConflict{
					Type: t, Winner: winner, Loser: id,
				})
				continue
			}
			m.appTypes[t] = id
		}
	}
}

// dropResolverLocked discards a plugin's cached resolver so the next
// resolve reloads it. Called when the plugin's code or grants change.
func (m *Manager) dropResolverLocked(id string) {
	if rt, ok := m.resolvers[id]; ok {
		_ = rt.Close()
		delete(m.resolvers, id)
	}
}
