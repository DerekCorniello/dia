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
	if v := info.Main.Version; v != "" && v != "(devel)" {
		Version = v
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			Commit = s.Value
		case "vcs.time":
			BuildTime = s.Value
		}
	}
}
