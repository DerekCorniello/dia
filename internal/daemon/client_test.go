package daemon

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// A broken spawn guard must fail the regression without recursively
	// running the suite in the child it launches.
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		os.Exit(91)
	}
	os.Exit(m.Run())
}

func TestEnsureRefusesTestBinary(t *testing.T) {
	c, err := Ensure(EnsureOpts{StateDir: t.TempDir()})
	if c != nil {
		_ = c.Close()
		t.Fatal("unexpected daemon connection")
	}
	if !errors.Is(err, ErrNoDaemon) || !strings.Contains(err.Error(), "refusing to spawn a test binary") {
		t.Fatalf("Ensure should refuse to relaunch the test binary, got %v", err)
	}
}

func TestEnsureAttachesDuringTest(t *testing.T) {
	dir := t.TempDir()
	startServer(t, dir)
	c, err := Ensure(EnsureOpts{StateDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
}
