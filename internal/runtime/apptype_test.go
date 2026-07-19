package runtime

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/plugins"
	"github.com/DerekCorniello/dia/internal/registry"
	"github.com/DerekCorniello/dia/internal/state"
)

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// resolverHost satisfies plugins.HostAPI without doing anything. App
// types resolve in a restricted runtime that cannot reach the host, so
// nothing here is ever called; the manager just requires a non-nil one.
type resolverHost struct{ plugins.HostAPI }

// pluginManagerWithType installs a plugin claiming `typeName`, grants
// it apps:resolve, and returns the manager. js is the plugin entry.
func pluginManagerWithType(t *testing.T, typeName, js string) *plugins.Manager {
	t.Helper()
	dir := t.TempDir()
	pdir := filepath.Join(dir, "launch-plug")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"launch-plug","name":"L","version":"0.1.0","entry":"index.js",` +
		`"capabilities":["apps:resolve"],"app_types":["` + typeName + `"],` +
		`"ui":{"type":"kv","title":"T"}}`
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "index.js"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr, err := plugins.NewManager(dir, &resolverHost{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}
	mgr.ApplyPersistedGrants(map[string][]string{"launch-plug": {plugins.CapAppsResolve}})
	return mgr
}

// runtimeWithPluginTypes builds a Runtime whose registry has the
// plugin's app types wired in, matching what the CLI and GUI do.
func runtimeWithPluginTypes(t *testing.T, mgr *plugins.Manager) (*Runtime, *mockPlatform) {
	t.Helper()
	st, err := state.OpenAt(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	reg := registry.New()
	if errs := registry.SyncPluginTypes(reg, mgr); len(errs) != 0 {
		t.Fatalf("SyncPluginTypes: %v", errs)
	}
	pf := newMock()
	rt := New(Options{Platform: pf, Store: st, Registry: reg, Logger: quietLogger()})
	return rt, pf
}

// The whole point of the feature: a workspace naming a plugin-provided
// type actually launches the command the plugin computed. Every other
// test stops at --dry-run or reg.Resolve; this one goes through
// Runtime.Start to platform.Launch.
func TestStart_LaunchesPluginProvidedAppType(t *testing.T) {
	mgr := pluginManagerWithType(t, "compose", `module.exports = {
		resolveApp: function (app) {
			return {
				cmd: "docker compose",
				args: ["up", "-d"].concat(app.args || []),
				cwd: app.cwd,
				env: { COMPOSE_PROJECT_NAME: "demo" }
			};
		}
	};`)
	rt, pf := runtimeWithPluginTypes(t, mgr)

	inst, err := rt.Start(&config.Workspace{
		Name: "svc",
		Apps: []config.App{{Type: "compose", Cwd: "/srv/api", Args: []string{"--build"}}},
	}, config.Source{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if inst.Status != state.StatusRunning {
		t.Fatalf("Status = %s, want running (apps: %+v)", inst.Status, inst.Apps)
	}

	if len(pf.launched) != 1 {
		t.Fatalf("launched %d processes, want 1", len(pf.launched))
	}
	got := pf.launched[0]
	// "docker compose" has to be split into program + arg, not exec'd
	// as a single binary name.
	if got.Cmd != "docker" {
		t.Errorf("Cmd = %q, want docker", got.Cmd)
	}
	want := []string{"compose", "up", "-d", "--build"}
	if strings.Join(got.Args, "|") != strings.Join(want, "|") {
		t.Errorf("Args = %v, want %v", got.Args, want)
	}
	if got.Cwd != "/srv/api" {
		t.Errorf("Cwd = %q, want /srv/api", got.Cwd)
	}
	if len(got.Env) != 1 || got.Env[0] != "COMPOSE_PROJECT_NAME=demo" {
		t.Errorf("Env = %v", got.Env)
	}
	if inst.Apps[0].Status != state.StatusRunning || inst.Apps[0].PID <= 0 {
		t.Errorf("app process not tracked: %+v", inst.Apps[0])
	}
}

// A resolver returning a URL must open it rather than launch anything.
func TestStart_PluginAppTypeOpeningURL(t *testing.T) {
	mgr := pluginManagerWithType(t, "dash", `module.exports = {
		resolveApp: function () { return { url: "https://example.com/dash" }; }
	};`)
	rt, pf := runtimeWithPluginTypes(t, mgr)

	inst, err := rt.Start(&config.Workspace{
		Name: "svc",
		Apps: []config.App{{Type: "dash"}},
	}, config.Source{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(pf.launched) != 0 {
		t.Errorf("a url descriptor must not launch a process, got %+v", pf.launched)
	}
	if len(pf.openURLs) != 1 || pf.openURLs[0] != "https://example.com/dash" {
		t.Errorf("openURLs = %v", pf.openURLs)
	}
	if inst.Apps[0].Cmd != "https://example.com/dash" {
		t.Errorf("the opened URL should be recorded in state, got %q", inst.Apps[0].Cmd)
	}
}

// A throwing resolver must fail that one app, not the workspace, and
// the reason has to reach the user through the app's Err.
func TestStart_PluginResolverErrorFailsOnlyThatApp(t *testing.T) {
	mgr := pluginManagerWithType(t, "broken", `module.exports = {
		resolveApp: function () { throw new Error("no compose file here"); }
	};`)
	rt, pf := runtimeWithPluginTypes(t, mgr)

	inst, err := rt.Start(&config.Workspace{
		Name: "svc",
		Apps: []config.App{
			{Type: "broken"},
			{Type: "local", Cmd: "echo ok"},
		},
	}, config.Source{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if inst.Apps[0].Status != state.StatusCrashed {
		t.Errorf("the broken app should be crashed, got %s", inst.Apps[0].Status)
	}
	if !strings.Contains(inst.Apps[0].Err, "no compose file here") {
		t.Errorf("Err = %q, should carry the plugin's message", inst.Apps[0].Err)
	}
	// The healthy app still comes up.
	if inst.Apps[1].Status != state.StatusRunning {
		t.Errorf("the other app should still run, got %s", inst.Apps[1].Status)
	}
	if len(pf.launched) != 1 {
		t.Errorf("launched %d, want 1 (only the healthy app)", len(pf.launched))
	}
}

// A descriptor the host cannot use must fail with a message naming the
// plugin, not with something cryptic from deep in the launch path.
func TestStart_MalformedDescriptorIsReported(t *testing.T) {
	mgr := pluginManagerWithType(t, "bad", `module.exports = {
		resolveApp: function () { return { cmd: "ls", url: "https://x.com" }; }
	};`)
	rt, pf := runtimeWithPluginTypes(t, mgr)

	inst, err := rt.Start(&config.Workspace{
		Name: "svc",
		Apps: []config.App{{Type: "bad"}},
	}, config.Source{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if inst.Apps[0].Status != state.StatusCrashed {
		t.Fatalf("want crashed, got %s", inst.Apps[0].Status)
	}
	if !strings.Contains(inst.Apps[0].Err, "launch-plug") {
		t.Errorf("Err = %q, should name the offending plugin", inst.Apps[0].Err)
	}
	if len(pf.launched) != 0 {
		t.Error("nothing should launch from an invalid descriptor")
	}
}

// Revoking the grant has to stop the type resolving on the very next
// start, not at the next restart.
func TestStart_RevokedGrantStopsResolving(t *testing.T) {
	mgr := pluginManagerWithType(t, "compose", `module.exports = {
		resolveApp: function () { return { cmd: "docker" }; }
	};`)
	st, err := state.OpenAt(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	reg := registry.New()
	registry.SyncPluginTypes(reg, mgr)
	pf := newMock()
	rt := New(Options{Platform: pf, Store: st, Registry: reg, Logger: quietLogger()})

	ws := &config.Workspace{Name: "svc", Apps: []config.App{{Type: "compose"}}}
	if _, err := rt.Start(ws, config.Source{}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(pf.launched) != 1 {
		t.Fatalf("expected the type to work while granted, launched %d", len(pf.launched))
	}

	mgr.ApplyPersistedGrants(map[string][]string{"launch-plug": {}})
	registry.SyncPluginTypes(reg, mgr)

	inst, err := rt.Start(ws, config.Source{})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if inst.Apps[0].Status != state.StatusCrashed {
		t.Errorf("want crashed after revoke, got %s", inst.Apps[0].Status)
	}
	if len(pf.launched) != 1 {
		t.Errorf("nothing more should launch after the grant is revoked, launched %d", len(pf.launched))
	}
}
