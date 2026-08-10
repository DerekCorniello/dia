package cli

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/DerekCorniello/dia/internal/daemon"
)

// startCLIDaemon boots an in-process daemon server on the state dir
// derived from the XDG state home, so lifecycle verbs in the test can
// dial it over the socket instead of spawning the (test) binary. t
// cleanup shuts it down and stops whatever it launched.
func startCLIDaemon(t *testing.T, stateHome string) {
	t.Helper()
	stateDir := filepath.Join(stateHome, "dia")
	srv, err := daemon.NewServer(daemon.Options{
		StateDir: stateDir,
		Logger:   slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	if err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = srv.Serve()
		close(done)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		c, err := daemon.Dial(stateDir)
		if err == nil {
			_ = c.Close()
			break
		}
		if time.Now().After(deadline) {
			srv.Close()
			<-done
			t.Fatalf("daemon socket never came up: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() {
		srv.Close()
		<-done
	})
}
