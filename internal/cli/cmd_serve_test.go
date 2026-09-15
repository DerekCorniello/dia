package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenDaemonLogRotatesAndRestrictsPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dia.log")
	old := strings.Repeat("x", maxDaemonLogBytes)
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := openDaemonLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	rotated, err := os.ReadFile(path + ".1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rotated) != len(old) {
		t.Fatalf("rotated log size = %d, want %d", len(rotated), len(old))
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows reports 666 regardless of the 0600 requested at creation;
	// Unix modes are not meaningful there.
	if runtime.GOOS != "windows" {
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("log permissions = %o, want 600", got)
		}
	}
}
