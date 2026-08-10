package wailsapp

import (
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// gitPluginRepo builds a real local git repo holding a plugin and
// returns a file:// URL, exercising the clone path without a network.
func gitPluginRepo(t *testing.T, manifest string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.js"),
		[]byte(`module.exports = { getData: function () { return {}; },
			resolveApp: function () { return { cmd: "from-git" }; } };`), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "t@e.com"}, {"config", "user.name", "T"},
		{"add", "."}, {"commit", "-q", "-m", "initial"},
	} {
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	// A file:// URL needs a hostless triple-slash form with a leading
	// path slash; on Windows a drive letter must not be parsed as the
	// host (file://C:/... hangs the clone). Slashes must be forward.
	p := filepath.ToSlash(dir)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return (&url.URL{Scheme: "file", Path: p}).String()
}

// urlPath reverses a file:// URL from gitPluginRepo back into the
// filesystem path the repo lives at.
func urlPath(u string) (string, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	p := parsed.Path
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = strings.TrimPrefix(p, "/")
	}
	return filepath.FromSlash(p), nil
}

const gitCapsManifest = `{"id":"caps-plug","name":"Caps","version":"0.2.0","entry":"index.js",` +
	`"capabilities":["workspaces:read","apps:resolve"],"app_types":["capstype"],` +
	`"ui":{"type":"kv","title":"T"}}`

// Inspecting must disclose what the plugin wants without installing it.
func TestInspectPluginSource_DisclosesWithoutInstalling(t *testing.T) {
	withTempXDG(t)
	a := startedApp(t)
	url := gitPluginRepo(t, gitCapsManifest)

	info, err := a.InspectPluginSource(url, "")
	if err != nil {
		t.Fatalf("InspectPluginSource: %v", err)
	}
	if info.ID != "caps-plug" || info.Version != "0.2.0" {
		t.Errorf("got %+v", info)
	}
	if !info.IsGit {
		t.Error("a file:// remote should be reported as a git source")
	}
	if len(info.AppTypes) != 1 || info.AppTypes[0] != "capstype" {
		t.Errorf("AppTypes = %v", info.AppTypes)
	}
	var sawMutating bool
	for _, c := range info.Requested {
		if c.Name == "apps:resolve" && c.Mutating {
			sawMutating = true
		}
	}
	if !sawMutating {
		t.Errorf("apps:resolve should be disclosed as mutating: %+v", info.Requested)
	}
	// Nothing installed.
	if _, ok := a.pmgr.Get("caps-plug"); ok {
		t.Error("InspectPluginSource must not install the plugin")
	}
}

func TestInspectPluginSource_RejectsBadSource(t *testing.T) {
	withTempXDG(t)
	a := startedApp(t)
	if _, err := a.InspectPluginSource("owner/repo", ""); err == nil {
		t.Error("a bare owner/repo is ambiguous and must be rejected")
	}
}

func TestInstallPluginFromSource_GitInstallsWithReadOnlyGrants(t *testing.T) {
	withTempXDG(t)
	a := startedApp(t)
	url := gitPluginRepo(t, gitCapsManifest)

	if _, err := a.InstallPluginFromSource(url, ""); err != nil {
		t.Fatalf("InstallPluginFromSource: %v", err)
	}
	info, err := a.GetPluginCapabilities("caps-plug")
	if err != nil {
		t.Fatalf("GetPluginCapabilities: %v", err)
	}
	// Installing discloses; it does not authorize.
	if plugins.HasCapability(info.Granted, "apps:resolve") {
		t.Errorf("installing must not grant a mutating capability, got %v", info.Granted)
	}
	if !plugins.HasCapability(info.Granted, "workspaces:read") {
		t.Errorf("read capability should be granted, got %v", info.Granted)
	}
	// And the app type stays unclaimed until it is granted.
	if _, err := a.reg.Resolve(config.App{Type: "capstype"}); err == nil {
		t.Error("app type should not resolve before apps:resolve is granted")
	}
}

// Installing has to make the plugin's app types usable as soon as the
// capability is granted, without restarting.
func TestInstallPluginFromSource_TypeUsableAfterGranting(t *testing.T) {
	withTempXDG(t)
	a := startedApp(t)
	url := gitPluginRepo(t, gitCapsManifest)

	if _, err := a.InstallPluginFromSource(url, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.SetPluginCapabilities("caps-plug", []string{"apps:resolve"}); err != nil {
		t.Fatal(err)
	}
	action, err := a.reg.Resolve(config.App{Type: "capstype"})
	if err != nil {
		t.Fatalf("app type should resolve: %v", err)
	}
	if action.Launch == nil || action.Launch.Cmd != "from-git" {
		t.Errorf("unexpected action: %+v", action)
	}
}

func TestUpdatePlugin_RequiresAGitSource(t *testing.T) {
	withTempXDG(t)
	writePlugin(t, capsManifest)
	a := startedApp(t)

	// Written to the plugins dir directly, so there is no provenance.
	if a.PluginIsUpdatable("caps-plug") {
		t.Error("a path-installed plugin is not updatable")
	}
	if _, err := a.UpdatePlugin("caps-plug"); err == nil {
		t.Error("expected an error updating a plugin with no recorded source")
	}
}

func TestUpdatePlugin_ReclonesAndKeepsGrants(t *testing.T) {
	withTempXDG(t)
	a := startedApp(t)
	url := gitPluginRepo(t, gitCapsManifest)
	repoDir, err := urlPath(url)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := a.InstallPluginFromSource(url, ""); err != nil {
		t.Fatal(err)
	}
	if !a.PluginIsUpdatable("caps-plug") {
		t.Fatal("a git-installed plugin should be updatable")
	}
	if err := a.SetPluginCapabilities("caps-plug", []string{"apps:resolve"}); err != nil {
		t.Fatal(err)
	}

	bumped := strings.Replace(gitCapsManifest, `"version":"0.2.0"`, `"version":"0.9.0"`, 1)
	if err := os.WriteFile(filepath.Join(repoDir, "plugin.json"), []byte(bumped), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-q", "-m", "bump"}} {
		c := exec.Command("git", args...)
		c.Dir = repoDir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	if _, err := a.UpdatePlugin("caps-plug"); err != nil {
		t.Fatalf("UpdatePlugin: %v", err)
	}
	l, _ := a.pmgr.Get("caps-plug")
	if l.Manifest.Version != "0.9.0" {
		t.Errorf("Version = %q, want 0.9.0", l.Manifest.Version)
	}
	// An update is not a reinstall; the grant survives.
	if !plugins.HasCapability(l.GrantedCaps, "apps:resolve") {
		t.Errorf("grants should survive an update, got %v", l.GrantedCaps)
	}
}

func TestUninstallPlugin_RemovesTypesAndForgetsGrants(t *testing.T) {
	withTempXDG(t)
	a := startedApp(t)
	url := gitPluginRepo(t, gitCapsManifest)

	if _, err := a.InstallPluginFromSource(url, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.SetPluginCapabilities("caps-plug", []string{"apps:resolve"}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.reg.Resolve(config.App{Type: "capstype"}); err != nil {
		t.Fatalf("precondition: type should resolve, got %v", err)
	}

	if err := a.UninstallPlugin("caps-plug"); err != nil {
		t.Fatalf("UninstallPlugin: %v", err)
	}
	if _, ok := a.pmgr.Get("caps-plug"); ok {
		t.Error("plugin should be gone")
	}
	if _, err := a.reg.Resolve(config.App{Type: "capstype"}); err == nil {
		t.Error("the app type should be unregistered on uninstall")
	}
	// Leaving grants behind would silently re-authorize a reinstall.
	if ps, ok := a.store.Snapshot().Plugins["caps-plug"]; ok && len(ps.GrantedCapabilities) > 0 {
		t.Errorf("persisted grants should be forgotten, got %+v", ps)
	}
}

func TestUninstallPlugin_UnknownPlugin(t *testing.T) {
	withTempXDG(t)
	a := startedApp(t)
	if err := a.UninstallPlugin("nope"); err == nil {
		t.Error("expected an error for an unknown plugin")
	}
}

// The reported crash: dia would not start at all when one installed
// plugin had an unloadable manifest, panicking in
// applyPersistedPluginState -> Manager.List.
func TestStartup_SurvivesABrokenPlugin(t *testing.T) {
	withTempXDG(t)
	writePlugin(t, capsManifest)

	stateHome := os.Getenv("XDG_STATE_HOME")
	broken := filepath.Join(stateHome, "dia", "plugins", "brokenplug")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "plugin.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := startedApp(t)

	list := a.ListPlugins()
	if len(list) != 2 {
		t.Fatalf("got %d plugins, want 2: %+v", len(list), list)
	}
	var brokenInfo *PluginInfo
	for i := range list {
		if list[i].ID == "brokenplug" {
			brokenInfo = &list[i]
		}
	}
	if brokenInfo == nil {
		t.Fatalf("the broken plugin should be listed with its id: %+v", list)
	}
	if brokenInfo.Status != "errored" || brokenInfo.LastError == "" {
		t.Errorf("it should carry its error: %+v", brokenInfo)
	}
	// The healthy plugin still works.
	if err := a.SetPluginCapabilities("caps-plug", []string{"apps:resolve"}); err != nil {
		t.Fatalf("the healthy plugin should still be grantable: %v", err)
	}
	if _, err := a.reg.Resolve(config.App{Type: "capstype"}); err != nil {
		t.Errorf("its app type should still resolve: %v", err)
	}
}

func TestGetPluginCapabilities_BrokenPluginFailsCleanly(t *testing.T) {
	withTempXDG(t)
	stateHome := os.Getenv("XDG_STATE_HOME")
	broken := filepath.Join(stateHome, "dia", "plugins", "brokenplug")
	if err := os.MkdirAll(broken, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "plugin.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	a := startedApp(t)

	if _, err := a.GetPluginCapabilities("brokenplug"); err == nil {
		t.Error("expected an error, not a panic")
	}
	if err := a.SetPluginCapabilities("brokenplug", []string{"cmd:exec"}); err == nil {
		t.Error("expected an error, not a panic")
	}
	if pids := a.Doctor(); len(pids) == 0 {
		t.Error("doctor should still run")
	}
}
