package daemon

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DerekCorniello/dia/internal/state"
)

func writeWorkspace(t *testing.T, dir, name string) string {
	t.Helper()
	w := `version: 1
name: ` + name + `
apps:
  - type: service
    cmd: sleep
    args: ["60"]
`
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, []byte(w), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func startServer(t *testing.T, stateDir string) *Server {
	t.Helper()
	srv, err := NewServer(Options{StateDir: stateDir})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		done <- srv.Serve()
	}()
	// Serve binds the socket before accepting; poll until it is up so
	// a racing Dial cannot miss it.
	path := socketPath(stateDir)
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := dialSocket(path)
		if err == nil {
			_ = conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server socket never came up: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Cleanup(func() { srv.Close(); <-done })
	return srv
}

func dialClient(t *testing.T, stateDir string) *Client {
	t.Helper()
	c, err := Dial(stateDir)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestServerStartAttachStop(t *testing.T) {
	stateDir := t.TempDir()
	wsPath := writeWorkspace(t, t.TempDir(), "dev")
	startServer(t, stateDir)
	c := dialClient(t, stateDir)

	var got struct {
		Version string `json:"version"`
	}
	if err := c.Do(MethodVersion, nil, &got); err != nil {
		t.Fatalf("version: %v", err)
	}
	if got.Version == "" {
		t.Fatal("empty version reply")
	}

	var first StartReply
	if err := c.Do(MethodStart, StartParams{Name: "dev", Path: wsPath}, &first); err != nil {
		t.Fatalf("start: %v", err)
	}
	if first.Attached {
		t.Fatal("first start reported attached")
	}
	if first.Instance == nil || first.Instance.ID == "" {
		t.Fatal("start returned no instance")
	}
	if first.Instance.Status != state.StatusRunning {
		t.Fatalf("status = %q, want running", first.Instance.Status)
	}
	if len(first.Instance.Apps) == 0 || first.Instance.Apps[0].PID <= 0 {
		t.Fatalf("expected a running app with pid, got %+v", first.Instance.Apps)
	}

	var second StartReply
	if err := c.Do(MethodStart, StartParams{Name: "dev", Path: wsPath}, &second); err != nil {
		t.Fatalf("second start: %v", err)
	}
	if !second.Attached {
		t.Fatal("second start should attach to the running instance")
	}
	if second.Instance == nil || second.Instance.ID != first.Instance.ID {
		t.Fatalf("attach returned different instance: %+v vs %+v", second.Instance, first.Instance)
	}

	var stop struct {
		Stopped []string `json:"stopped"`
	}
	if err := c.Do(MethodStop, StopParams{Name: "dev"}, &stop); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if len(stop.Stopped) != 1 || stop.Stopped[0] != first.Instance.ID {
		t.Fatalf("stop ids = %v, want %q", stop.Stopped, first.Instance.ID)
	}

	var instances []state.Instance
	if err := c.Do(MethodList, nil, &instances); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(instances) != 1 {
		t.Fatalf("list length = %d, want 1", len(instances))
	}
	if instances[0].Status != state.StatusStopped {
		t.Fatalf("status after stop = %q, want stopped", instances[0].Status)
	}
}

func TestServerStartNotFound(t *testing.T) {
	stateDir := t.TempDir()
	startServer(t, stateDir)
	c := dialClient(t, stateDir)

	var reply StartReply
	err := c.Do(MethodStart, StartParams{Name: "missing"}, &reply)
	if err == nil {
		t.Fatal("start of unknown workspace should error")
	}
}

func TestServerRestart(t *testing.T) {
	stateDir := t.TempDir()
	wsPath := writeWorkspace(t, t.TempDir(), "dev")
	startServer(t, stateDir)
	c := dialClient(t, stateDir)

	var first StartReply
	if err := c.Do(MethodStart, StartParams{Name: "dev", Path: wsPath}, &first); err != nil {
		t.Fatalf("start: %v", err)
	}
	var again StartReply
	if err := c.Do(MethodRestart, StartParams{Name: "dev", Path: wsPath}, &again); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if again.Instance == nil {
		t.Fatal("restart returned no instance")
	}
	if again.Instance.ID == first.Instance.ID {
		t.Fatal("restart should launch a fresh instance, not reuse the old id")
	}
	var instances []state.Instance
	if err := c.Do(MethodList, nil, &instances); err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, inst := range instances {
		if inst.Status == state.StatusRunning && inst.ID != again.Instance.ID {
			t.Fatalf("unexpected running instance %s after restart", inst.ID)
		}
	}
}

func TestServerShutdown(t *testing.T) {
	stateDir := t.TempDir()
	startServer(t, stateDir)
	c, err := Dial(stateDir)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}

	if err := c.Do(MethodShutdown, nil, &struct{}{}); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	_ = c.Close()

	// Serve may still be draining an in-flight Accept (recreating a
	// pipe instance on Windows), so a dial can succeed once; poll until
	// the listener is fully gone, mirroring TestShutdown in the CLI.
	for i := 0; i < 50; i++ {
		nc, err := Dial(stateDir)
		if err != nil {
			return
		}
		_ = nc.Close()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("daemon still reachable after shutdown")
}
