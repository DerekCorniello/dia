//go:build linux || darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// unixProcess is the ProcessHandle returned by the Unix Platform.
type unixProcess struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func (p *unixProcess) PID() int { return p.cmd.Process.Pid }

func (p *unixProcess) Done() <-chan struct{} { return p.done }

// unixPlatform implements Platform for Linux and macOS.
type unixPlatform struct{}

func newUnixPlatform() Platform { return &unixPlatform{} }

func (unixPlatform) Launch(opts LaunchOpts) (ProcessHandle, error) {
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
	// Setsid puts the child in a new session and process group,
	// so a kill on the pgid (or the leading pid) reaches the
	// whole tree without taking dia down with it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	// Detach from dia's stdio. The user can still see output via
	// the launching app (terminal, editor, browser).
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("launch %s: %w", opts.Cmd, err)
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	return &unixProcess{cmd: cmd, done: done}, nil
}

func (unixPlatform) IsRunning(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}
	// Signal 0 is the standard "is this process alive?" probe on
	// POSIX. It returns nil if the process exists and we have
	// permission to signal it, EPERM if it exists but we do not,
	// and ESRCH if it does not exist.
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true, nil
	}
	if err == syscall.ESRCH {
		return false, nil
	}
	if err == syscall.EPERM {
		// Exists but we cannot signal it; treat as running so
		// dia does not falsely mark a live process stopped.
		return true, nil
	}
	return false, err
}

func (unixPlatform) Kill(pid int, force bool) error {
	if pid <= 0 {
		return nil
	}
	sig := syscall.SIGTERM
	if force {
		sig = syscall.SIGKILL
	}
	if err := syscall.Kill(pid, sig); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("kill %d: %w", pid, err)
	}
	return nil
}

func (unixPlatform) RevealInFileManager(path string) error {
	// Both Linux and macOS will open the directory if path is a
	// dir, and open the file's containing directory if path is a
	// file. The OS-specific helper in platform_linux.go and
	// platform_darwin.go handles the "open" command.
	return revealImpl(path)
}

func (unixPlatform) OpenFile(path string) error {
	return openFileImpl(path)
}
