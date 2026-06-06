// Package config defines the workspace schema, loads workspace YAML files,
// validates them, and discovers workspaces from global and project-local
// locations.
package config

// SchemaVersion is the config schema major version dia understands.
// Configs with a newer major version are rejected at load time.
const SchemaVersion = 1

// Workspace is the declarative definition of a dev environment. One
// workspace per YAML file. Resolution to launchable apps happens in
// internal/apps (Phase 3).
type Workspace struct {
	Version int    `yaml:"version"`
	Name    string `yaml:"name"`
	Apps    []App  `yaml:"apps"`
}

// App is a single component of a workspace. The Type field picks a
// built-in launcher or a gh CLI wrapper. Cmd and Url are mutually
// exclusive entry points; the runtime picks based on Type.
type App struct {
	Type string            `yaml:"type,omitempty"`
	Cmd  string            `yaml:"cmd,omitempty"`
	Args []string          `yaml:"args,omitempty"`
	Cwd  string            `yaml:"cwd,omitempty"`
	Env  map[string]string `yaml:"env,omitempty"`
	Url  string            `yaml:"url,omitempty"`
	Open bool              `yaml:"open,omitempty"`
	Wait bool              `yaml:"wait,omitempty"`
}
