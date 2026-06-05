package version

import "testing"

func TestDefaults(t *testing.T) {
	if Version == "" {
		t.Fatal("Version should not be empty")
	}
	if Commit == "" {
		t.Fatal("Commit should not be empty")
	}
	if BuildTime == "" {
		t.Fatal("BuildTime should not be empty")
	}
}
