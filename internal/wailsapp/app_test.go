package wailsapp

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempXDG sets XDG_CONFIG_HOME and XDG_STATE_HOME to t.TempDir
// for the duration of the test so config/state resolve to a clean
// scratch space.
func withTempXDG(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("XDG_STATE_HOME", tmp)
}

// TestNewWorkspace_WritesFile verifies NewWorkspace creates a
// starter YAML in the global config dir.
func TestNewWorkspace_WritesFile(t *testing.T) {
	withTempXDG(t)
	a := New()
	path, err := a.NewWorkspace("demo")
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
	if !contains(body, "type: editor") {
		t.Errorf("file missing 'type: editor':\n%s", body)
	}
}

// TestNewWorkspace_RefusesExisting verifies a second call with the
// same name returns an error and does not overwrite.
func TestNewWorkspace_RefusesExisting(t *testing.T) {
	withTempXDG(t)
	a := New()
	if _, err := a.NewWorkspace("dup"); err != nil {
		t.Fatalf("first NewWorkspace: %v", err)
	}
	if _, err := a.NewWorkspace("dup"); err == nil {
		t.Fatal("expected error on duplicate name, got nil")
	}
}

// TestNewWorkspace_RejectsBadName verifies the validator runs.
func TestNewWorkspace_RejectsBadName(t *testing.T) {
	withTempXDG(t)
	a := New()
	for _, bad := range []string{"", "Has-Caps", "-leading", "trailing-", "has space", "with/slash"} {
		if _, err := a.NewWorkspace(bad); err == nil {
			t.Errorf("NewWorkspace(%q) = nil err, want error", bad)
		}
	}
}

// TestStartup_BuildsRuntime verifies Startup wires up the runtime
// without panicking. The runtime is reachable via ListInstances,
// which is a no-op before any workspace has been started.
func TestStartup_BuildsRuntime(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	if a.rt == nil {
		t.Fatal("rt not set after Startup")
	}
	if got := a.ListInstances(); len(got) != 0 {
		t.Errorf("ListInstances = %d, want 0", len(got))
	}
	// Plugins is allowed to be nil or an empty slice depending on
	// whether the OS PATH was parsed to anything; both are valid
	// for the contract.
	if got := a.Plugins(); got != nil && len(got) != 0 {
		t.Errorf("Plugins = %v, want nil or empty", got)
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
