//go:build windows

package state

func lockOwnerAlive(int) bool { return true }
