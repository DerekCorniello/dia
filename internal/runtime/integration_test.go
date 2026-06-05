//go:build !windows

package runtime

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/DerekCorniello/dia/internal/config"
	"github.com/DerekCorniello/dia/internal/platform"
	"github.com/DerekCorniello/dia/internal/state"
)

func TestIntegration_StartStopRealProcess(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("integration test only runs on linux/darwin")
	}
	st, err := state.OpenAt(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("state.OpenAt: %v", err)
	}
	rt := New(Options{Platform: platform.New(), Store: st})

	// `sleep 60` is a benign long-running binary on every unix.
	sleepPath, err := exec.LookPath("sleep")
	if err != nil {
		t.Skipf("sleep not found: %v", err)
	}
	w := &config.Workspace{
		Name: "integ",
		Apps: []config.App{
			{Type: "local", Cmd: sleepPath, Args: []string{"60"}, Cwd: t.TempDir()},
		},
	}
	inst, err := rt.Start(w, config.Source{Path: filepath.Join(t.TempDir(), "x.yaml")})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if inst.Apps[0].PID <= 0 {
		t.Fatalf("PID not set")
	}
	running, err := rt.pf.IsRunning(inst.Apps[0].PID)
	if err != nil {
		t.Fatalf("IsRunning: %v", err)
	}
	if !running {
		t.Fatalf("process %d not running after Start", inst.Apps[0].PID)
	}
	if err := rt.Stop(inst.ID, true); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r, _ := rt.pf.IsRunning(inst.Apps[0].PID)
		if !r {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Errorf("process %d still running 2s after forced stop", inst.Apps[0].PID)
}
