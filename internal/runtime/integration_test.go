//go:build !windows

package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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

func TestIntegration_StopKillsProcessGroup(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("integration test only runs on linux/darwin")
	}
	st, err := state.OpenAt(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	rt := New(Options{Platform: platform.New(), Store: st})
	childFile := filepath.Join(t.TempDir(), "child.pid")
	w := &config.Workspace{
		Name: "group",
		Apps: []config.App{{Type: "local", Cmd: "sh", Args: []string{"-c", "sleep 60 & echo $! > " + childFile + "; wait"}}},
	}
	inst, err := rt.Start(w, config.Source{Path: filepath.Join(t.TempDir(), "group.yaml")})
	if err != nil {
		t.Fatal(err)
	}
	var childPID int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if b, readErr := os.ReadFile(childFile); readErr == nil {
			childPID, _ = strconv.Atoi(strings.TrimSpace(string(b)))
			if childPID > 0 {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if childPID == 0 {
		t.Fatal("child process did not start")
	}
	if err := rt.Stop(inst.ID, true); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		running, _ := rt.pf.IsRunning(childPID)
		if !running {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child process %d survived process-group stop", childPID)
}
