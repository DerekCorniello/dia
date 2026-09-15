//go:build !windows

package daemon

import "syscall"

func daemonLockOwnerAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}
