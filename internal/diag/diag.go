// Package diag holds diagnostic helpers shared by the CLI and the
// wails-bound App: platform open-helper name and the smoke check
// rows. Centralising them keeps the CLI and GUI consistent (same
// checks, same labels) without forcing one to import the other.
package diag

import (
	"os/exec"
	"runtime"
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

// RunChecks produces the standard set of smoke checks for dia:
// platform, state dir/file, gh CLI, and URL handler. Callers can
// append or override rows; the returned slice is the baseline.
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
	return checks
}
