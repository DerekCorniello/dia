package version

import "runtime/debug"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	applyBuildInfo(info)
}

// applyBuildInfo fills Version, Commit, and BuildTime from Go's
// module/VCS build info, but only for fields still at their zero-value
// default. An explicit -ldflags -X override must always win: it is the
// deliberate value set by `make build` and the release workflow, while
// build info is Go's generic fallback for a plain `go install
// pkg@version`, which never gets ldflags at all. Go stamps a
// pseudo-version onto the main module for any build run inside a VCS
// checkout, including a plain `go build` with no ldflags, so build info
// is never actually absent in practice; without this guard it silently
// overwrites the real version on every local and release build.
func applyBuildInfo(info *debug.BuildInfo) {
	if Version == "dev" {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			Version = v
		}
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if Commit == "unknown" {
				Commit = s.Value
			}
		case "vcs.time":
			if BuildTime == "unknown" {
				BuildTime = s.Value
			}
		}
	}
}
