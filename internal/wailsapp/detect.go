package wailsapp

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// DetectTools scans PATH and environment variables for known editors,
// terminals, and browsers. Returns categorized results for the
// WorkspaceEditor quick-add UI.
func (a *App) DetectTools() []ToolCategory {
	return []ToolCategory{
		{Name: "Editors", Tools: detectEditors()},
		{Name: "Terminals", Tools: detectTerminals()},
		{Name: "Browsers", Tools: detectBrowsers()},
	}
}

func detectEditors() []DetectedTool {
	candidates := []struct {
		label string
		bin   string
	}{
		{"VS Code", "code"},
		{"VS Code (Insiders)", "code-insiders"},
		{"Cursor", "cursor"},
		{"IntelliJ IDEA", "idea"},
		{"PyCharm", "pycharm"},
		{"WebStorm", "webstorm"},
		{"GoLand", "goland"},
		{"CLion", "clion"},
		{"RustRover", "rust-rover"},
		{"Neovim", "nvim"},
		{"Vim", "vim"},
		{"Emacs", "emacs"},
		{"Nano", "nano"},
		{"Sublime Text", "subl"},
		{"Kate", "kate"},
		{"Gedit", "gedit"},
		{"GNOME Text Editor", "gnome-text-editor"},
	}
	var tools []DetectedTool
	seen := map[string]bool{}
	for _, c := range candidates {
		if _, err := exec.LookPath(c.bin); err != nil {
			continue
		}
		if seen[c.bin] {
			continue
		}
		seen[c.bin] = true
		tools = append(tools, DetectedTool{
			Label:   c.label,
			Command: c.bin,
			Url:     "",
		})
	}
	// Check $EDITOR / $VISUAL
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := os.Getenv(env); v != "" {
			bin := filepath.Base(v)
			if !seen[bin] {
				tools = append(tools, DetectedTool{
					Label:   "Default editor ($" + env + ")",
					Command: v,
					Url:     "",
				})
				seen[bin] = true
			}
		}
	}
	sortTools(tools)
	return tools
}

func detectTerminals() []DetectedTool {
	candidates := []struct {
		label string
		bin   string
	}{
		{"GNOME Terminal", "gnome-terminal"},
		{"Konsole", "konsole"},
		{"XFCE Terminal", "xfce4-terminal"},
		{"Kitty", "kitty"},
		{"Alacritty", "alacritty"},
		{"WezTerm", "wezterm"},
		{"Tmux", "tmux"},
		{"Tilix", "tilix"},
		{"Terminator", "terminator"},
		{"XTerm", "xterm"},
		{"Foot", "foot"},
		{"ST", "st"},
		{"URxvt", "urxvt"},
	}
	var tools []DetectedTool
	seen := map[string]bool{}
	for _, c := range candidates {
		if _, err := exec.LookPath(c.bin); err != nil {
			continue
		}
		if seen[c.bin] {
			continue
		}
		seen[c.bin] = true
		tools = append(tools, DetectedTool{
			Label:   c.label,
			Command: c.bin,
		})
	}
	if v := os.Getenv("TERMINAL"); v != "" {
		bin := filepath.Base(v)
		if !seen[bin] {
			tools = append(tools, DetectedTool{
				Label:   "Default terminal ($TERMINAL)",
				Command: v,
			})
			seen[bin] = true
		}
	}
	sortTools(tools)
	return tools
}

func detectBrowsers() []DetectedTool {
	candidates := []struct {
		label string
		bin   string
	}{
		{"Firefox", "firefox"},
		{"Google Chrome", "google-chrome"},
		{"Chromium", "chromium"},
		{"Brave", "brave-browser"},
		{"Microsoft Edge", "microsoft-edge"},
		{"Opera", "opera"},
		{"Vivaldi", "vivaldi"},
		{"Zen Browser", "zen-browser"},
	}
	var tools []DetectedTool
	seen := map[string]bool{}
	for _, c := range candidates {
		if _, err := exec.LookPath(c.bin); err != nil {
			continue
		}
		if seen[c.bin] {
			continue
		}
		seen[c.bin] = true
		tools = append(tools, DetectedTool{
			Label:   c.label,
			Command: c.bin,
			Url:     "",
		})
	}
	if v := os.Getenv("BROWSER"); v != "" {
		bin := filepath.Base(v)
		if !seen[bin] {
			tools = append(tools, DetectedTool{
				Label:   "Default browser ($BROWSER)",
				Command: v,
				Url:     "",
			})
			seen[bin] = true
		}
	}
	// Try xdg-settings for default web browser
	if _, err := exec.LookPath("xdg-settings"); err == nil {
		out, err := exec.Command("xdg-settings", "get", "default-web-browser").Output()
		if err == nil && len(out) > 0 {
			bin := filepath.Base(string(out[:len(out)-1]))
			if !seen[bin] && bin != "" {
				tools = append(tools, DetectedTool{
					Label:   "Default browser (xdg)",
					Command: bin,
					Url:     "",
				})
				seen[bin] = true
			}
		}
	}
	sortTools(tools)
	return tools
}

func sortTools(tools []DetectedTool) {
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Label < tools[j].Label
	})
}
