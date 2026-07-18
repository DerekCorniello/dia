package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupResolverPlugin writes a plugin that claims appTypes and returns
// a manager with it discovered and granted apps:resolve.
func setupResolverPlugin(t *testing.T, id string, appTypes []string, js string, cfg map[string]any) *Manager {
	t.Helper()
	dir := t.TempDir()
	pdir := filepath.Join(dir, id)
	if err := os.MkdirAll(pdir, 0o755); err != nil {
		t.Fatal(err)
	}
	types, err := json.Marshal(appTypes)
	if err != nil {
		t.Fatal(err)
	}
	manifest := `{"id":"` + id + `","name":"T","version":"0.1.0","entry":"index.js",` +
		`"capabilities":["apps:resolve"],"app_types":` + string(types) +
		`,"ui":{"type":"kv","title":"T"}}`
	if err := os.WriteFile(filepath.Join(pdir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pdir, "index.js"), []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr, err := NewManager(dir, &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}
	if cfg != nil {
		if err := mgr.SetConfig(id, cfg); err != nil {
			t.Fatal(err)
		}
	}
	mgr.ApplyPersistedGrants(map[string][]string{id: {CapAppsResolve}})
	return mgr
}

const echoResolver = `module.exports = {
	resolveApp: function (app) {
		return { cmd: "echo", args: [app.type, app.cmd || ""], cwd: app.cwd };
	}
};`

func TestResolveApp_ReturnsDescriptor(t *testing.T) {
	mgr := setupResolverPlugin(t, "echo-type", []string{"echo-app"}, echoResolver, nil)

	got, err := mgr.ResolveApp("echo-app", map[string]any{
		"type": "echo-app", "cmd": "hello", "cwd": "/tmp",
	})
	if err != nil {
		t.Fatalf("ResolveApp: %v", err)
	}
	if got["cmd"] != "echo" {
		t.Errorf("cmd = %v, want echo", got["cmd"])
	}
	if got["cwd"] != "/tmp" {
		t.Errorf("cwd = %v, want /tmp", got["cwd"])
	}
}

func TestAppTypes_ClaimedOnlyWithCapabilityGranted(t *testing.T) {
	mgr := setupResolverPlugin(t, "echo-type", []string{"echo-app"}, echoResolver, nil)
	if _, ok := mgr.AppTypes()["echo-app"]; !ok {
		t.Fatal("expected echo-app to be claimed once apps:resolve is granted")
	}

	// Revoke the capability: the claim must go with it.
	mgr.ApplyPersistedGrants(map[string][]string{"echo-type": {}})
	if _, ok := mgr.AppTypes()["echo-app"]; ok {
		t.Error("app type should not be claimed without apps:resolve")
	}
	if _, err := mgr.ResolveApp("echo-app", map[string]any{"type": "echo-app"}); err == nil {
		t.Error("expected ResolveApp to fail for an unclaimed type")
	}
}

func TestResolveApp_UnknownType(t *testing.T) {
	mgr := setupResolverPlugin(t, "echo-type", []string{"echo-app"}, echoResolver, nil)
	_, err := mgr.ResolveApp("not-claimed", map[string]any{})
	if err == nil {
		t.Fatal("expected an error for an unclaimed type")
	}
	if !strings.Contains(err.Error(), "no plugin provides") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResolveApp_MissingExport(t *testing.T) {
	mgr := setupResolverPlugin(t, "no-resolve", []string{"whatever"},
		`module.exports = { getData: function () { return {}; } };`, nil)
	_, err := mgr.ResolveApp("whatever", map[string]any{"type": "whatever"})
	if err == nil {
		t.Fatal("expected an error when resolveApp is not exported")
	}
	if !strings.Contains(err.Error(), "resolveApp") {
		t.Errorf("error should name the missing export: %v", err)
	}
}

func TestResolveApp_ThrownErrorIsReported(t *testing.T) {
	mgr := setupResolverPlugin(t, "thrower", []string{"boom"},
		`module.exports = { resolveApp: function () { throw new Error("no compose file"); } };`, nil)
	_, err := mgr.ResolveApp("boom", map[string]any{"type": "boom"})
	if err == nil {
		t.Fatal("expected the thrown error to surface")
	}
	if !strings.Contains(err.Error(), "no compose file") {
		t.Errorf("error should carry the plugin's message: %v", err)
	}
}

// The resolver is pure by construction. If this starts passing, the
// restricted dia object has grown a host-reaching method and
// --dry-run is no longer side-effect free.
func TestResolverRuntime_HasNoHostAccess(t *testing.T) {
	for _, call := range []string{
		"dia.listWorkspaces()",
		"dia.exec('echo', ['hi'])",
		"dia.fetch('http://example.com')",
		"dia.startWorkspace('x')",
	} {
		t.Run(call, func(t *testing.T) {
			js := `module.exports = { resolveApp: function () { return { cmd: String(` + call + `) }; } };`
			mgr := setupResolverPlugin(t, "pure-test", []string{"pure"}, js, nil)
			_, err := mgr.ResolveApp("pure", map[string]any{"type": "pure"})
			if err == nil {
				t.Fatalf("%s should not be reachable from a resolver", call)
			}
		})
	}
}

func TestResolverRuntime_CanReadItsOwnConfig(t *testing.T) {
	js := `module.exports = {
		resolveApp: function () {
			var cfg = dia.getConfig() || {};
			return { cmd: cfg.binary || "fallback" };
		}
	};`
	mgr := setupResolverPlugin(t, "cfg-test", []string{"cfg"}, js,
		map[string]any{"binary": "podman-compose"})

	got, err := mgr.ResolveApp("cfg", map[string]any{"type": "cfg"})
	if err != nil {
		t.Fatalf("ResolveApp: %v", err)
	}
	if got["cmd"] != "podman-compose" {
		t.Errorf("cmd = %v, want the configured binary", got["cmd"])
	}
}

func TestResolveApp_ConflictingClaimIsRecorded(t *testing.T) {
	dir := t.TempDir()
	for _, id := range []string{"alpha-plug", "beta-plug"} {
		pdir := filepath.Join(dir, id)
		if err := os.MkdirAll(pdir, 0o755); err != nil {
			t.Fatal(err)
		}
		manifest := `{"id":"` + id + `","name":"T","version":"0.1.0","entry":"index.js",` +
			`"capabilities":["apps:resolve"],"app_types":["shared"],"ui":{"type":"kv","title":"T"}}`
		if err := os.WriteFile(filepath.Join(pdir, "plugin.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(pdir, "index.js"), []byte(echoResolver), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mgr, err := NewManager(dir, &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}
	mgr.ApplyPersistedGrants(map[string][]string{
		"alpha-plug": {CapAppsResolve},
		"beta-plug":  {CapAppsResolve},
	})

	conflicts := mgr.AppTypeConflicts()
	if len(conflicts) != 1 {
		t.Fatalf("got %d conflicts, want 1: %+v", len(conflicts), conflicts)
	}
	// Ordering must be deterministic, not dependent on map iteration.
	if conflicts[0].Winner != "alpha-plug" || conflicts[0].Loser != "beta-plug" {
		t.Errorf("winner/loser = %s/%s, want alpha-plug/beta-plug",
			conflicts[0].Winner, conflicts[0].Loser)
	}
	if mgr.AppTypes()["shared"] != "alpha-plug" {
		t.Errorf("shared claimed by %q, want alpha-plug", mgr.AppTypes()["shared"])
	}
}

func TestDecodeDescriptor(t *testing.T) {
	tests := []struct {
		name    string
		in      map[string]any
		wantErr string
	}{
		{name: "cmd only", in: map[string]any{"cmd": "ls"}},
		{name: "url only", in: map[string]any{"url": "https://example.com"}},
		{
			name:    "both",
			in:      map[string]any{"cmd": "ls", "url": "https://example.com"},
			wantErr: "exactly one",
		},
		{name: "neither", in: map[string]any{}, wantErr: "exactly one"},
		{
			name:    "url with cmd-only fields",
			in:      map[string]any{"url": "https://example.com", "cwd": "/tmp"},
			wantErr: "cmd-only fields",
		},
		{
			name:    "unknown key",
			in:      map[string]any{"command": "ls"},
			wantErr: "unusable object",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeDescriptor(tt.in)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestManifest_AppTypeValidation(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr string
	}{
		{
			name: "valid",
			json: `{"id":"ok-plug","name":"N","version":"1","capabilities":["apps:resolve"],` +
				`"app_types":["compose","compose:down"],"ui":{"type":"kv","title":"T"}}`,
		},
		{
			name: "app_types without the capability",
			json: `{"id":"ok-plug","name":"N","version":"1","app_types":["compose"],` +
				`"ui":{"type":"kv","title":"T"}}`,
			wantErr: "apps:resolve",
		},
		{
			name: "bad type name",
			json: `{"id":"ok-plug","name":"N","version":"1","capabilities":["apps:resolve"],` +
				`"app_types":["Not Valid"],"ui":{"type":"kv","title":"T"}}`,
			wantErr: "must match",
		},
		{
			name: "duplicate type",
			json: `{"id":"ok-plug","name":"N","version":"1","capabilities":["apps:resolve"],` +
				`"app_types":["a-type","a-type"],"ui":{"type":"kv","title":"T"}}`,
			wantErr: "declared twice",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(tt.json), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadManifest(dir)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestAppsResolveIsMutating(t *testing.T) {
	// It is not a read capability: the command a resolver returns is
	// executed by dia, so it must never be granted by default.
	if !IsMutatingCapability(CapAppsResolve) {
		t.Error("apps:resolve must be mutating")
	}
	for _, c := range DefaultReadCapabilities() {
		if c == CapAppsResolve {
			t.Error("apps:resolve must not be granted by default")
		}
	}
}
