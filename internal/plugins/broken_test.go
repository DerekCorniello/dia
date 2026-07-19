package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

// writeBrokenPlugin drops a plugin whose manifest cannot be loaded.
func writeBrokenPlugin(t *testing.T, base, id, manifest string) {
	t.Helper()
	dir := filepath.Join(base, id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

// One plugin with an unreadable manifest used to take down the whole
// GUI at startup: List sorted on Manifest.ID, and a failed load leaves
// Manifest nil. Reported as a SIGSEGV in applyPersistedPluginState.
func TestList_SurvivesABrokenManifest(t *testing.T) {
	dir := t.TempDir()
	writeBrokenPlugin(t, dir, "brokenplug", "{not json")
	writeBrokenPlugin(t, dir, "goodplug",
		`{"id":"goodplug","name":"G","version":"1","ui":{"type":"list","title":"T"}}`)

	mgr, err := NewManager(dir, &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}

	list := mgr.List()
	if len(list) != 2 {
		t.Fatalf("got %d plugins, want 2 (the broken one must still be listed)", len(list))
	}
	// Sorted by directory name, which every plugin has.
	if list[0].ID != "brokenplug" || list[1].ID != "goodplug" {
		t.Errorf("order = %q, %q; want brokenplug, goodplug", list[0].ID, list[1].ID)
	}
	if list[0].Manifest != nil {
		t.Error("the broken plugin should have no manifest")
	}
	if list[0].Status != StatusErrored {
		t.Errorf("Status = %q, want errored", list[0].Status)
	}
	if list[0].LastError == "" {
		t.Error("the broken plugin should carry the load error")
	}
}

// A manifest that parses but fails validation is the same hazard.
func TestList_SurvivesAnInvalidManifest(t *testing.T) {
	dir := t.TempDir()
	// app_types without the capability that gates them.
	writeBrokenPlugin(t, dir, "invalidplug",
		`{"id":"invalidplug","name":"N","version":"1","app_types":["x"],"ui":{"type":"kv","title":"T"}}`)

	mgr, err := NewManager(dir, &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}
	list := mgr.List()
	if len(list) != 1 || list[0].ID != "invalidplug" {
		t.Fatalf("got %+v", list)
	}
	if list[0].Status != StatusErrored {
		t.Errorf("Status = %q, want errored", list[0].Status)
	}
}

// Everything derived from the plugin set has to tolerate a broken one.
func TestBrokenPluginDoesNotBreakTheRest(t *testing.T) {
	dir := t.TempDir()
	writeBrokenPlugin(t, dir, "brokenplug", "{not json")
	writeBrokenPlugin(t, dir, "goodplug",
		`{"id":"goodplug","name":"G","version":"1","capabilities":["apps:resolve"],`+
			`"app_types":["goodtype"],"ui":{"type":"kv","title":"T"}}`)
	if err := os.WriteFile(filepath.Join(dir, "goodplug", "index.js"),
		[]byte(`module.exports = { resolveApp: function () { return { cmd: "ok" }; } };`), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr, err := NewManager(dir, &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}

	// Applying grants must skip the broken plugin rather than panic.
	mgr.ApplyPersistedGrants(map[string][]string{
		"brokenplug": {CapAppsResolve},
		"goodplug":   {CapAppsResolve},
	})
	if _, ok := mgr.AppTypes()["goodtype"]; !ok {
		t.Error("the healthy plugin's app type should still be claimed")
	}

	// Enabling the broken one fails cleanly.
	if err := mgr.Enable("brokenplug"); err == nil {
		t.Error("enabling a plugin with no manifest should fail, not panic")
	}
	// The healthy one still enables. EnableWithGrants, because plain
	// Enable resets to the read-only defaults and would drop the
	// apps:resolve granted above.
	if err := mgr.EnableWithGrants("goodplug", []string{CapAppsResolve}); err != nil {
		t.Errorf("the healthy plugin should still enable: %v", err)
	}
	// And resolving through it still works.
	if _, err := mgr.ResolveApp("goodtype", map[string]any{"type": "goodtype"}); err != nil {
		t.Errorf("resolve through the healthy plugin: %v", err)
	}
}

func TestSetGrants_BrokenPluginFailsCleanly(t *testing.T) {
	dir := t.TempDir()
	writeBrokenPlugin(t, dir, "brokenplug", "{not json")
	mgr, err := NewManager(dir, &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}
	if err := mgr.SetGrants("brokenplug", []string{CapCmdExec}); err == nil {
		t.Error("expected an error, not a panic")
	}
}

// A plugin that was fine and then breaks must keep its identity, or it
// vanishes from the list with no explanation.
func TestRediscover_KeepsIDWhenAManifestGoesBad(t *testing.T) {
	dir := t.TempDir()
	writeBrokenPlugin(t, dir, "flaky",
		`{"id":"flaky","name":"F","version":"1","ui":{"type":"list","title":"T"}}`)

	mgr, err := NewManager(dir, &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "flaky", "plugin.json"), []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := mgr.Discover(); err != nil {
		t.Fatal(err)
	}

	list := mgr.List()
	if len(list) != 1 {
		t.Fatalf("got %d, want 1", len(list))
	}
	if list[0].ID != "flaky" {
		t.Errorf("ID = %q, want flaky", list[0].ID)
	}
	if list[0].Status != StatusErrored || list[0].LastError == "" {
		t.Errorf("should be errored with a reason: %+v", list[0])
	}
}
