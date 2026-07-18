package wailsapp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/plugins"
)

const capsManifest = `{"id":"caps-plug","name":"Caps","version":"0.1.0","entry":"index.js",` +
	`"capabilities":["workspaces:read","cmd:exec","apps:resolve"],"app_types":["capstype"],` +
	`"ui":{"type":"kv","title":"T"}}`

// writePlugin drops a plugin into the global plugins dir of the temp
// XDG state home, before the App starts.
func writePlugin(t *testing.T, manifest string) {
	t.Helper()
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		t.Fatal("XDG_STATE_HOME not set; call withTempXDG first")
	}
	dir := filepath.Join(stateHome, "dia", "plugins", "caps-plug")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	js := `module.exports = {
		getData: function () { return {}; },
		resolveApp: function (app) { return { cmd: "resolved-cmd", cwd: app.cwd }; }
	};`
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
}

func startedApp(t *testing.T) *App {
	t.Helper()
	a := New()
	a.Startup(testCtx())
	return a
}

// A freshly discovered plugin gets read-only capabilities, no matter
// what its manifest asks for.
func TestGetPluginCapabilities_DefaultsToReadOnly(t *testing.T) {
	withTempXDG(t)
	writePlugin(t, capsManifest)
	a := startedApp(t)

	info, err := a.GetPluginCapabilities("caps-plug")
	if err != nil {
		t.Fatalf("GetPluginCapabilities: %v", err)
	}
	if len(info.Requested) != 3 {
		t.Fatalf("Requested = %+v, want 3 entries", info.Requested)
	}
	for _, c := range info.Requested {
		switch c.Name {
		case "workspaces:read":
			if c.Mutating {
				t.Error("workspaces:read should not be mutating")
			}
			if !c.Granted {
				t.Error("read capability should be granted by default")
			}
		case "cmd:exec", "apps:resolve":
			if !c.Mutating {
				t.Errorf("%s should be mutating", c.Name)
			}
			if c.Granted {
				t.Errorf("%s must not be granted by default", c.Name)
			}
		default:
			t.Errorf("unexpected capability %q", c.Name)
		}
	}
}

// Granting apps:resolve has to take effect immediately: the app type
// must become resolvable without restarting dia.
func TestSetPluginCapabilities_RegistersAppTypeImmediately(t *testing.T) {
	withTempXDG(t)
	writePlugin(t, capsManifest)
	a := startedApp(t)

	if _, err := a.reg.Resolve(config.App{Type: "capstype"}); err == nil {
		t.Fatal("app type should not resolve before apps:resolve is granted")
	}

	if err := a.SetPluginCapabilities("caps-plug", []string{"workspaces:read", "apps:resolve"}); err != nil {
		t.Fatalf("SetPluginCapabilities: %v", err)
	}

	action, err := a.reg.Resolve(config.App{Type: "capstype", Cwd: "/w"})
	if err != nil {
		t.Fatalf("app type should resolve after the grant: %v", err)
	}
	if action.Launch == nil || action.Launch.Cmd != "resolved-cmd" {
		t.Errorf("unexpected action: %+v", action)
	}
}

// Revoking has to take effect immediately too, or a withdrawn grant
// would keep working until the next restart.
func TestSetPluginCapabilities_RevokeUnregistersAppType(t *testing.T) {
	withTempXDG(t)
	writePlugin(t, capsManifest)
	a := startedApp(t)

	if err := a.SetPluginCapabilities("caps-plug", []string{"apps:resolve"}); err != nil {
		t.Fatalf("grant: %v", err)
	}
	if _, err := a.reg.Resolve(config.App{Type: "capstype"}); err != nil {
		t.Fatalf("expected the type to resolve after granting: %v", err)
	}

	if err := a.SetPluginCapabilities("caps-plug", []string{"workspaces:read"}); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := a.reg.Resolve(config.App{Type: "capstype"}); err == nil {
		t.Error("app type should stop resolving once apps:resolve is revoked")
	}
}

func TestSetPluginCapabilities_DropsUnrequestedCapabilities(t *testing.T) {
	withTempXDG(t)
	writePlugin(t, capsManifest)
	a := startedApp(t)

	// themes:write is not in the manifest.
	if err := a.SetPluginCapabilities("caps-plug", []string{"cmd:exec", "themes:write"}); err != nil {
		t.Fatalf("SetPluginCapabilities: %v", err)
	}
	info, err := a.GetPluginCapabilities("caps-plug")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range info.Granted {
		if c == "themes:write" {
			t.Error("a capability the manifest never requested must be dropped")
		}
	}
	if !plugins.HasCapability(info.Granted, "cmd:exec") {
		t.Errorf("cmd:exec should be granted, got %v", info.Granted)
	}
}

// The grant has to survive a restart, which is the whole point of
// persisting it. applyPersistedPluginState previously ignored the
// store and reset everything to read-only on every launch.
func TestPluginGrantsSurviveRestart(t *testing.T) {
	withTempXDG(t)
	writePlugin(t, capsManifest)

	a := startedApp(t)
	if err := a.SetPluginCapabilities("caps-plug", []string{"cmd:exec", "apps:resolve"}); err != nil {
		t.Fatalf("SetPluginCapabilities: %v", err)
	}

	// Same XDG dirs, fresh App: this is a restart.
	b := startedApp(t)
	info, err := b.GetPluginCapabilities("caps-plug")
	if err != nil {
		t.Fatalf("GetPluginCapabilities after restart: %v", err)
	}
	if !plugins.HasCapability(info.Granted, "cmd:exec") {
		t.Errorf("cmd:exec did not survive the restart, got %v", info.Granted)
	}
	if !plugins.HasCapability(info.Granted, "apps:resolve") {
		t.Errorf("apps:resolve did not survive the restart, got %v", info.Granted)
	}
	if _, err := b.reg.Resolve(config.App{Type: "capstype"}); err != nil {
		t.Errorf("the app type should be registered on a restart: %v", err)
	}
}

func TestGetPluginCapabilities_UnknownPlugin(t *testing.T) {
	withTempXDG(t)
	a := startedApp(t)
	if _, err := a.GetPluginCapabilities("nope"); err == nil {
		t.Error("expected an error for an unknown plugin")
	}
	if err := a.SetPluginCapabilities("nope", nil); err == nil {
		t.Error("expected an error for an unknown plugin")
	}
}

// Doctor must surface a conflict the user cannot otherwise see.
func TestDoctor_IncludesAppTypeConflicts(t *testing.T) {
	withTempXDG(t)
	writePlugin(t, capsManifest)

	stateHome := os.Getenv("XDG_STATE_HOME")
	rival := filepath.Join(stateHome, "dia", "plugins", "rival-plug")
	if err := os.MkdirAll(rival, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"rival-plug","name":"R","version":"0.1.0","entry":"index.js",` +
		`"capabilities":["apps:resolve"],"app_types":["capstype"],"ui":{"type":"kv","title":"T"}}`
	if err := os.WriteFile(filepath.Join(rival, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rival, "index.js"),
		[]byte(`module.exports = { resolveApp: function () { return { cmd: "x" }; } };`), 0o644); err != nil {
		t.Fatal(err)
	}

	a := startedApp(t)
	if err := a.SetPluginCapabilities("caps-plug", []string{"apps:resolve"}); err != nil {
		t.Fatal(err)
	}
	if err := a.SetPluginCapabilities("rival-plug", []string{"apps:resolve"}); err != nil {
		t.Fatal(err)
	}

	found := false
	for _, c := range a.Doctor() {
		if c.Name == "app type conflict" {
			found = true
		}
	}
	if !found {
		t.Error("Doctor should report the app-type conflict")
	}
}
