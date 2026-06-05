// Package diag holds diagnostic helpers shared by the CLI and the
// wails-bound App: plugin discovery, platform open-helper name, and
// the smoke check rows. Centralising them keeps the CLI and GUI
// consistent (same checks, same labels) without forcing one to
// import the other.
package diag

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Check is one row of the doctor output. Status is "ok", "warn", or
// "fail"; Detail is an optional human-readable annotation (e.g. the
// gh binary path, or "not found").
type Check struct {
	Name   string
	Status string
	Detail string
}

// PlatformOpenHelper returns the binary dia uses to open URLs on
// the current OS.
func PlatformOpenHelper() string {
	switch runtime.GOOS {
	case "darwin":
		return "open"
	case "windows":
		return "cmd"
	}
	return "xdg-open"
}

// ScanPlugins walks the process PATH looking for executables named
// dia-*. Returns the sorted list of absolute paths.
func ScanPlugins() []string {
	dirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	seen := map[string]bool{}
	var out []string
	for _, d := range dirs {
		if d == "" {
			continue
		}
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, "dia-") {
				continue
			}
			full := filepath.Join(d, name)
			info, err := e.Info()
			if err != nil {
				continue
			}
			if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
				continue
			}
			if seen[full] {
				continue
			}
			seen[full] = true
			out = append(out, full)
		}
	}
	sort.Strings(out)
	return out
}

// RunChecks produces the standard set of smoke checks for dia:
// platform, state dir/file, gh CLI, URL handler, and plugin scan.
// Callers can append or override rows; the returned slice is the
// baseline.
func RunChecks(stateDir, stateFile string) []Check {
	var checks []Check
	checks = append(checks, Check{
		Name:   "platform",
		Status: "ok",
		Detail: runtime.GOOS + "/" + runtime.GOARCH,
	})
	if stateDir != "" {
		checks = append(checks, Check{Name: "state dir", Status: "ok", Detail: stateDir})
	}
	if stateFile != "" {
		checks = append(checks, Check{Name: "state file", Status: "ok", Detail: stateFile})
	}
	if ghPath, err := exec.LookPath("gh"); err != nil {
		checks = append(checks, Check{Name: "gh cli", Status: "warn", Detail: "not found"})
	} else {
		checks = append(checks, Check{Name: "gh cli", Status: "ok", Detail: ghPath})
	}
	helper := PlatformOpenHelper()
	if path, err := exec.LookPath(helper); err != nil {
		checks = append(checks, Check{Name: "url handler", Status: "warn", Detail: helper + " not found"})
	} else {
		checks = append(checks, Check{Name: "url handler", Status: "ok", Detail: path})
	}
	plugins := ScanPlugins()
	checks = append(checks, Check{
		Name:   "plugins",
		Status: "ok",
		Detail: fmt.Sprintf("%d dia-* found", len(plugins)),
	})
	return checks
}
