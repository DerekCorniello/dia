package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installAppTypePlugin writes a plugin claiming `compose` into the
// global plugins dir under stateDir, and grants it apps:resolve in the
// state file -- the same two things the user does with
// `dia plugin install` and `dia plugin enable --caps`.
func installAppTypePlugin(t *testing.T, stateDir string) {
	t.Helper()
	pdir := filepath.Join(stateDir, "dia", "plugins", "compose-type")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"compose-type","name":"Compose","version":"0.1.0","entry":"index.js",` +
		`"capabilities":["apps:resolve"],"app_types":["compose"],"ui":{"type":"kv","title":"T"}}`
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	js := `module.exports = {
		resolveApp: function (app) {
			return { cmd: "docker compose", args: ["up", "-d"], cwd: app.cwd };
		}
	};`
	if err := os.WriteFile(filepath.Join(pdir, "index.js"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}

	stateFile := filepath.Join(stateDir, "dia", "state.json")
	if err := os.MkdirAll(filepath.Dir(stateFile), 0o755); err != nil {
		t.Fatal(err)
	}
	data := `{"version":1,"instances":{},"plugins":{"compose-type":` +
		`{"enabled":true,"granted_capabilities":["apps:resolve"]}}}`
	if err := os.WriteFile(stateFile, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeGlobalWorkspace puts a workspace where `dia start` will find it.
func writeGlobalWorkspace(t *testing.T, configHome, name, yaml string) {
	t.Helper()
	dir := filepath.Join(configHome, "dia", "workspaces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A plugin-provided app type must resolve end to end: the workspace
// names `compose`, and the command that comes back is the one the
// plugin computed, not anything in the YAML.
func TestDryRun_ResolvesPluginProvidedAppType(t *testing.T) {
	stateDir := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	installAppTypePlugin(t, stateDir)
	writeGlobalWorkspace(t, configHome, "svc", `name: svc
apps:
  - type: compose
    cwd: /srv/app
`)

	var out, errOut bytes.Buffer
	code := runWithIO([]string{"--json", "start", "svc", "--dry-run"},
		strings.NewReader(""), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("exit %d: %s%s", code, out.String(), errOut.String())
	}

	var payload struct {
		Apps []struct {
			Type   string `json:"type"`
			Action string `json:"action"`
			Cmd    string `json:"cmd"`
			Cwd    string `json:"cwd"`
			Plugin string `json:"plugin"`
			Error  string `json:"error"`
		} `json:"apps"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("parse json: %v\n%s", err, out.String())
	}
	if len(payload.Apps) != 1 {
		t.Fatalf("got %d apps, want 1: %+v", len(payload.Apps), payload.Apps)
	}
	app := payload.Apps[0]
	if app.Error != "" {
		t.Fatalf("resolution failed: %s", app.Error)
	}
	if app.Action != "launch" {
		t.Errorf("action = %q, want launch", app.Action)
	}
	if app.Cmd != "docker compose up -d" {
		t.Errorf("cmd = %q, want the plugin-resolved command", app.Cmd)
	}
	if app.Cwd != "/srv/app" {
		t.Errorf("cwd = %q, want /srv/app", app.Cwd)
	}
	// The report has to say where a non-builtin type came from.
	if app.Plugin != "compose-type" {
		t.Errorf("plugin = %q, want compose-type", app.Plugin)
	}
}

// Without the apps:resolve grant the type is not claimed, and the app
// fails to resolve rather than silently launching nothing.
func TestDryRun_UngrantedPluginTypeDoesNotResolve(t *testing.T) {
	stateDir := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	installAppTypePlugin(t, stateDir)
	// Revoke the grant.
	stateFile := filepath.Join(stateDir, "dia", "state.json")
	if err := os.WriteFile(stateFile,
		[]byte(`{"version":1,"instances":{},"plugins":{"compose-type":{"enabled":true,"granted_capabilities":[]}}}`),
		0o644); err != nil {
		t.Fatal(err)
	}
	writeGlobalWorkspace(t, configHome, "svc", "name: svc\napps:\n  - type: compose\n")

	var out, errOut bytes.Buffer
	code := runWithIO([]string{"--json", "start", "svc", "--dry-run"},
		strings.NewReader(""), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("dry-run should still report, exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "unknown app type") {
		t.Errorf("expected an unresolved app, got:\n%s", out.String())
	}
}

func TestDryRun_ReportsHooks(t *testing.T) {
	stateDir := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	writeGlobalWorkspace(t, configHome, "hooked", `name: hooked
hooks:
  pre_start:
    - docker compose up -d
apps:
  - type: local
    cmd: echo hi
`)

	var out, errOut bytes.Buffer
	code := runWithIO([]string{"start", "hooked", "--dry-run"},
		strings.NewReader(""), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	// Hooks change what a start actually does, so a preview that hides
	// them is misleading.
	if !strings.Contains(out.String(), "pre_start") {
		t.Errorf("dry run should list hooks:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "docker compose up -d") {
		t.Errorf("dry run should show the hook command:\n%s", out.String())
	}
}

// An app with no type at all is documented as "runs cmd"; it must
// survive a real resolve, not just config validation.
func TestDryRun_UntypedAppResolves(t *testing.T) {
	stateDir := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("XDG_CONFIG_HOME", configHome)

	writeGlobalWorkspace(t, configHome, "plain", "name: plain\napps:\n  - cmd: echo hi\n")

	var out, errOut bytes.Buffer
	code := runWithIO([]string{"start", "plain", "--dry-run"},
		strings.NewReader(""), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if strings.Contains(out.String(), "fail") {
		t.Errorf("an untyped app should resolve:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "echo hi") {
		t.Errorf("expected the resolved command:\n%s", out.String())
	}
}

func TestDoctor_ReportsPluginAppTypes(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	installAppTypePlugin(t, stateDir)

	var out, errOut bytes.Buffer
	code := runWithIO([]string{"doctor"}, strings.NewReader(""), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "compose") {
		t.Errorf("doctor should list plugin-provided app types:\n%s", out.String())
	}
}

func TestDoctor_WarnsOnAppTypeConflict(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	installAppTypePlugin(t, stateDir)

	// A second plugin claiming the same type.
	pdir := filepath.Join(stateDir, "dia", "plugins", "rival-type")
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"rival-type","name":"Rival","version":"0.1.0","entry":"index.js",` +
		`"capabilities":["apps:resolve"],"app_types":["compose"],"ui":{"type":"kv","title":"T"}}`
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "index.js"),
		[]byte(`module.exports = { resolveApp: function () { return { cmd: "rival" }; } };`), 0o644); err != nil {
		t.Fatal(err)
	}
	stateFile := filepath.Join(stateDir, "dia", "state.json")
	data := `{"version":1,"instances":{},"plugins":{` +
		`"compose-type":{"enabled":true,"granted_capabilities":["apps:resolve"]},` +
		`"rival-type":{"enabled":true,"granted_capabilities":["apps:resolve"]}}}`
	if err := os.WriteFile(stateFile, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	code := runWithIO([]string{"doctor"}, strings.NewReader(""), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "app type conflict") {
		t.Errorf("doctor should warn about the conflict:\n%s", out.String())
	}
}

// writeBrokenPluginDir drops a plugin with an unloadable manifest into
// the global plugins dir.
func writeBrokenPluginDir(t *testing.T, stateDir, id string) {
	t.Helper()
	dir := filepath.Join(stateDir, "dia", "plugins", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// `dia plugin list` crashed on a broken manifest: its nil check ran
// after Manager.List had already panicked sorting on Manifest.ID.
func TestPluginList_SurvivesABrokenManifest(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeBrokenPluginDir(t, stateDir, "brokenplug")
	installAppTypePlugin(t, stateDir)

	var out, errOut bytes.Buffer
	code := runWithIO([]string{"plugin", "list"}, strings.NewReader(""), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	// The broken plugin has to be visible, not silently dropped.
	if !strings.Contains(out.String(), "brokenplug") {
		t.Errorf("broken plugin should be listed:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "errored") {
		t.Errorf("its status should say errored:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "compose-type") {
		t.Errorf("the healthy plugin should still be listed:\n%s", out.String())
	}
}

// `plugin info` is how you diagnose a broken plugin, so it has to work
// on one and report the reason.
func TestPluginInfo_ReportsABrokenManifest(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeBrokenPluginDir(t, stateDir, "brokenplug")

	var out, errOut bytes.Buffer
	code := runWithIO([]string{"plugin", "info", "brokenplug"}, strings.NewReader(""), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "brokenplug") {
		t.Errorf("should report the id:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "error:") {
		t.Errorf("should report why it failed to load:\n%s", out.String())
	}
}

// A broken plugin must not stop an unrelated workspace from resolving.
func TestDryRun_UnaffectedByABrokenPlugin(t *testing.T) {
	stateDir := t.TempDir()
	configHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	writeBrokenPluginDir(t, stateDir, "brokenplug")
	installAppTypePlugin(t, stateDir)
	writeGlobalWorkspace(t, configHome, "svc", "name: svc\napps:\n  - type: compose\n")

	var out, errOut bytes.Buffer
	code := runWithIO([]string{"start", "svc", "--dry-run"}, strings.NewReader(""), &out, &errOut)
	if code != ExitOK {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "via plugin compose-type") {
		t.Errorf("the healthy plugin's type should still resolve:\n%s", out.String())
	}
}

func TestDoctor_SurvivesABrokenPlugin(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateDir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	writeBrokenPluginDir(t, stateDir, "brokenplug")

	if code := Run([]string{"doctor"}); code != ExitOK {
		t.Errorf("doctor returned %d", code)
	}
}
