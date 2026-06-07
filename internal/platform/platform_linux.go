//go:build linux

package platform

import (
	"fmt"
	"os/exec"
	"syscall"
)

func (unixPlatform) OpenURL(url string) error {
	return runDetached("xdg-open", url)
}

func revealImpl(path string) error {
	return runDetached("xdg-open", path)
}

func openFileImpl(path string) error {
	return runDetached("xdg-open", path)
}

func runDetached(prog string, args ...string) error {
	cmd := exec.Command(prog, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", prog, err)
	}
	// Detach: release immediately so we don't wait on the helper.
	go func() { _ = cmd.Wait() }()
	return nil
}
