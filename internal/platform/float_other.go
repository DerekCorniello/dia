//go:build !linux

package platform

// SetFloating is a no-op off Linux: only the Hyprland path is
// implemented, and window placement stays with the compositor.
func SetFloating(pid int, floating bool) error { return nil }
