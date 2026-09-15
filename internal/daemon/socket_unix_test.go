//go:build linux || darwin

package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListenSocketProtectsAndRefusesLiveSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "serve.sock")
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
