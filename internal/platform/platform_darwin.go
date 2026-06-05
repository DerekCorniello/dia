//go:build darwin

package platform

import (
	"fmt"
	"os/exec"
	"syscall"
)

func (unixPlatform) OpenURL(url string) error {
	return runDetached("open", url)
}

func revealImpl(path string) error {
	return runDetached("open", path)
}

func runDetached(prog string, args ...string) error {
	cmd := exec.Command(prog, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", prog, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
