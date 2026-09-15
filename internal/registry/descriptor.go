package registry

// FieldDescriptor describes one editable field of an app type.
type FieldDescriptor struct {
	Name      string `json:"name"`
	Label     string `json:"label"`
	Type      string `json:"type"`
	Required  bool   `json:"required,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty"`
	Help      string `json:"help,omitempty"`
}

// AppTypeDescriptor describes an app type for the GUI editor and
// summary rendering. It is the single source of truth for which fields
// a type uses; the frontend no longer duplicates builtinAppTypes.
type AppTypeDescriptor struct {
	Type        string            `json:"type"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Fields      []FieldDescriptor `json:"fields"`
	// Summary is a hint for WorkspaceCard rendering.
	Summary string `json:"summary,omitempty"`
}

// Descriptors returns the sorted descriptors for all registered types.
// Built-ins have rich descriptors; plugin-provided types return a
// generic descriptor so the editor never loses a field.
func (r *Registry) Descriptors() []AppTypeDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]AppTypeDescriptor, 0, len(r.handlers))
	for name := range r.handlers {
		if d, ok := builtinDescriptor(name); ok {
			out = append(out, d)
		} else {
			out = append(out, AppTypeDescriptor{
				Type:        name,
				Label:       name,
				Description: "Plugin-provided app type",
				Fields: []FieldDescriptor{
					{Name: "label", Label: "label", Type: "string", Help: "Display name"},
					{Name: "cmd", Label: "command", Type: "string", Help: "Command or entry point"},
					{Name: "args", Label: "arguments", Type: "stringArray", Help: "One per line"},
					{Name: "cwd", Label: "cwd", Type: "string", Help: "Working directory"},
					{Name: "env", Label: "env", Type: "map", Help: "Environment variables"},
					{Name: "url", Label: "url", Type: "string", Help: "URL if applicable"},
				},
			})
		}
	}
	// Ensure empty-type default appears first.
	for i := range out {
		if out[i].Type == "" {
			out[i].Label = "default"
			out[i].Description = "Runs a command (cmd)"
		}
	}
	return out
}

func builtinDescriptor(name string) (AppTypeDescriptor, bool) {
	switch name {
	case "":
		return AppTypeDescriptor{
			Type:        "",
			Label:       "default",
			Description: "Runs a command",
			Fields: []FieldDescriptor{
				{Name: "label", Label: "label", Type: "string"},
				{Name: "cmd", Label: "command", Type: "string", Required: true, Help: "Binary, may include args (e.g. code --wait)"},
				{Name: "args", Label: "arguments", Type: "stringArray", Help: "Additional args, one per line"},
				{Name: "cwd", Label: "cwd", Type: "string"},
				{Name: "env", Label: "env", Type: "map"},
			},
		}, true
	case "local", "terminal", "editor", "service", "custom", "ai":
		return AppTypeDescriptor{
			Type:        name,
			Label:       name,
			Description: "Runs a local command",
			Fields: []FieldDescriptor{
				{Name: "label", Label: "label", Type: "string"},
				{Name: "cmd", Label: "command", Type: "string", Required: true},
				{Name: "args", Label: "arguments", Type: "stringArray"},
				{Name: "cwd", Label: "cwd", Type: "string"},
				{Name: "env", Label: "env", Type: "map"},
			},
			Summary: "cmd + args",
		}, true
	case "open":
		return AppTypeDescriptor{
			Type:        "open",
			Label:       "open",
			Description: "Opens a URL with the OS handler",
			Fields: []FieldDescriptor{
				{Name: "label", Label: "label", Type: "string"},
				{Name: "url", Label: "url", Type: "string", Required: true},
			},
			Summary: "url",
		}, true
	case "browser":
		return AppTypeDescriptor{
			Type:        "browser",
			Label:       "browser",
			Description: "Opens URLs in a browser (dedicated profile when browser is set)",
			Fields: []FieldDescriptor{
				{Name: "label", Label: "label", Type: "string"},
				{Name: "browser", Label: "browser", Type: "string", Help: "Binary e.g. zen-browser, firefox; empty = OS default"},
				{Name: "urls", Label: "urls", Type: "stringArray", Required: true, Help: "One per line, tabs in one window"},
				{Name: "url", Label: "url", Type: "string", Help: "Single URL (alternative to urls)"},
				{Name: "newWindow", Label: "new window", Type: "boolean"},
				{Name: "env", Label: "env", Type: "map"},
				{Name: "cwd", Label: "cwd", Type: "string"},
			},
			Summary: "browser + urls",
		}, true
	case "gh":
		return AppTypeDescriptor{
			Type:        "gh",
			Label:       "gh",
			Description: "Runs a gh CLI subcommand",
			Fields: []FieldDescriptor{
				{Name: "label", Label: "label", Type: "string"},
				{Name: "cmd", Label: "subcommand", Type: "string", Required: true, Help: "e.g. pr view"},
				{Name: "args", Label: "arguments", Type: "stringArray"},
				{Name: "cwd", Label: "cwd", Type: "string"},
				{Name: "env", Label: "env", Type: "map"},
			},
		}, true
	case "gh:pr", "gh:issue", "gh:checkout":
		return AppTypeDescriptor{
			Type:        name,
			Label:       name,
			Description: "Runs gh " + name,
			Fields: []FieldDescriptor{
				{Name: "label", Label: "label", Type: "string"},
				{Name: "args", Label: "arguments", Type: "stringArray"},
				{Name: "cwd", Label: "cwd", Type: "string"},
				{Name: "env", Label: "env", Type: "map"},
			},
		}, true
	case "gh:repo-clone":
		return AppTypeDescriptor{
			Type:        "gh:repo-clone",
			Label:       "gh:repo-clone",
			Description: "Clones a repo via gh",
			Fields: []FieldDescriptor{
				{Name: "label", Label: "label", Type: "string"},
				{Name: "url", Label: "repo url", Type: "string", Required: true},
				{Name: "cwd", Label: "cwd", Type: "string", Help: "Destination directory"},
			},
		}, true
	default:
		return AppTypeDescriptor{}, false
	}
}
