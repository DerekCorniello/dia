package version

import (
	"runtime/debug"
	"testing"
)

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

// snapshot saves the package vars and returns a func that restores
// them, so tests can mutate Version/Commit/BuildTime without leaking
// state into other tests.
func snapshot() func() {
	v, c, b := Version, Commit, BuildTime
	return func() { Version, Commit, BuildTime = v, c, b }
}

// TestApplyBuildInfoPrefersLdflags pins the bug that made every
// release binary report the wrong version: go build stamps a
// pseudo-version onto the main module for any build run inside a VCS
// checkout, even a plain `go build` with no ldflags, so build info is
// never actually empty. It must not be allowed to clobber a value
// -ldflags -X already set.
func TestApplyBuildInfoPrefersLdflags(t *testing.T) {
	defer snapshot()()
	Version = "v0.3.0"
	Commit = "abc1234"
	BuildTime = "2026-01-01T00:00:00Z"

	applyBuildInfo(&debug.BuildInfo{
		Main: debug.Module{Version: "v0.0.0-20260717000000-deadbeefdead+dirty"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"},
			{Key: "vcs.time", Value: "2026-07-17T00:00:00Z"},
		},
	})

	if Version != "v0.3.0" {
		t.Errorf("Version = %q, want ldflags value preserved", Version)
	}
	if Commit != "abc1234" {
		t.Errorf("Commit = %q, want ldflags value preserved", Commit)
	}
	if BuildTime != "2026-01-01T00:00:00Z" {
		t.Errorf("BuildTime = %q, want ldflags value preserved", BuildTime)
	}
}

// TestApplyBuildInfoFallsBackWithoutLdflags covers `go install
// pkg@version`, which never runs with -ldflags, so build info is the
// only source of a real version string.
func TestApplyBuildInfoFallsBackWithoutLdflags(t *testing.T) {
	defer snapshot()()
	Version, Commit, BuildTime = "dev", "unknown", "unknown"

	applyBuildInfo(&debug.BuildInfo{
		Main: debug.Module{Version: "v0.2.5"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "cafebabe"},
			{Key: "vcs.time", Value: "2026-05-01T00:00:00Z"},
		},
	})

	if Version != "v0.2.5" {
		t.Errorf("Version = %q, want v0.2.5", Version)
	}
	if Commit != "cafebabe" {
		t.Errorf("Commit = %q, want cafebabe", Commit)
	}
	if BuildTime != "2026-05-01T00:00:00Z" {
		t.Errorf("BuildTime = %q, want 2026-05-01T00:00:00Z", BuildTime)
	}
}

// TestApplyBuildInfoIgnoresDevelVersion covers building outside any
// resolved module version, where Main.Version is the literal string
// "(devel)"; that is not a usable version and must not override the
// "dev" default.
func TestApplyBuildInfoIgnoresDevelVersion(t *testing.T) {
	defer snapshot()()
	Version = "dev"

	applyBuildInfo(&debug.BuildInfo{Main: debug.Module{Version: "(devel)"}})

	if Version != "dev" {
		t.Errorf("Version = %q, want dev", Version)
	}
}
