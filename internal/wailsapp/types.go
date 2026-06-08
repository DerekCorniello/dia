package wailsapp

// WorkspaceInfo is a one-line summary of a discovered workspace,
// suitable for the workspace list in the UI. The full app list lives
// in WorkspaceDetail.
type WorkspaceInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Apps        int      `json:"apps"`
	Source      string   `json:"source"`
	Path        string   `json:"path"`
	Running     bool     `json:"running"`
	Plugins     []string `json:"plugins,omitempty"`
	UseCount    int      `json:"useCount,omitempty"`
}

// AppInfo is the launch summary of one entry in a workspace's apps
// list. Cmd is the executable (or URL for type=open/browser); Args is
// the joined argument string.
type AppInfo struct {
	Type string `json:"type"`
	Cmd  string `json:"cmd"`
	Args string `json:"args"`
	URL  string `json:"url,omitempty"`
}

// WorkspaceDetail is everything the UI needs to render a workspace
// card and its apps, plus whether it is currently running.
type WorkspaceDetail struct {
	WorkspaceInfo
	AppDetails []AppInfo `json:"app_details"`
}

// InstanceInfo is the UI-friendly view of a running workspace. It
// is intentionally a separate type from state.Instance so the
// internal state shape can change without breaking the JS bindings.
// StartedAt is a time.Time that wails marshals as an RFC 3339
// string on the JS side (the binding generator only knows Go
// primitives and structs).
type InstanceInfo struct {
	ID            string        `json:"id"`
	Workspace     string        `json:"workspace"`
	WorkspacePath string        `json:"workspace_path"`
	StartedAt     string        `json:"started_at"`
	Status        string        `json:"status"`
	Apps          []ProcessInfo `json:"apps"`
	Plugins       []string      `json:"plugins,omitempty"`
}

// ProcessInfo is the UI-friendly view of one running app process.
type ProcessInfo struct {
	Type   string `json:"type"`
	Cmd    string `json:"cmd"`
	PID    int    `json:"pid"`
	Status string `json:"status"`
	Err    string `json:"err,omitempty"`
}

// ReconcileInfo summarizes what Reconcile did so the UI can show
// "dropped N stale entries".
type ReconcileInfo struct {
	Reconciled int `json:"reconciled"`
	Remaining  int `json:"remaining"`
	Total      int `json:"total"`
}

// CheckInfo is one row of the doctor output.
type CheckInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// PathsInfo exposes the on-disk locations dia uses, so the UI can
// show them in a "settings" panel and offer "open in finder".
type PathsInfo struct {
	GlobalConfigDir string `json:"global_config_dir"`
	StateDir        string `json:"state_dir"`
	StateFile       string `json:"state_file"`
}

// CustomThemeInfo is the UI-friendly view of a user-defined theme.
// It mirrors state.CustomTheme but is a separate type so the on-disk
// shape can evolve without breaking the wails JS bindings.
type CustomThemeInfo struct {
	Name        string            `json:"name"`
	ColorScheme string            `json:"color_scheme"`
	Colors      map[string]string `json:"colors"`
}

// PluginInfo is the UI-friendly view of one installed plugin.
// Source is "global" (under the dia state dir) or "local" (under
// <cwd>/.dia/plugins). GrantedCapabilities is the set the plugin
// currently has access to; the frontend highlights differences vs.
// the manifest's requested set in the UI.
type PluginInfo struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Version             string            `json:"version"`
	Description         string            `json:"description"`
	LongDescription     string            `json:"longDescription,omitempty"`
	Author              string            `json:"author"`
	Source              string            `json:"source"`
	Dir                 string            `json:"dir"`
	Enabled             bool              `json:"enabled"`
	Status              string            `json:"status"`
	LastError           string            `json:"lastError,omitempty"`
	GrantedCapabilities []string          `json:"grantedCapabilities"`
	UI                  PluginUIInfo      `json:"ui"`
	Actions             []PluginActionDef `json:"actions"`
	ConfigSchema        map[string]any    `json:"configSchema,omitempty"`
}

// PluginUIInfo describes how the host should render a plugin's panel.
type PluginUIInfo struct {
	Type        string           `json:"type"`
	Title       string           `json:"title"`
	Refreshable bool             `json:"refreshable"`
	Columns     []PluginUIColumn `json:"columns,omitempty"`
}

// PluginUIColumn describes one column of a "table" plugin UI.
type PluginUIColumn struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Format string `json:"format,omitempty"`
}

// PluginActionDef is one button in a plugin panel.
type PluginActionDef struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Confirm    bool   `json:"confirm"`
	Capability string `json:"capability"`
}

// PluginPathsInfo is the on-disk location of the global and local
// plugin directories, so the UI can show paths and "open folder"
// buttons in settings.
type PluginPathsInfo struct {
	GlobalDir string `json:"globalDir"`
	LocalDir  string `json:"localDir,omitempty"`
}

// ToolCategory is a group of detected tools (editors, browsers, etc.)
type ToolCategory struct {
	Name  string         `json:"name"`
	Tools []DetectedTool `json:"tools"`
}

// DetectedTool is a tool found on the system that the user can
// quick-add to a workspace app list.
type DetectedTool struct {
	Label   string `json:"label"`
	Command string `json:"command"`
	Url     string `json:"url"`
}

// AppEditor is the editable view of one workspace app for the
// form-based workspace editor.
type AppEditor struct {
	Label   string `json:"label"`
	Cmd     string `json:"cmd"`
	Cwd     string `json:"cwd"`
	Url     string `json:"url"`
	TermCmd string `json:"termCmd"`
}

// WorkspaceEditor is the editable view of an entire workspace.
// OriginalName is the name at the time the editor was opened, used
// to detect renames so the old file can be removed on save.
// DefaultCwd is the detected current working directory.
type WorkspaceEditor struct {
	OriginalName string            `json:"originalName"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	DefaultCwd   string            `json:"defaultCwd"`
	Apps         []AppEditor       `json:"apps"`
	Plugins      []PluginRefEditor `json:"plugins"`
}

// PluginRefEditor is a plugin reference with config in the editor.
type PluginRefEditor struct {
	ID     string         `json:"id"`
	Config map[string]any `json:"config"`
}
