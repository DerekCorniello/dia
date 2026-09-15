//go:build linux

package platform

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeHyprctl installs a stub hyprctl on PATH that reports one tiled
// client and records dispatches, so SetFloating can be tested without
// a compositor.
func fakeHyprctl(t *testing.T, pid int) (dir string, dispatched *string) {
	t.Helper()
	dir = t.TempDir()
	var got string
	dispatched = &got
	script := `#!/bin/sh
if [ "$1" = "clients" ]; then
  echo '[{"address":"0xabc","pid":` + itoa(pid) + `,"floating":false}]'
else
  echo "$@" >> "` + filepath.Join(dir, "calls") + `"
fi
`
	if err := os.WriteFile(filepath.Join(dir, "hyprctl"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "test-sig")
	return dir, dispatched
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func TestSetFloatingDispatchesToggle(t *testing.T) {
	dir, _ := fakeHyprctl(t, 4242)
	if err := SetFloating(4242, true); err != nil {
		t.Fatalf("SetFloating: %v", err)
	}
	calls, err := os.ReadFile(filepath.Join(dir, "calls"))
	if err != nil {
		t.Fatalf("hyprctl never dispatched: %v", err)
	}
	if string(calls) != "dispatch togglefloating address:0xabc\n" {
		t.Errorf("unexpected dispatch: %q", calls)
	}
}

func TestSetFloatingNoopWhenAlreadyTiled(t *testing.T) {
	dir, _ := fakeHyprctl(t, 4242)
	if err := SetFloating(4242, false); err != nil {
		t.Fatalf("SetFloating: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "calls")); !os.IsNotExist(err) {
		t.Error("SetFloating dispatched even though the window already tiles")
	}
}

func TestSetFloatingNoopWithoutHyprland(t *testing.T) {
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "")
	if err := SetFloating(4242, true); err != nil {
		t.Errorf("expected no-op without Hyprland, got %v", err)
	}
}
