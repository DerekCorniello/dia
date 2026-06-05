//go:build linux || darwin

package platform

func newForOS() Platform { return newUnixPlatform() }
