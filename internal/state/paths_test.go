package state

import (
	"path/filepath"
	"testing"
)

func TestResolveStateDirAt(t *testing.T) {
	base := t.TempDir()
	dir, err := ResolveStateDirAt(base)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, AppName)
	if dir != want {
		t.Errorf("got %q, want %q", dir, want)
	}
	if _, err := filepath.Abs(dir); err != nil {
		t.Errorf("expected absolute path: %v", err)
	}
}

func TestResolveStateDirAtRejectsEmpty(t *testing.T) {
	if _, err := ResolveStateDirAt(""); err == nil {
		t.Error("expected error for empty state home")
	}
}

func TestResolveStateDirHonorsXDG(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", base)
	dir, err := ResolveStateDir()
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(base, AppName) {
		t.Errorf("got %q, want suffix %q", dir, AppName)
	}
}
