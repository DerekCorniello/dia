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
	Hooks       *Hooks      `yaml:"hooks,omitempty"`
}

// Hooks are shell-style commands run around a workspace's lifecycle.
// Each phase runs its commands in order, one at a time, and blocks
// until the command exits. A non-zero exit aborts the phase: for
// pre_start that means the workspace does not start at all, which is
// the point (bringing up infrastructure that the apps need).
//
// This is deliberately not a dependency graph. The case that actually
// comes up is "run this before/after everything", and per-app
// ordering with health checks is a different, much larger feature.
type Hooks struct {
	PreStart  []string `yaml:"pre_start,omitempty"`
	PostStart []string `yaml:"post_start,omitempty"`
	PreStop   []string `yaml:"pre_stop,omitempty"`
	PostStop  []string `yaml:"post_stop,omitempty"`
}

// Phases returns the hook phases in a fixed order alongside the field
// name each came from, so validation and diagnostics can report a
// precise path without repeating the phase list.
func (h *Hooks) Phases() []HookPhase {
	if h == nil {
		return nil
	}
	return []HookPhase{
		{Name: "pre_start", Cmds: h.PreStart},
		{Name: "post_start", Cmds: h.PostStart},
		{Name: "pre_stop", Cmds: h.PreStop},
		{Name: "post_stop", Cmds: h.PostStop},
	}
}

// HookPhase is one named group of hook commands.
type HookPhase struct {
	Name string
	Cmds []string
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
}
