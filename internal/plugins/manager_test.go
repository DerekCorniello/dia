package plugins

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type fakeHost struct {
	mu          sync.Mutex
	workspaces  []any
	instances   []any
	stopped     []string
	stoppedAll  int
	started     []string
	doctorOut   []any
	pathsOut    any
	theme       string
	customOut   []any
	newWspath   string
	stopErr     error
	startErr    error
	setThemeErr error
}

func (f *fakeHost) ListWorkspaces(ctx context.Context) ([]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]any, len(f.workspaces))
	copy(out, f.workspaces)
	return out, nil
}
func (f *fakeHost) GetWorkspace(ctx context.Context, name string) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, w := range f.workspaces {
		if m, ok := w.(map[string]any); ok {
			if m["name"] == name {
				return m, nil
			}
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeHost) StartWorkspace(ctx context.Context, name string) (any, error) {
	if f.startErr != nil {
		return nil, f.startErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, name)
	return map[string]any{"id": "i1", "workspace": name, "status": "running"}, nil
}
func (f *fakeHost) ListInstances(ctx context.Context) ([]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]any, len(f.instances))
	copy(out, f.instances)
	return out, nil
}
func (f *fakeHost) StopInstance(ctx context.Context, id string) error {
	if f.stopErr != nil {
		return f.stopErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, id)
	return nil
}
func (f *fakeHost) StopAll(ctx context.Context) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stoppedAll++
	return f.stoppedAll, nil
}
func (f *fakeHost) Doctor(ctx context.Context) ([]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.doctorOut, nil
}
func (f *fakeHost) Paths(ctx context.Context) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.pathsOut, nil
}
func (f *fakeHost) GetTheme(ctx context.Context) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.theme, nil
}
func (f *fakeHost) SetTheme(ctx context.Context, name string) error {
	if f.setThemeErr != nil {
		return f.setThemeErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.theme = name
	return nil
}
func (f *fakeHost) ListCustomThemes(ctx context.Context) ([]any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.customOut, nil
}
func (f *fakeHost) SetCustomTheme(ctx context.Context, info any) error       { return nil }
func (f *fakeHost) DeleteCustomTheme(ctx context.Context, name string) error { return nil }
func (f *fakeHost) NewWorkspace(ctx context.Context, name string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.newWspath = filepath.Join("/tmp", name+".yaml")
	return f.newWspath, nil
}
func setupPlugin(t *testing.T, host HostAPI, id, js string, caps []string) (string, *Manager) {
	t.Helper()
	dir := t.TempDir()
	pdir := filepath.Join(dir, id)
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	mf := Manifest{ID: id, Name: "Test", Version: "0.1.0", Entry: "index.js", Capabilities: caps, UI: UISpec{Type: "list", Title: "T"}}
	mdata := []byte(`{"id":"` + mf.ID + `","name":"` + mf.Name + `","version":"` + mf.Version + `","entry":"index.js","capabilities":` + jsonCaps(caps) + `,"ui":{"type":"list","title":"T"}}`)
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"), mdata, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "index.js"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(dir, host)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}
	return pdir, mgr
}
func jsonCaps(caps []string) string {
	if len(caps) == 0 {
		return "[]"
	}
	out := "["
	for i, c := range caps {
		if i > 0 {
			out += ","
		}
		out += `"` + c + `"`
	}
	return out + "]"
}
func TestManagerDiscover(t *testing.T) {
	host := &fakeHost{workspaces: []any{map[string]any{"name": "alpha"}}}
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "hello"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello", "plugin.json"), []byte(`{"id":"hello","name":"Hi","version":"0.1.0","ui":{"type":"list","title":"T"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(dir, host)
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}
	all := mgr.List()
	if len(all) != 1 {
		t.Fatalf("want 1 plugin, got %d", len(all))
	}
	if all[0].Manifest.ID != "hello" {
		t.Errorf("got id %q", all[0].Manifest.ID)
	}
}
func TestManagerEnableDisable(t *testing.T) {
	host := &fakeHost{}
	js := `module.exports = { ping: function() { return "pong" } };`
	_, mgr := setupPlugin(t, host, "hello", js, nil)
	if err := mgr.Enable("hello"); err != nil {
		t.Fatal(err)
	}
	v, err := mgr.Call("hello", "ping", nil)
	if err != nil {
		t.Fatal(err)
	}
	if v != "pong" {
		t.Errorf("got %v", v)
	}
	if err := mgr.Disable("hello"); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Call("hello", "ping", nil); err == nil {
		t.Errorf("expected error after disable")
	}
}
func TestPluginReadsData(t *testing.T) {
	host := &fakeHost{workspaces: []any{map[string]any{"name": "alpha"}, map[string]any{"name": "beta"}}}
	js := `module.exports = { getData: async function() { return await dia.listWorkspaces(); } };`
	_, mgr := setupPlugin(t, host, "hello", js, []string{CapWorkspacesRead})
	if err := mgr.Enable("hello"); err != nil {
		t.Fatal(err)
	}
	v, err := mgr.Call("hello", "getData", nil)
	if err != nil {
		t.Fatal(err)
	}
	items, ok := v.([]any)
	if !ok {
		t.Fatalf("expected array, got %T", v)
	}
	if len(items) != 2 {
		t.Errorf("want 2, got %d", len(items))
	}
}
func TestPluginCannotCallForbidden(t *testing.T) {
	host := &fakeHost{}
	js := `module.exports = { getData: async function() { return await dia.startWorkspace("alpha"); } };`
	_, mgr := setupPlugin(t, host, "hello", js, []string{CapWorkspacesRead})
	if err := mgr.Enable("hello"); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Call("hello", "getData", nil); err == nil {
		t.Errorf("expected error for ungranted capability")
	}
}
func TestPluginCanCallGranted(t *testing.T) {
	host := &fakeHost{}
	js := `module.exports = { go: async function(name) { return await dia.startWorkspace(name); } };`
	_, mgr := setupPlugin(t, host, "hello", js, []string{CapWorkspacesRead, CapWorkspacesStart})
	if err := mgr.Enable("hello"); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Call("hello", "go", []any{"alpha"}); err != nil {
		t.Fatal(err)
	}
	if len(host.started) != 1 || host.started[0] != "alpha" {
		t.Errorf("host did not receive start call: %v", host.started)
	}
}
func TestPluginErrorCaptured(t *testing.T) {
	host := &fakeHost{}
	js := `throw new Error("init failed"); module.exports = {};`
	_, mgr := setupPlugin(t, host, "hello", js, nil)
	if err := mgr.Enable("hello"); err == nil {
		t.Errorf("expected error from bad plugin")
	}
	l, _ := mgr.Get("hello")
	if l.Status != StatusErrored {
		t.Errorf("want errored status, got %q", l.Status)
	}
	if l.LastError == "" {
		t.Error("expected last error to be set")
	}
}
func TestPluginUninstall(t *testing.T) {
	host := &fakeHost{}
	js := `module.exports = {};`
	_, mgr := setupPlugin(t, host, "hello", js, nil)
	if err := mgr.Enable("hello"); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Uninstall("hello"); err != nil {
		t.Fatal(err)
	}
	if _, ok := mgr.Get("hello"); ok {
		t.Error("plugin should be gone after uninstall")
	}
}

func TestLocalPluginDiscovery(t *testing.T) {
	host := &fakeHost{}
	cwd := t.TempDir()
	localDir := LocalPluginsDir(cwd)
	if err := os.MkdirAll(filepath.Join(localDir, "local-only"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "local-only", "plugin.json"),
		[]byte(`{"id":"local-only","name":"L","version":"0.1.0","ui":{"type":"list","title":"T"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	globalDir := t.TempDir()
	mgr, err := NewManager(globalDir, host)
	if err != nil {
		t.Fatal(err)
	}
	mgr.SetLocalDir(cwd)
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}
	all := mgr.List()
	if len(all) != 1 {
		t.Fatalf("want 1, got %d", len(all))
	}
	if all[0].Source != SourceLocal {
		t.Errorf("expected source=local, got %q", all[0].Source)
	}
	if mgr.LocalDir() == "" {
		t.Error("expected local dir to be set")
	}
}

func TestLocalOverridesGlobal(t *testing.T) {
	host := &fakeHost{}
	cwd := t.TempDir()
	globalDir := t.TempDir()
	for _, side := range []struct {
		base   string
		ver    string
		source Source
	}{
		{globalDir, "1.0.0", SourceGlobal},
		{filepath.Join(cwd, ".dia", "plugins"), "2.0.0", SourceLocal},
	} {
		if err := os.MkdirAll(filepath.Join(side.base, "shared"), 0o755); err != nil {
			t.Fatal(err)
		}
		body := `{"id":"shared","name":"S","version":"` + side.ver + `","ui":{"type":"list","title":"T"}}`
		if err := os.WriteFile(filepath.Join(side.base, "shared", "plugin.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mgr, err := NewManager(globalDir, host)
	if err != nil {
		t.Fatal(err)
	}
	mgr.SetLocalDir(cwd)
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}
	all := mgr.List()
	if len(all) != 1 {
		t.Fatalf("want 1 (local wins), got %d", len(all))
	}
	if all[0].Source != SourceLocal {
		t.Errorf("expected local to win, got source=%q", all[0].Source)
	}
	if all[0].Manifest.Version != "2.0.0" {
		t.Errorf("expected local version 2.0.0, got %q", all[0].Manifest.Version)
	}
}

func TestLocalInstall(t *testing.T) {
	host := &fakeHost{}
	cwd := t.TempDir()
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "plugin.json"),
		[]byte(`{"id":"demo","name":"D","version":"0.1.0","ui":{"type":"list","title":"T"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(t.TempDir(), host)
	if err != nil {
		t.Fatal(err)
	}
	mgr.SetLocalDir(cwd)
	dst, err := mgr.InstallLocal(src, cwd)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dst, LocalPluginsDir(cwd)) {
		t.Errorf("expected install under .dia/plugins, got %s", dst)
	}
}

func TestGlobalAndLocalCoexist(t *testing.T) {
	host := &fakeHost{}
	cwd := t.TempDir()
	globalDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(globalDir, "glob"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "glob", "plugin.json"),
		[]byte(`{"id":"glob","name":"G","version":"0.1.0","ui":{"type":"list","title":"T"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(LocalPluginsDir(cwd)+"/loc", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(LocalPluginsDir(cwd)+"/loc/plugin.json",
		[]byte(`{"id":"loc","name":"L","version":"0.1.0","ui":{"type":"list","title":"T"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(globalDir, host)
	if err != nil {
		t.Fatal(err)
	}
	mgr.SetLocalDir(cwd)
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}
	all := mgr.List()
	if len(all) != 2 {
		t.Fatalf("want 2 (glob + loc), got %d", len(all))
	}
	sources := map[string]Source{}
	for _, l := range all {
		sources[l.Manifest.ID] = l.Source
	}
	if sources["glob"] != SourceGlobal {
		t.Errorf("glob should be global")
	}
	if sources["loc"] != SourceLocal {
		t.Errorf("loc should be local")
	}
}

func TestEnableWithGrants(t *testing.T) {
	host := &fakeHost{}
	js := "module.exports = { ping: function() { return 'pong'; } };"
	_, mgr := setupPlugin(t, host, "demo", js, []string{CapWorkspacesRead, CapWorkspacesStart})
	grants := []string{CapWorkspacesRead}
	if err := mgr.EnableWithGrants("demo", grants); err != nil {
		t.Fatal(err)
	}
	loaded, _ := mgr.Loaded("demo")
	if !loaded.Enabled {
		t.Fatal("should be enabled")
	}
	if len(loaded.GrantedCaps) != 1 || loaded.GrantedCaps[0] != CapWorkspacesRead {
		t.Errorf("grants not stored: %v", loaded.GrantedCaps)
	}
	rt, ok := mgr.runtimes["demo"]
	if !ok {
		t.Fatal("runtime not stored")
	}
	if !rt.bridge.has(CapWorkspacesRead) {
		t.Error("bridge should have workspaces:read")
	}
	if rt.bridge.has(CapWorkspacesStart) {
		t.Error("bridge should NOT have workspaces:start")
	}
}
