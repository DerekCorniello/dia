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
}

// TestSetCustomTheme_RoundTrip verifies a custom theme is persisted
// in the state store and shows up in ListCustomThemes.
func TestSetCustomTheme_RoundTrip(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())

	info := CustomThemeInfo{
		Name:        "midnight-rose",
		ColorScheme: "dark",
		Colors: map[string]string{
			"primary":      "#bd93f9",
			"secondary":    "#ff79c6",
			"accent":       "#50fa7b",
			"base_100":     "#1a1b26",
			"base_200":     "#16161e",
			"base_300":     "#101014",
			"base_content": "#c0caf5",
		},
	}
	if err := a.SetCustomTheme(info); err != nil {
		t.Fatalf("SetCustomTheme: %v", err)
	}

	list := a.ListCustomThemes()
	if len(list) != 1 {
		t.Fatalf("ListCustomThemes len = %d, want 1", len(list))
	}
	if list[0].Name != info.Name {
		t.Errorf("Name = %q, want %q", list[0].Name, info.Name)
	}
	if list[0].Colors["primary"] != "#bd93f9" {
		t.Errorf("Colors[primary] = %q, want #bd93f9", list[0].Colors["primary"])
	}
	if list[0].ColorScheme != "dark" {
		t.Errorf("ColorScheme = %q, want dark", list[0].ColorScheme)
	}
}

// TestSetCustomTheme_RejectsBadName verifies the name validator
// catches unsafe names.
func TestSetCustomTheme_RejectsBadName(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	for _, bad := range []string{"", "has space", "with/slash", "very-long-name-that-is-clearly-beyond-the-sixty-four-character-limit-and-should-fail"} {
		if err := a.SetCustomTheme(CustomThemeInfo{
			Name:        bad,
			ColorScheme: "dark",
			Colors:      map[string]string{"primary": "#000000"},
		}); err == nil {
			t.Errorf("SetCustomTheme(%q) = nil err, want error", bad)
		}
	}
}

// TestSetCustomTheme_RejectsBadColorScheme verifies only the two
// daisyUI color schemes are accepted.
func TestSetCustomTheme_RejectsBadColorScheme(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	for _, bad := range []string{"", "auto", "system", "DARK"} {
		if err := a.SetCustomTheme(CustomThemeInfo{
			Name:        "ok",
			ColorScheme: bad,
			Colors:      map[string]string{"primary": "#000000"},
		}); err == nil {
			t.Errorf("SetCustomTheme(color_scheme=%q) = nil err, want error", bad)
		}
	}
}

// TestSetCustomTheme_RejectsBadHexAndSlot verifies malformed hex
// values and unknown color slots are rejected before persistence.
func TestSetCustomTheme_RejectsBadHexAndSlot(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	if err := a.SetCustomTheme(CustomThemeInfo{
		Name:        "ok",
		ColorScheme: "dark",
		Colors:      map[string]string{"primary": "not-a-hex"},
	}); err == nil {
		t.Error("expected error on bad hex")
	}
	if err := a.SetCustomTheme(CustomThemeInfo{
		Name:        "ok",
		ColorScheme: "dark",
		Colors:      map[string]string{"made_up_slot": "#000000"},
	}); err == nil {
		t.Error("expected error on unknown slot")
	}
	if err := a.SetCustomTheme(CustomThemeInfo{
		Name:        "ok",
		ColorScheme: "dark",
		Colors:      map[string]string{},
	}); err == nil {
		t.Error("expected error on empty colors")
	}
}

// TestDeleteCustomTheme verifies deletion works and a missing name
// is a no-op (not an error).
func TestDeleteCustomTheme(t *testing.T) {
	withTempXDG(t)
	a := New()
	a.Startup(testCtx())
	if err := a.SetCustomTheme(CustomThemeInfo{
		Name:        "x",
		ColorScheme: "dark",
		Colors:      map[string]string{"primary": "#000000"},
	}); err != nil {
		t.Fatalf("SetCustomTheme: %v", err)
	}
	if err := a.DeleteCustomTheme("x"); err != nil {
		t.Fatalf("DeleteCustomTheme: %v", err)
	}
	if got := a.ListCustomThemes(); len(got) != 0 {
		t.Errorf("after delete, len = %d, want 0", len(got))
	}
	// Deleting a non-existent name should be a no-op, not an error.
	if err := a.DeleteCustomTheme("nope"); err != nil {
		t.Errorf("DeleteCustomTheme missing: %v", err)
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
