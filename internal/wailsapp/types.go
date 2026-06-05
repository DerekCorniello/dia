package wailsapp

// WorkspaceInfo is a one-line summary of a discovered workspace,
// suitable for the workspace list in the UI. The full app list lives
// in WorkspaceDetail.
type WorkspaceInfo struct {
	Name    string `json:"name"`
	Apps    int    `json:"apps"`
	Source  string `json:"source"`
	Path    string `json:"path"`
	Running bool   `json:"running"`
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
