package wailsapp

import (
	"errors"

	"github.com/DerekCorniello/dia/internal/state"
)

// GetKeybindings returns the current keybinding overrides.
func (a *App) GetKeybindings() map[string]string {
	if a.store == nil {
		return map[string]string{}
	}
	snap := a.store.Snapshot()
	if snap.Keybindings == nil {
		return map[string]string{}
	}
	return snap.Keybindings
}

// SetKeybinding stores a keybinding override for the given action.
// Pass keys="" to remove the override (restoring the default).
func (a *App) SetKeybinding(action, keys string) error {
	if a.store == nil {
		return errors.New("store not initialized")
	}
	return a.store.Mutate(func(d *state.Data) {
		if d.Keybindings == nil {
			d.Keybindings = map[string]string{}
		}
		if keys == "" {
			delete(d.Keybindings, action)
		} else {
			d.Keybindings[action] = keys
		}
	})
}

// ResetKeybindings removes all keybinding overrides.
func (a *App) ResetKeybindings() error {
	if a.store == nil {
		return errors.New("store not initialized")
	}
	return a.store.Mutate(func(d *state.Data) {
		d.Keybindings = nil
	})
}
