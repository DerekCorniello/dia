package wailsapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/DerekCorniello/dia/internal/state"
)

// withTempXDG sets XDG_CONFIG_HOME and XDG_STATE_HOME to t.TempDir
// for the duration of the test so config/state resolve to a clean
// scratch space.
func withTempXDG(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("XDG_STATE_HOME", tmp)
}

// TestNewWorkspace_WritesFile verifies NewWorkspace creates a
// starter YAML in the global config dir.
func TestNewWorkspace_WritesFile(t *testing.T) {
	withTempXDG(t)
	a := New()
	path, err := a.NewWorkspace("demo", false)
	if err != nil {
		t.Fatalf("NewWorkspace: %v", err)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("path = %q, want absolute", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !contains(body, "name: demo") {
		t.Errorf("file missing 'name: demo':\n%s", body)
	}
	if !contains(body, "version: 1") {
		t.Errorf("file missing 'version: 1':\n%s", body)
	}
}

// TestNewWorkspace_RefusesExisting verifies a second call with the
// same name returns an error and does not overwrite.
func TestNewWorkspace_RefusesExisting(t *testing.T) {
	withTempXDG(t)
	a := New()
	if _, err := a.NewWorkspace("dup", false); err != nil {
		t.Fatalf("first NewWorkspace: %v", err)
	}
	if _, err := a.NewWorkspace("dup", false); err == nil {
		t.Fatal("expected error on duplicate name, got nil")
	}
}

// TestNewWorkspace_RejectsBadName verifies the validator runs.
func TestNewWorkspace_RejectsBadName(t *testing.T) {
	withTempXDG(t)
	a := New()
	for _, bad := range []string{"", "Has-Caps", "-leading", "trailing-", "has space", "with/slash"} {
		if _, err := a.NewWorkspace(bad, false); err == nil {
			t.Errorf("NewWorkspace(%q) = nil err, want error", bad)
		}
	}
}

// TestStartup_BuildsRuntime verifies Startup wires up the runtime
// without panicking. The runtime is reachable via ListInstances,
// which is a no-op before any workspace has been started.
func TestStartup_BuildsRuntime(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	if a.rt == nil {
		t.Fatal("rt not set after Startup")
	}
	if got := a.ListInstances(); len(got) != 0 {
		t.Errorf("ListInstances = %d, want 0", len(got))
	}
}

// TestSetCustomTheme_RoundTrip verifies a custom theme is persisted
// in the state store and shows up in ListCustomThemes.
func TestSetCustomTheme_RoundTrip(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())

	info := CustomThemeInfo{
		Name:        "midnight-rose",
		ColorScheme: "dark",
		Colors: map[string]string{
			"primary":      "#bd93f9",
			"secondary":    "#ff79c6",
			"accent":       "#50fa7b",
			"base_100":     "#1a1b26",
			"base_200":     "#16161e",
			"base_300":     "#101014",
			"base_content": "#c0caf5",
		},
	}
	if err := a.SetCustomTheme(info); err != nil {
		t.Fatalf("SetCustomTheme: %v", err)
	}

	list := a.ListCustomThemes()
	if len(list) != 1 {
		t.Fatalf("ListCustomThemes len = %d, want 1", len(list))
	}
	if list[0].Name != info.Name {
		t.Errorf("Name = %q, want %q", list[0].Name, info.Name)
	}
	if list[0].Colors["primary"] != "#bd93f9" {
		t.Errorf("Colors[primary] = %q, want #bd93f9", list[0].Colors["primary"])
	}
	if list[0].ColorScheme != "dark" {
		t.Errorf("ColorScheme = %q, want dark", list[0].ColorScheme)
	}
}

// TestSetCustomTheme_RejectsBadName verifies the name validator
// catches unsafe names.
func TestSetCustomTheme_RejectsBadName(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	for _, bad := range []string{"", "has space", "with/slash", "very-long-name-that-is-clearly-beyond-the-sixty-four-character-limit-and-should-fail"} {
		if err := a.SetCustomTheme(CustomThemeInfo{
			Name:        bad,
			ColorScheme: "dark",
			Colors:      map[string]string{"primary": "#000000"},
		}); err == nil {
			t.Errorf("SetCustomTheme(%q) = nil err, want error", bad)
		}
	}
}

// TestSetCustomTheme_RejectsBadColorScheme verifies only the two
// daisyUI color schemes are accepted.
func TestSetCustomTheme_RejectsBadColorScheme(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	for _, bad := range []string{"", "auto", "system", "DARK"} {
		if err := a.SetCustomTheme(CustomThemeInfo{
			Name:        "ok",
			ColorScheme: bad,
			Colors:      map[string]string{"primary": "#000000"},
		}); err == nil {
			t.Errorf("SetCustomTheme(color_scheme=%q) = nil err, want error", bad)
		}
	}
}

// TestSetCustomTheme_RejectsBadHexAndSlot verifies malformed hex
// values and unknown color slots are rejected before persistence.
func TestSetCustomTheme_RejectsBadHexAndSlot(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	if err := a.SetCustomTheme(CustomThemeInfo{
		Name:        "ok",
		ColorScheme: "dark",
		Colors:      map[string]string{"primary": "not-a-hex"},
	}); err == nil {
		t.Error("expected error on bad hex")
	}
	if err := a.SetCustomTheme(CustomThemeInfo{
		Name:        "ok",
		ColorScheme: "dark",
		Colors:      map[string]string{"made_up_slot": "#000000"},
	}); err == nil {
		t.Error("expected error on unknown slot")
	}
	if err := a.SetCustomTheme(CustomThemeInfo{
		Name:        "ok",
		ColorScheme: "dark",
		Colors:      map[string]string{},
	}); err == nil {
		t.Error("expected error on empty colors")
	}
}

// TestDeleteCustomTheme verifies deletion works and a missing name
// is a no-op (not an error).
func TestDeleteCustomTheme(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	if err := a.SetCustomTheme(CustomThemeInfo{
		Name:        "x",
		ColorScheme: "dark",
		Colors:      map[string]string{"primary": "#000000"},
	}); err != nil {
		t.Fatalf("SetCustomTheme: %v", err)
	}
	if err := a.DeleteCustomTheme("x"); err != nil {
		t.Fatalf("DeleteCustomTheme: %v", err)
	}
	if got := a.ListCustomThemes(); len(got) != 0 {
		t.Errorf("after delete, len = %d, want 0", len(got))
	}
	// Deleting a non-existent name should be a no-op, not an error.
	if err := a.DeleteCustomTheme("nope"); err != nil {
		t.Errorf("DeleteCustomTheme missing: %v", err)
	}
}

func contains(haystack []byte, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(haystack []byte, needle string) int {
	n := len(needle)
	for i := 0; i+n <= len(haystack); i++ {
		if string(haystack[i:i+n]) == needle {
			return i
		}
	}
	return -1
}

func TestStartup_DiscoversPlugins(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	if a.pmgr == nil {
		t.Fatal("pmgr not initialized after Startup")
	}
	if a.pmgr.GlobalDir() == "" {
		t.Error("global plugins dir not set")
	}
	all := a.ListPlugins()
	if len(all) != 0 {
		t.Errorf("expected 0 plugins in fresh state, got %d", len(all))
	}
	if paths := a.PluginPaths(); paths.GlobalDir == "" {
		t.Error("PluginPaths did not return global dir")
	}
}

func TestSetPluginEnabled_RoundTrip(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	stateDir, err := resolveStateDir(t)
	if err != nil {
		t.Fatal(err)
	}
	pdir := filepath.Join(stateDir, "plugins", "demo")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"),
		[]byte(`{"id":"demo","name":"Demo","version":"0.1.0","capabilities":["workspaces:read"],"ui":{"type":"list","title":"T"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	js := "module.exports = { ping: function() { return 'pong'; } };"
	if err := os.WriteFile(filepath.Join(pdir, "index.js"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.pmgr.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.SetPluginEnabled("demo", true, []string{"workspaces:read"}); err != nil {
		t.Fatalf("SetPluginEnabled: %v", err)
	}
	all := a.ListPlugins()
	if len(all) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(all))
	}
	if !all[0].Enabled {
		t.Error("plugin should be enabled")
	}
	out, err := a.PluginCall("demo", "ping", "[]")
	if err != nil {
		t.Fatalf("PluginCall: %v", err)
	}
	var v any
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v != "pong" {
		t.Errorf("ping = %v, want pong", v)
	}
	if err := a.SetPluginEnabled("demo", false, nil); err != nil {
		t.Fatalf("disable: %v", err)
	}
	all = a.ListPlugins()
	if all[0].Enabled {
		t.Error("plugin should be disabled")
	}
}

func TestPluginCall_FailsForNotEnabled(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	stateDir, err := resolveStateDir(t)
	if err != nil {
		t.Fatal(err)
	}
	pdir := filepath.Join(stateDir, "plugins", "x")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"),
		[]byte(`{"id":"x","name":"X","version":"0.1.0","ui":{"type":"list","title":"T"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.pmgr.Discover(); err != nil {
		t.Fatal(err)
	}
	if _, err := a.PluginCall("x", "ping", "[]"); err == nil {
		t.Error("expected error calling disabled plugin")
	}
}

// TestListPlugins_NoActionsMarshalsAsArray guards against a regression
// where a plugin with no UI.Actions produced "actions": null in JSON,
// which broke `{#each plugin.actions}` in the Svelte template. Same
// guard for grantedCapabilities when the plugin is freshly discovered
// (no enable round-trip).
func TestListPlugins_NoActionsMarshalsAsArray(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	stateDir, err := resolveStateDir(t)
	if err != nil {
		t.Fatal(err)
	}
	pdir := filepath.Join(stateDir, "plugins", "bare")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"),
		[]byte(`{"id":"bare","name":"Bare","version":"0.1.0","ui":{"type":"window","title":"B","entry":"panel.js","width":1,"height":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "index.js"), []byte("module.exports = {};"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.pmgr.Discover(); err != nil {
		t.Fatal(err)
	}
	all := a.ListPlugins()
	if len(all) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(all))
	}
	raw, err := json.Marshal(all[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("unmarshal into probe: %v", err)
	}
	actionsRaw, ok := probe["actions"]
	if !ok {
		t.Fatalf("`actions` field missing in JSON: %s", string(raw))
	}
	var actions []any
	if err := json.Unmarshal(actionsRaw, &actions); err != nil {
		t.Fatalf("`actions` is not a JSON array: %s (got %s)", err, string(actionsRaw))
	}
	if len(actions) != 0 {
		t.Errorf("`actions` should be empty array, got %d items", len(actions))
	}
	if probe["grantedCapabilities"] == nil {
		t.Errorf("`grantedCapabilities` should be present (possibly empty), got null/missing")
	}
}

func resolveStateDir(t *testing.T) (string, error) {
	t.Helper()
	return filepath.Join(os.Getenv("XDG_STATE_HOME"), "dia"), nil
}

func TestEnableWorkspacePlugin_RoundTrip(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	stateDir, err := resolveStateDir(t)
	if err != nil {
		t.Fatal(err)
	}
	pdir := filepath.Join(stateDir, "plugins", "wspl")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"),
		[]byte(`{"id":"wspl","name":"Wspl","version":"0.1.0","capabilities":["workspaces:read"],"ui":{"type":"list","title":"T"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "index.js"), []byte("module.exports = {};"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.pmgr.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.enableWorkspacePlugin("wspl", nil); err != nil {
		t.Fatalf("enableWorkspacePlugin: %v", err)
	}
	snap := a.store.Snapshot()
	ps, ok := snap.Plugins["wspl"]
	if !ok {
		t.Fatal("plugin state not persisted")
	}
	if !ps.Enabled {
		t.Error("plugin should be enabled")
	}
	if len(ps.GrantedCapabilities) != 1 || ps.GrantedCapabilities[0] != "workspaces:read" {
		t.Errorf("granted caps: %v", ps.GrantedCapabilities)
	}
	loaded, ok := a.pmgr.Loaded("wspl")
	if !ok {
		t.Fatal("plugin not loaded")
	}
	if !loaded.Enabled {
		t.Error("plugin should be enabled in manager")
	}
}

func TestStopInstance_DisablesWorkspacePlugins(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	stateDir, err := resolveStateDir(t)
	if err != nil {
		t.Fatal(err)
	}
	pdir := filepath.Join(stateDir, "plugins", "stp")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"),
		[]byte(`{"id":"stp","name":"Stp","version":"0.1.0","capabilities":["workspaces:read"],"ui":{"type":"list","title":"T"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "index.js"), []byte("module.exports = {};"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.pmgr.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := a.SetPluginEnabled("stp", true, []string{"workspaces:read"}); err != nil {
		t.Fatal(err)
	}
	instID := "fake-inst"
	_ = a.store.Mutate(func(d *state.Data) {
		d.Instances[instID] = state.Instance{
			ID:      instID,
			Plugins: []string{"stp"},
			Status:  state.StatusStopped,
			Apps:    []state.AppProcess{},
		}
	})
	_ = a.StopInstance(instID)
	snap := a.store.Snapshot()
	ps := snap.Plugins["stp"]
	if ps.Enabled {
		t.Error("plugin should be disabled after StopInstance")
	}
}
