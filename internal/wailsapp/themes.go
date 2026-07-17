package wailsapp

import (
	"errors"
	"fmt"

	"github.com/DerekCorniello/dia/internal/state"
)

// GetTheme returns the current UI theme name from persisted state.
func (a *App) GetTheme() string {
	if a.store == nil {
		return state.DefaultTheme
	}
	t := a.store.Snapshot().Theme
	if t == "" {
		return state.DefaultTheme
	}
	return t
}

// SetTheme persists the UI theme name so it survives restart.
func (a *App) SetTheme(theme string) error {
	if a.store == nil {
		return errors.New("state store not initialized")
	}
	return a.store.Mutate(func(d *state.Data) {
		d.Theme = theme
	})
}

// ListCustomThemes returns every user-defined theme. The frontend
// merges this list with the built-in daisyUI themes at render time.
func (a *App) ListCustomThemes() []CustomThemeInfo {
	if a.store == nil {
		return nil
	}
	d := a.store.Snapshot()
	out := make([]CustomThemeInfo, 0, len(d.CustomThemes))
	for name, t := range d.CustomThemes {
		colors := make(map[string]string, len(t.Colors))
		for k, v := range t.Colors {
			colors[k] = v
		}
		out = append(out, CustomThemeInfo{
			Name:        name,
			ColorScheme: t.ColorScheme,
			Colors:      colors,
		})
	}
	return out
}

// SetCustomTheme upserts a user-defined theme by name. An empty
// color_scheme or zero colors returns an error; the name must be
// non-empty and contain only ASCII letters, digits, hyphen, and
// underscore.
func (a *App) SetCustomTheme(info CustomThemeInfo) error {
	if a.store == nil {
		return errors.New("state store not initialized")
	}
	if err := validateThemeName(info.Name); err != nil {
		return err
	}
	if info.ColorScheme != "light" && info.ColorScheme != "dark" {
		return fmt.Errorf("color_scheme must be \"light\" or \"dark\", got %q", info.ColorScheme)
	}
	if len(info.Colors) == 0 {
		return errors.New("colors must not be empty")
	}
	for k, v := range info.Colors {
		if !isValidColorKey(k) {
			return fmt.Errorf("unknown color slot %q", k)
		}
		if !isValidHex(v) {
			return fmt.Errorf("color %s = %q is not a valid #rrggbb hex string", k, v)
		}
	}
	return a.store.Mutate(func(d *state.Data) {
		colors := make(map[string]string, len(info.Colors))
		for k, v := range info.Colors {
			colors[k] = v
		}
		d.CustomThemes[info.Name] = state.CustomTheme{
			ColorScheme: info.ColorScheme,
			Colors:      colors,
		}
	})
}

// DeleteCustomTheme removes a user-defined theme. Built-in themes
// are not stored in state and cannot be deleted; this is a no-op
// for names that are not in the custom map.
func (a *App) DeleteCustomTheme(name string) error {
	if a.store == nil {
		return errors.New("state store not initialized")
	}
	if err := validateThemeName(name); err != nil {
		return err
	}
	return a.store.Mutate(func(d *state.Data) {
		delete(d.CustomThemes, name)
	})
}

func validateThemeName(name string) error {
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) > 64 {
		return errors.New("name must be 64 characters or fewer")
	}
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return fmt.Errorf("name %q must match [A-Za-z0-9_-]+", name)
		}
	}
	return nil
}

// isValidColorKey checks that a color slot is one of the daisyUI
// v4 semantic colors we surface in the editor.
func isValidColorKey(k string) bool {
	switch k {
	case "primary", "primary_content",
		"secondary", "secondary_content",
		"accent", "accent_content",
		"neutral", "neutral_content",
		"base_100", "base_200", "base_300", "base_content",
		"info", "success", "warning", "error":
		return true
	}
	return false
}

func isValidHex(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
