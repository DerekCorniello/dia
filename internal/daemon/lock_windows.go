//go:build windows

package daemon

func daemonLockOwnerAlive(int) bool { return true }
