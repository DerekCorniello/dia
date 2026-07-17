package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExamplePluginsValidate loads every plugin.json under examples/
// and checks it validates. The example plugins are documentation as
// much as code -- a broken one would silently mislead whoever copies
// it as a starting point -- and this is the only thing that actually
// exercises them.
func TestExamplePluginsValidate(t *testing.T) {
	root := filepath.Join("..", "..", "examples")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read examples dir: %v", err)
	}
	found := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(dir, "plugin.json")); err != nil {
			continue
		}
		found++
		if _, err := LoadManifest(dir); err != nil {
			t.Errorf("%s: %v", e.Name(), err)
		}
	}
	if found == 0 {
		t.Fatal("no example plugins found under examples/")
	}
}
