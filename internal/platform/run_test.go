//go:build linux || darwin

package platform

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRun_CapturesStdoutAndStderr(t *testing.T) {
	out, err := cmdRunner{}.Run(LaunchOpts{
		Cmd:  "sh",
		Args: []string{"-c", "echo to-stdout; echo to-stderr >&2"},
	}, time.Minute)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "to-stdout") || !strings.Contains(out, "to-stderr") {
		t.Errorf("output %q should contain both streams", out)
	}
}

func TestRun_NonZeroExitReturnsErrorAndOutput(t *testing.T) {
	out, err := cmdRunner{}.Run(LaunchOpts{
		Cmd:  "sh",
		Args: []string{"-c", "echo why-it-failed; exit 3"},
	}, time.Minute)
	if err == nil {
		t.Fatal("expected an error for a non-zero exit")
	}
	// The output is what tells the user what went wrong, so it has to
	// come back even on failure.
	if !strings.Contains(out, "why-it-failed") {
		t.Errorf("output %q should be returned alongside the error", out)
	}
}

func TestRun_TimeoutIsDistinguishable(t *testing.T) {
	_, err := cmdRunner{}.Run(LaunchOpts{
		Cmd:  "sh",
		Args: []string{"-c", "sleep 5"},
	}, 50*time.Millisecond)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if !errors.Is(err, ErrRunTimeout) {
		t.Errorf("error %v should wrap ErrRunTimeout so a hang is distinguishable from a failure", err)
	}
}

func TestRun_RespectsCwd(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := cmdRunner{}.Run(LaunchOpts{Cmd: "ls", Cwd: dir}, time.Minute)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "marker.txt") {
		t.Errorf("output %q should list the temp dir contents", out)
	}
}

func TestRun_AppendsEnv(t *testing.T) {
	out, err := cmdRunner{}.Run(LaunchOpts{
		Cmd:  "sh",
		Args: []string{"-c", "echo $DIA_TEST_VAR"},
		Env:  []string{"DIA_TEST_VAR=hello"},
	}, time.Minute)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.TrimSpace(out) != "hello" {
		t.Errorf("output = %q, want hello", strings.TrimSpace(out))
	}
}

func TestRun_EmptyCommand(t *testing.T) {
	r := cmdRunner{}
	if _, err := r.Run(LaunchOpts{}, time.Minute); err == nil {
		t.Error("expected an error for an empty command")
	}
}

func TestRun_CommandNotFound(t *testing.T) {
	r := cmdRunner{}
	if _, err := r.Run(LaunchOpts{Cmd: "dia-no-such-binary-xyz"}, time.Minute); err == nil {
		t.Error("expected an error for a nonexistent binary")
	}
}

// Run must be reachable through the interface, not just the embedded
// helper: the runtime calls it as pf.Run.
func TestRun_ReachableThroughPlatformInterface(t *testing.T) {
	out, err := New().Run(LaunchOpts{Cmd: "sh", Args: []string{"-c", "echo via-interface"}}, time.Minute)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(out, "via-interface") {
		t.Errorf("output = %q", out)
	}
}
