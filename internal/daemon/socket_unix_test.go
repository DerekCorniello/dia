//go:build linux || darwin

package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

// shortTempDir returns a temp dir under /tmp instead of the OS default.
// t.TempDir on macOS lives under /var/folders with a long per-test
// name, which blows past the 104-byte sun_path limit for unix sockets.
func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "diasock")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func TestListenSocketProtectsAndRefusesLiveSocket(t *testing.T) {
	path := filepath.Join(shortTempDir(t), "serve.sock")
	ln, err := listenSocket(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("socket mode = %o, want 600", got)
	}
	if _, err := listenSocket(path); err == nil {
		t.Fatal("second listener unexpectedly succeeded")
	}
}
