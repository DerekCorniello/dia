//go:build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// winProcess is the ProcessHandle for the Windows implementation.
type winProcess struct {
	cmd  *exec.Cmd
	pid  int
	done chan struct{}
}

func (p *winProcess) PID() int              { return p.pid }
func (p *winProcess) Done() <-chan struct{} { return p.done }

type winPlatform struct{}

func newWinPlatform() Platform { return &winPlatform{} }

func (winPlatform) Launch(opts LaunchOpts) (ProcessHandle, error) {
	if opts.Cmd == "" {
		return nil, fmt.Errorf("launch: empty command")
	}
	cmd := exec.Command(opts.Cmd, opts.Args...)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}
	// CREATE_NEW_PROCESS_GROUP so Ctrl+C in dia does not
	// propagate to the child, and so taskkill /T reaches the
	// tree reliably.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch %s: %w", opts.Cmd, err)
	}
	done := make(chan struct{})
	pid := cmd.Process.Pid
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	return &winProcess{cmd: cmd, pid: pid, done: done}, nil
}

func (winPlatform) IsRunning(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	// tasklist filters by PID and prints CSV. /NH suppresses the
	// header. We look for the PID in the output to decide.
	out, err := exec.Command(
		"tasklist",
		"/FI", fmt.Sprintf("PID eq %d", pid),
		"/NH", "/FO", "CSV",
	).Output()
	if err != nil {
		return false, err
	}
	return strings.Contains(string(out), strconv.Itoa(pid)), nil
}

func (winPlatform) Kill(pid int, force bool) error {
	if pid <= 0 {
		return nil
	}
	// /T kills the process tree; /F forces. We always force on
	// the second call (not exposed to Platform callers; force
	// means skip the grace path entirely).
	args := []string{"/T", "/PID", strconv.Itoa(pid)}
	if force {
		args = append([]string{"/F"}, args...)
	}
	cmd := exec.Command("taskkill", args...)
	if err := cmd.Run(); err != nil {
		// Exit code 128 + signal usually means the process is
		// already gone; treat that as success.
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 128 {
			return nil
		}
		return fmt.Errorf("taskkill %d: %w", pid, err)
	}
	return nil
}

func (winPlatform) OpenURL(url string) error {
	// The leading empty "" is the window title; required so cmd
	// does not try to interpret the URL as the title.
	return runDetached("cmd", "/c", "start", "", url)
}

func (winPlatform) RevealInFileManager(path string) error {
	return runDetached("explorer", path)
}

func runDetached(prog string, args ...string) error {
	cmd := exec.Command(prog, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", prog, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
