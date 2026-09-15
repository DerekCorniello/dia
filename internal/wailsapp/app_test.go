package wailsapp

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/daemon"
)

// withTempXDG sets XDG_CONFIG_HOME and XDG_STATE_HOME to a temp dir
// for the duration of the test so config/state resolve to a clean
// scratch space. The dir lives directly under /tmp (except Windows)
// because tests that host a daemon bind serve.sock inside it, and
// t.TempDir on macOS nests under /var/folders with names long enough
// to blow past the 104-byte sun_path limit.
func withTempXDG(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	if runtime.GOOS != "windows" {
		short, err := os.MkdirTemp("/tmp", "dia-xdg")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(short) })
		tmp = short
	}
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("XDG_STATE_HOME", tmp)
}

func TestWorkspaceEditorPreservesBrowserAndCommandFields(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	editor := WorkspaceEditor{
		Name:  "roundtrip",
		Hooks: &HooksEditor{PreStart: []string{"echo prepare"}, PostStop: []string{"echo cleanup"}},
		Apps: []AppEditor{
			{
				Type: "browser", Label: "dashboards", Browser: "zen-browser",
				Urls: []string{"dc/gh/mux/prs", "https://example.com"}, NewWindow: true,
				Env: map[string]string{"MOZ_ENABLE_WAYLAND": "1"},
			},
			{
				Type: "editor", Label: "code", Cmd: "code", Cwd: "/tmp/project",
				Args: []string{"--wait", "."}, Env: map[string]string{"A": "B"},
			},
		},
	}
	if err := a.SaveWorkspaceEditor(editor); err != nil {
		t.Fatalf("SaveWorkspaceEditor: %v", err)
	}
	path := filepath.Join(config.DefaultGlobalDir(), "roundtrip.yaml")
	ws, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load saved workspace: %v", err)
	}
	if got := ws.Apps[0]; got.Browser != "zen-browser" || !got.NewWindow || !slices.Equal(got.Urls, editor.Apps[0].Urls) || got.Env["MOZ_ENABLE_WAYLAND"] != "1" {
		t.Errorf("browser app did not round-trip: %#v", got)
	}
	if got := ws.Apps[1]; got.Type != "editor" || got.Cmd != "code" || !slices.Equal(got.Args, []string{"--wait", "."}) || got.Env["A"] != "B" {
		t.Errorf("command app did not round-trip: %#v", got)
	}
	if ws.Hooks == nil || !slices.Equal(ws.Hooks.PreStart, []string{"echo prepare"}) || !slices.Equal(ws.Hooks.PostStop, []string{"echo cleanup"}) {
		t.Errorf("hooks did not round-trip: %#v", ws.Hooks)
	}
	editor.Description = "updated"
	if err := a.SaveWorkspaceEditor(editor); err != nil {
		t.Fatalf("SaveWorkspaceEditor update: %v", err)
	}
	backup, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatalf("read workspace backup: %v", err)
	}
	if !contains(backup, "name: roundtrip") || contains(backup, "description: updated") {
		t.Fatalf("workspace backup does not contain the previous version:\n%s", backup)
	}
}

// TestEditorRoundTripAllTypes is the lossless-editor fixture table: every
// built-in type plus hooks, env, and plugin types must survive
// Save -> GetWorkspaceEditor -> Save without dropping representable data.
func TestEditorRoundTripAllTypes(t *testing.T) {
	withTempXDG(t)
	startDaemon(t, xdgStateDir(t))
	a := New()
	a.Startup(testCtx())
	cases := []AppEditor{
		{Type: "terminal", Label: "shell", Cmd: "kitty", Cwd: "/tmp", Args: []string{"--hold"}, Env: map[string]string{"A": "1"}, TermCmd: "claude"},
		{Type: "editor", Label: "code", Cmd: "code", Args: []string{"--wait", "."}},
		{Type: "service", Label: "db", Cmd: "postgres", Args: []string{"-D", "/data"}},
		{Type: "browser", Label: "dash", Browser: "zen-browser", Urls: []string{"dc/gh/mux/prs", "https://example.com"}, NewWindow: true},
		{Type: "browser", Label: "single", Url: "https://example.com"},
		{Type: "open", Label: "docs", Url: "https://example.com/docs"},
		{Type: "local", Label: "run", Cmd: "make", Args: []string{"test"}},
		{Type: "custom", Label: "thing", Cmd: "thing", Env: map[string]string{"X": "y"}},
		{Type: "ai", Label: "agent", Cmd: "agent", Args: []string{"--model", "m"}},
		{Type: "gh", Label: "prs", Cmd: "pr", Args: []string{"list"}},
		{Type: "gh:pr", Label: "pr", Args: []string{"view", "12"}},
		{Type: "gh:issue", Label: "iss", Args: []string{"list"}},
		{Type: "gh:checkout", Label: "co", Args: []string{"12"}},
		{Type: "gh:repo-clone", Label: "clone", Url: "https://github.com/o/r", Cwd: "/tmp/dst"},
		{Type: "my-plugin-type", Label: "plug", Cmd: "plug-cmd", Args: []string{"--x"}},
	}
	editor := WorkspaceEditor{
		Name:        "alltypes",
		Description: "every type",
		Hooks:       &HooksEditor{PreStart: []string{"echo a"}, PostStart: []string{"echo b"}, PreStop: []string{"echo c"}, PostStop: []string{"echo d"}},
		Apps:        cases,
	}
	if err := a.SaveWorkspaceEditor(editor); err != nil {
		t.Fatalf("SaveWorkspaceEditor: %v", err)
	}
	path := filepath.Join(config.DefaultGlobalDir(), "alltypes.yaml")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.GetWorkspaceEditor("alltypes")
	if err != nil {
		t.Fatalf("GetWorkspaceEditor: %v", err)
	}
	if len(got.Apps) != len(cases) {
		t.Fatalf("apps = %d, want %d", len(got.Apps), len(cases))
	}
	// Saving the reloaded editor must be a fixed point: identical bytes.
	got.OriginalName = "alltypes"
	got.OriginalPath = path
	if err := a.SaveWorkspaceEditor(*got); err != nil {
		t.Fatalf("resave: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("editor is not a fixed point:\n--- first ---\n%s\n--- second ---\n%s", before, after)
	}
	ws, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(ws.Apps) != len(cases) {
		t.Fatalf("reloaded apps = %d, want %d", len(ws.Apps), len(cases))
	}
	for i, want := range cases {
		gotApp := ws.Apps[i]
		if gotApp.Type != want.Type || gotApp.Label != want.Label {
			t.Errorf("app[%d] type/label = %q/%q, want %q/%q", i, gotApp.Type, gotApp.Label, want.Type, want.Label)
		}
		if !reflect.DeepEqual(gotApp.Env, emptyToNil(want.Env)) {
			t.Errorf("app[%d] env = %#v, want %#v", i, gotApp.Env, want.Env)
		}
	}
}

func emptyToNil(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return m
}

// xdgStateDir returns the state dir the app binds under the current
// XDG_STATE_HOME, matching state.ResolveStateDir.
func xdgStateDir(t *testing.T) string {
	t.Helper()
	dir, err := resolveStateDir(t)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// startDaemon boots an in-process daemon server on the given state dir
// so the app under test dials it instead of spawning the (test)
// binary. Cleanup shuts it down.
func startDaemon(t *testing.T, stateDir string) {
	t.Helper()
	srv, err := daemon.NewServer(daemon.Options{
		StateDir: stateDir,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = srv.Serve()
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		c, err := daemon.Dial(stateDir)
		if err == nil {
			_ = c.Close()
			break
		}
		if time.Now().After(deadline) {
			srv.Close()
			<-done
			t.Fatalf("daemon socket never came up: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() {
		srv.Close()
		<-done
	})
}

// TestNewWorkspace_WritesFile verifies NewWorkspace creates a
// starter YAML in the global config dir.
func TestNewWorkspace_WritesFile(t *testing.T) {
	withTempXDG(t)
	a := New()
	path, err := a.NewWorkspace("demo", "")
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
	if _, err := a.NewWorkspace("dup", ""); err != nil {
		t.Fatalf("first NewWorkspace: %v", err)
	}
	if _, err := a.NewWorkspace("dup", ""); err == nil {
		t.Fatal("expected error on duplicate name, got nil")
	}
}

// TestNewWorkspace_RejectsBadName verifies the validator runs.
func TestNewWorkspace_RejectsBadName(t *testing.T) {
	withTempXDG(t)
	a := New()
	for _, bad := range []string{"", "has space", "with/slash"} {
		if _, err := a.NewWorkspace(bad, ""); err == nil {
			t.Errorf("NewWorkspace(%q) = nil err, want error", bad)
		}
	}
	// These should now be valid (uppercase, leading/trailing hyphens allowed)
	for _, good := range []string{"Has-Caps", "-leading", "trailing-"} {
		if _, err := a.NewWorkspace(good, ""); err != nil {
			t.Errorf("NewWorkspace(%q) = %v, want nil", good, err)
		}
	}
}

// TestStartup_BuildsRuntime verifies Startup wires up the plugin and
// state handles without panicking, and that lifecycle works against an
// in-process daemon on the resolved state dir (mirroring how the CLI
// tests host a server that dial, not spawn).
func TestStartup_BuildsRuntime(t *testing.T) {
	withTempXDG(t)
	stateDir := xdgStateDir(t)
	a := New()
	startDaemon(t, stateDir)
	a.Startup(testCtx())
	if a.store == nil {
		t.Fatal("store not set after Startup")
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

func TestGetPluginReturnsDetailOmittedFromList(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	stateDir, err := resolveStateDir(t)
	if err != nil {
		t.Fatal(err)
	}
	pdir := filepath.Join(stateDir, "plugins", "details")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"details","name":"Details","version":"0.1.0","long_description":"Long help","config_schema":{"color":{"type":"text"}},"ui":{"type":"table","title":"Rows","columns":[{"key":"name","label":"Name"}]}}`
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "index.js"), []byte("module.exports = {};"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.pmgr.Discover(); err != nil {
		t.Fatal(err)
	}

	list := a.ListPlugins()
	if len(list) != 1 {
		t.Fatalf("ListPlugins len = %d, want 1", len(list))
	}
	if list[0].LongDescription != "" || list[0].ConfigSchema != nil {
		t.Errorf("list contains detail fields: %#v", list[0])
	}
	if len(list[0].UI.Columns) != 1 {
		t.Fatalf("list omitted table rendering contract: %#v", list[0].UI)
	}
	detail, err := a.GetPlugin("details")
	if err != nil {
		t.Fatalf("GetPlugin: %v", err)
	}
	if detail.LongDescription != "Long help" || detail.ConfigSchema["color"] == nil {
		t.Errorf("detail fields missing: %#v", detail)
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
