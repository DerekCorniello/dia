package diag

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanPlugins_EmptyPath(t *testing.T) {
	t.Setenv("PATH", "")
	got := ScanPlugins()
	if len(got) != 0 {
		t.Errorf("ScanPlugins() = %v, want []", got)
	}
}

func TestScanPlugins_FindsExecutable(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"dia-fake", "dia-ok", "not-dia", "dia-noexec"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	// Non-executable should be skipped on unix.
	if err := os.Chmod(filepath.Join(dir, "dia-noexec"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	got := ScanPlugins()
	want := map[string]bool{
		filepath.Join(dir, "dia-fake"): true,
		filepath.Join(dir, "dia-ok"):   true,
	}
	if len(got) != len(want) {
		t.Fatalf("ScanPlugins() = %v, want %d entries", got, len(want))
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected plugin path: %s", p)
		}
	}
}

func TestPlatformOpenHelper(t *testing.T) {
	got := PlatformOpenHelper()
	switch got {
	case "open", "cmd", "xdg-open":
	default:
		t.Errorf("PlatformOpenHelper() = %q, want open/cmd/xdg-open", got)
	}
}

func TestRunChecks_AllFieldsPopulated(t *testing.T) {
	got := RunChecks("/tmp/state", "/tmp/state/state.json")
	names := map[string]bool{}
	for _, c := range got {
		names[c.Name] = true
		if c.Status != "ok" && c.Status != "warn" && c.Status != "fail" {
			t.Errorf("check %q has bad status %q", c.Name, c.Status)
		}
	}
	for _, want := range []string{"platform", "state dir", "state file", "gh cli", "url handler", "plugins"} {
		if !names[want] {
			t.Errorf("missing check %q", want)
		}
	}
}
