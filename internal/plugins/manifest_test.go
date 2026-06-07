package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func writePlugin(t *testing.T, dir string, manifest Manifest, entry string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mf := manifest
	if mf.Entry == "" {
		mf.Entry = "index.js"
	}
	data := []byte(`{"id":"` + mf.ID + `","name":"` + mf.Name + `","version":"` + mf.Version + `","description":"` + mf.Description + `","author":"` + mf.Author + `","entry":"` + mf.Entry + `","capabilities":[],"ui":{"type":"list","title":"Test"}}`)
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if entry != "" {
		if err := os.WriteFile(filepath.Join(dir, mf.Entry), []byte(entry), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
func TestManifestValid(t *testing.T) {
	m := Manifest{
		ID: "hello-world", Name: "Hello", Version: "0.1.0",
		UI: UISpec{Type: "list", Title: "Items"},
	}
	if err := m.Validate(); err != nil {
		t.Errorf("expected valid, got %v", err)
	}
}

func TestManifestCanvas(t *testing.T) {
	m := Manifest{
		ID: "whiteboard", Name: "Whiteboard", Version: "0.1.0",
		UI: UISpec{Type: "canvas", Title: "Board"},
	}
	if err := m.Validate(); err != nil {
		t.Errorf("expected valid canvas, got %v", err)
	}
}

func TestManifestWindow(t *testing.T) {
	m := Manifest{
		ID: "scratchpad", Name: "Scratchpad", Version: "0.1.0",
		UI: UISpec{Type: "window", Title: "Scratchpad", Entry: "panel/main.js", Width: 800, Height: 600},
	}
	if err := m.Validate(); err != nil {
		t.Errorf("expected valid window, got %v", err)
	}
	if m.UI.Entry != "panel/main.js" {
		t.Errorf("entry should remain %q, got %q", "panel/main.js", m.UI.Entry)
	}
}

func TestManifestWindowRejectsBadEntry(t *testing.T) {
	for _, e := range []string{"/abs/x.js", "../x.js"} {
		m := Manifest{
			ID: "scratchpad", Name: "Scratchpad", Version: "0.1.0",
			UI: UISpec{Type: "window", Title: "Scratchpad", Entry: e},
		}
		if err := m.Validate(); err == nil {
			t.Errorf("expected error for ui.entry=%q", e)
		}
	}
}
func TestManifestRejectsBadID(t *testing.T) {
	for _, id := range []string{"ab", "UPPER", "-leading", "trailing-", "with space", "with_underscore", "this-id-is-far-too-long-to-be-accepted-by-the-validator"} {
		m := Manifest{ID: id, Name: "n", Version: "0.1.0", UI: UISpec{Type: "list", Title: "t"}}
		if err := m.Validate(); err == nil {
			t.Errorf("expected error for id %q", id)
		}
	}
}
func TestManifestRequiresFields(t *testing.T) {
	tests := []struct {
		name string
		mod  func(m *Manifest)
	}{
		{"no name", func(m *Manifest) { m.Name = "" }},
		{"no version", func(m *Manifest) { m.Version = "" }},
		{"no ui.title", func(m *Manifest) { m.UI.Title = "" }},
		{"bad ui.type", func(m *Manifest) { m.UI.Type = "weird" }},
		{"table without columns", func(m *Manifest) { m.UI.Type = "table" }},
		{"abs entry", func(m *Manifest) { m.Entry = "/etc/passwd" }},
		{"parent entry", func(m *Manifest) { m.Entry = "../x.js" }},
		{"bad capability", func(m *Manifest) { m.Capabilities = []string{"bogus"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Manifest{ID: "hello", Name: "n", Version: "0.1.0", UI: UISpec{Type: "list", Title: "t"}}
			tt.mod(&m)
			if err := m.Validate(); err == nil {
				t.Errorf("expected error")
			}
		})
	}
}
func TestLoadManifestMissingFile(t *testing.T) {
	dir := t.TempDir()
	if _, err := LoadManifest(dir); err == nil {
		t.Errorf("expected error for missing plugin.json")
	}
}
func TestLoadManifestInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte("{ not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(dir); err == nil {
		t.Errorf("expected error for invalid json")
	}
}
func TestLoadManifestValid(t *testing.T) {
	dir := t.TempDir()
	writePlugin(t, dir, Manifest{ID: "hello", Name: "Hi", Version: "0.1.0"}, "")
	m, err := LoadManifest(dir)
	if err != nil {
		t.Fatal(err)
	}
	if m.ID != "hello" {
		t.Errorf("got %q", m.ID)
	}
	if m.Entry != "index.js" {
		t.Errorf("default entry should be index.js, got %q", m.Entry)
	}
}
