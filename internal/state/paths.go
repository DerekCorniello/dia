package state

import (
	"errors"
	"os"
	"path/filepath"
)

// AppName is the dia subdirectory under the XDG state home.
const AppName = "dia"

// StateFile is the JSON file holding the persisted state.
const StateFile = "state.json"

// LogFile is the log file written under the state dir.
const LogFile = "dia.log"

// ResolveStateDir returns the absolute state directory for dia, creating
// it if necessary. Honors $XDG_STATE_HOME; falls back to ~/.local/state.
func ResolveStateDir() (string, error) {
	return ResolveStateDirAt(defaultStateHome())
}

// ResolveStateDirAt resolves the state dir given an explicit XDG state
// home. The returned path is created if missing.
func ResolveStateDirAt(stateHome string) (string, error) {
	if stateHome == "" {
		return "", errors.New("state home is empty")
	}
	dir := filepath.Join(stateHome, AppName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func defaultStateHome() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), AppName)
	}
	return filepath.Join(home, ".local", "state")
}

// FilePath returns the absolute path to the state JSON file.
func FilePath() (string, error) {
	dir, err := ResolveStateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, StateFile), nil
}
