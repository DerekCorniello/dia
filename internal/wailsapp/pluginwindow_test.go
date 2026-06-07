package wailsapp

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DerekCorniello/dia/internal/plugins"
)

func writeWhiteboardPlugin(t *testing.T, dir string) string {
	t.Helper()
	pluginDir := filepath.Join(dir, "whiteboard")
	if err := os.MkdirAll(filepath.Join(pluginDir, "panel"), 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "id": "whiteboard",
  "name": "Whiteboard",
  "version": "0.1.0",
  "entry": "index.js",
  "capabilities": [],
  "ui": {
    "type": "window",
    "title": "Whiteboard",
    "entry": "panel/panel.js",
    "width": 800,
    "height": 600
  }
}
`
	if err := os.WriteFile(filepath.Join(pluginDir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "index.js"), []byte("module.exports = { getData: function () { return {}; } };"), 0o644); err != nil {
		t.Fatal(err)
	}
	panel := `(function () { document.getElementById('root'); })();`
	if err := os.WriteFile(filepath.Join(pluginDir, "panel", "panel.js"), []byte(panel), 0o644); err != nil {
		t.Fatal(err)
	}
	return pluginDir
}

func loadWhiteboardManifest(t *testing.T, dir string) *plugins.Manifest {
	t.Helper()
	m, err := plugins.LoadManifest(dir)
	if err != nil {
		t.Fatalf("load manifest: %v", err)
	}
	return m
}

func TestPluginAssetHandler_ServesPanelJS(t *testing.T) {
	dir := t.TempDir()
	pluginDir := writeWhiteboardPlugin(t, dir)
	h := &pluginAssetHandler{pluginDir: pluginDir, manifest: loadWhiteboardManifest(t, pluginDir)}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panel.js", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body, _ := io.ReadAll(rr.Result().Body)
	if !strings.Contains(string(body), "document.getElementById") {
		t.Errorf("body did not contain panel.js contents: %q", body)
	}
}

func TestPluginAssetHandler_GeneratesIndexWhenMissing(t *testing.T) {
	dir := t.TempDir()
	pluginDir := writeWhiteboardPlugin(t, dir)
	h := &pluginAssetHandler{pluginDir: pluginDir, manifest: loadWhiteboardManifest(t, pluginDir)}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body, _ := io.ReadAll(rr.Result().Body)
	if !strings.Contains(string(body), "<div id=\"root\">") {
		t.Errorf("generated html missing root: %q", body)
	}
	if !strings.Contains(string(body), "/panel.js") {
		t.Errorf("generated html missing panel.js script: %q", body)
	}
}

func TestPluginAssetHandler_ServesDiaJS(t *testing.T) {
	dir := t.TempDir()
	pluginDir := writeWhiteboardPlugin(t, dir)
	h := &pluginAssetHandler{pluginDir: pluginDir, manifest: loadWhiteboardManifest(t, pluginDir)}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dia.js", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body, _ := io.ReadAll(rr.Result().Body)
	if !strings.Contains(string(body), "window.dia") {
		t.Errorf("dia.js missing window.dia: %q", body)
	}
}

func TestPluginAssetHandler_404OnUnknown(t *testing.T) {
	dir := t.TempDir()
	pluginDir := writeWhiteboardPlugin(t, dir)
	h := &pluginAssetHandler{pluginDir: pluginDir, manifest: loadWhiteboardManifest(t, pluginDir)}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/etc/passwd", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestPluginAssetHandler_ServesCustomIndex(t *testing.T) {
	dir := t.TempDir()
	pluginDir := writeWhiteboardPlugin(t, dir)
	custom := "<!doctype html><html><body><h1>custom</h1></body></html>"
	if err := os.WriteFile(filepath.Join(pluginDir, "panel", "index.html"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &pluginAssetHandler{pluginDir: pluginDir, manifest: loadWhiteboardManifest(t, pluginDir)}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body, _ := io.ReadAll(rr.Result().Body)
	if !strings.Contains(string(body), "custom") {
		t.Errorf("custom index not served: %q", body)
	}
}

func TestPluginAssetHandler_StylesMissingReturns404(t *testing.T) {
	dir := t.TempDir()
	pluginDir := writeWhiteboardPlugin(t, dir)
	h := &pluginAssetHandler{pluginDir: pluginDir, manifest: loadWhiteboardManifest(t, pluginDir)}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/styles.css", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestPluginWindowHost_DispatchReadOnly(t *testing.T) {
	stateDir := t.TempDir()
	h, err := newPluginWindowHost(stateDir)
	if err != nil {
		t.Fatalf("newPluginWindowHost: %v", err)
	}
	for _, method := range []string{"listWorkspaces", "listInstances", "doctor", "paths", "getTheme", "listCustomThemes"} {
		if _, err := h.dispatch(method, nil); err != nil {
			t.Errorf("dispatch(%q) error: %v", method, err)
		}
	}
	if _, err := h.dispatch("startWorkspace", nil); !errIs(err, errHostUnhandled) {
		t.Errorf("startWorkspace should be unhandled, got %v", err)
	}
}

func TestPluginWindowHost_PathsIncludesStateFile(t *testing.T) {
	stateDir := t.TempDir()
	h, err := newPluginWindowHost(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	v, err := h.dispatch("paths", nil)
	if err != nil {
		t.Fatal(err)
	}
	paths, ok := v.(PathsInfo)
	if !ok {
		t.Fatalf("paths returned %T, want PathsInfo", v)
	}
	if paths.StateFile == "" {
		t.Errorf("state file path empty")
	}
}

func errIs(err, target error) bool {
	return err != nil && (err == target || (err.Error() == target.Error()))
}
