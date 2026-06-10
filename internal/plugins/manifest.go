package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Manifest is the parsed contents of a plugin's plugin.json file.
// Fields are validated by Validate; the zero value is invalid.
type Manifest struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Version         string         `json:"version"`
	Description     string         `json:"description"`
	LongDescription string         `json:"long_description,omitempty"`
	Author          string         `json:"author"`
	Entry           string         `json:"entry"`
	Capabilities    []string       `json:"capabilities"`
	ConfigSchema    map[string]any `json:"config_schema"`
	UI              UISpec         `json:"ui"`
}

// UISpec describes how the host should render a plugin's panel. The
// type drives which host component renders the plugin's getData()
// result; actions are buttons that call back into the plugin. A
// type of "window" is a plugin that runs in its own OS-level window:
// the host spawns a second dia process that serves the plugin's
// panel/ folder and exposes dia.* as a wails binding.
type UISpec struct {
	Type        string         `json:"type"`
	Title       string         `json:"title"`
	Entry       string         `json:"entry,omitempty"`
	Width       int            `json:"width,omitempty"`
	Height      int            `json:"height,omitempty"`
	Refreshable bool           `json:"refreshable,omitempty"`
	Actions     []UIAction     `json:"actions,omitempty"`
	Columns     []UIColumn     `json:"columns,omitempty"`
	Extra       map[string]any `json:"-"`
}

// UIAction is a button in a plugin panel. Capability is required;
// an action's call is rejected if the plugin lacks the capability.
type UIAction struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Confirm    bool   `json:"confirm,omitempty"`
	Capability string `json:"capability"`
}

// UIColumn describes one column of a "table" UI.
type UIColumn struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Format string `json:"format,omitempty"`
}

// LoadManifest reads and validates plugin.json from dir.
func LoadManifest(dir string) (*Manifest, error) {
	mfPath := filepath.Join(dir, manifestFile)
	data, err := os.ReadFile(mfPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("plugin.json not found in %s", dir)
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	raw := map[string]any{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	m := &Manifest{}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if ui, ok := raw["ui"].(map[string]any); ok {
		m.UI.Extra = ui
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

// PanelJSPath returns the path of the plugin's panel entry JS
// relative to the plugin dir. Defaults to panel/panel.js. Safe
// on a nil receiver; returns the default in that case.
func (m *Manifest) PanelJSPath() string {
	if m == nil {
		return DefaultPanelJS
	}
	if m.UI.Type != "window" {
		return ""
	}
	if m.UI.Entry != "" {
		return m.UI.Entry
	}
	return DefaultPanelJS
}

// Validate enforces the manifest schema. The rules:
//   - id matches [a-z0-9][a-z0-9-]{2,39}
//   - name 1..60 chars
//   - version 1..32 chars
//   - description 0..200 chars
//   - author 0..60 chars
//   - entry is a relative path (no leading slash, no "..")
//   - capabilities is a subset of the known capability list
//   - ui.type is one of list|grid|table|kv|text|canvas|window
//   - ui.title is non-empty
//   - ui.columns is required for type=table
//   - ui.entry, when set, is a relative path inside the plugin
//     folder (used by type=window to find the panel/panel.js)
func (m *Manifest) Validate() error {
	if !validID(m.ID) {
		return fmt.Errorf("plugin id %q must match %s", m.ID, idPattern)
	}
	if m.Name == "" {
		return errors.New("plugin name is required")
	}
	if len(m.Name) > nameMaxLen {
		return fmt.Errorf("plugin name must be %d characters or fewer", nameMaxLen)
	}
	if m.Version == "" {
		return errors.New("plugin version is required")
	}
	if len(m.Version) > versionMaxLen {
		return fmt.Errorf("plugin version must be %d characters or fewer", versionMaxLen)
	}
	if len(m.Description) > descMaxLen {
		return fmt.Errorf("plugin description must be %d characters or fewer", descMaxLen)
	}
	if len(m.LongDescription) > longDescMaxLen {
		return fmt.Errorf("plugin long description must be %d characters or fewer", longDescMaxLen)
	}
	if len(m.Author) > authorMaxLen {
		return fmt.Errorf("plugin author must be %d characters or fewer", authorMaxLen)
	}
	if m.Entry == "" {
		m.Entry = defaultEntry
	}
	if filepath.IsAbs(m.Entry) || strings.HasPrefix(m.Entry, "/") || strings.HasPrefix(m.Entry, "..") {
		return fmt.Errorf("plugin entry %q must be a relative path with no parent references", m.Entry)
	}
	for _, c := range m.Capabilities {
		if !IsKnownCapability(c) {
			return fmt.Errorf("plugin capability %q is not recognized", c)
		}
	}
	switch m.UI.Type {
	case "list", "grid", "kv", "text", "canvas":
	case "table":
		if len(m.UI.Columns) == 0 {
			return errors.New("ui.type=table requires at least one column")
		}
		for _, c := range m.UI.Columns {
			if c.Key == "" {
				return errors.New("ui.column key is required")
			}
			if c.Label == "" {
				return fmt.Errorf("ui.column %q label is required", c.Key)
			}
			if c.Format != "" && c.Format != "badge" && c.Format != "duration" && c.Format != "text" {
				return fmt.Errorf("ui.column %q format %q must be badge|duration|text", c.Key, c.Format)
			}
		}
	case "window":
		if m.UI.Entry != "" {
			if filepath.IsAbs(m.UI.Entry) || strings.HasPrefix(m.UI.Entry, "/") || strings.HasPrefix(m.UI.Entry, "..") {
				return fmt.Errorf("ui.entry %q must be a relative path with no parent references", m.UI.Entry)
			}
		}
	default:
		return fmt.Errorf("ui.type %q must be one of list|grid|table|kv|text|canvas|window", m.UI.Type)
	}
	if m.UI.Title == "" {
		return errors.New("ui.title is required")
	}
	for _, a := range m.UI.Actions {
		if a.ID == "" {
			return errors.New("ui.action id is required")
		}
		if a.Label == "" {
			return fmt.Errorf("ui.action %q label is required", a.ID)
		}
		if a.Capability != "" && !IsKnownCapability(a.Capability) {
			return fmt.Errorf("ui.action %q capability %q is not recognized", a.ID, a.Capability)
		}
	}
	return nil
}
