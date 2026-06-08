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
	Version     int         `yaml:"version"`
	Name        string      `yaml:"name"`
	Description string      `yaml:"description,omitempty"`
	Apps        []App       `yaml:"apps"`
	Plugins     []PluginRef `yaml:"plugins,omitempty"`
}

// PluginRef is a reference to an installed plugin by ID. When the
// workspace starts, the listed plugins are enabled; when it stops
// they are disabled. Config is an optional workspace-scoped JSON
// object the plugin receives via dia.getConfig().
type PluginRef struct {
	ID     string         `yaml:"id"`
	Config map[string]any `yaml:"config,omitempty"`
}

// App is a single component of a workspace. The Type field picks a
// built-in launcher or a gh CLI wrapper. Cmd and Url are mutually
// exclusive entry points; the runtime picks based on Type.
type App struct {
	Type  string            `yaml:"type,omitempty"`
	Label string            `yaml:"label,omitempty"`
	Cmd   string            `yaml:"cmd,omitempty"`
	Args  []string          `yaml:"args,omitempty"`
	Cwd   string            `yaml:"cwd,omitempty"`
	Env   map[string]string `yaml:"env,omitempty"`
	Url   string            `yaml:"url,omitempty"`
	Open  bool              `yaml:"open,omitempty"`
	Wait  bool              `yaml:"wait,omitempty"`
}
