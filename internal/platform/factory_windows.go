//go:build windows

package platform

func newForOS() Platform { return newWinPlatform() }
