// Package diag holds diagnostic helpers shared by the CLI and the
// wails-bound App: platform open-helper name and the smoke check
// rows. Centralising them keeps the CLI and GUI consistent (same
// checks, same labels) without forcing one to import the other.
package diag

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
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
// platform, state dir/file/parse, daemon socket liveness, disk space,
// browser profile perms, gh CLI, and URL handler. Callers can
// append or override rows; the returned slice is the baseline.
func RunChecks(stateDir, stateFile string) []Check {
	var checks []Check
	checks = append(checks, Check{
		Name:   "platform",
		Status: "ok",
		Detail: runtime.GOOS + "/" + runtime.GOARCH,
	})
	if stateDir != "" {
		checks = append(checks, permissionCheck("state dir", stateDir, 0o700, stateDir))
	}
	if stateFile != "" {
		checks = append(checks, permissionCheck("state file", stateFile, 0o600, stateFile))
		checks = append(checks, stateParseCheck(stateFile))
		if stateDir != "" {
			checks = append(checks, socketCheck(filepath.Join(stateDir, "serve.sock")))
			checks = append(checks, diskSpaceCheck(stateDir))
			checks = append(checks, permissionCheck("browser profiles", filepath.Join(stateDir, "browser"), 0o700, "no managed profiles"))
		}
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

// stateParseCheck verifies the state file parses. A torn write from a
// crash must surface here, not as a silent empty launcher.
func stateParseCheck(path string) Check {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Check{Name: "state parse", Status: "ok", Detail: "no state yet"}
		}
		return Check{Name: "state parse", Status: "fail", Detail: err.Error()}
	}
	if len(data) == 0 {
		return Check{Name: "state parse", Status: "ok", Detail: "empty"}
	}
	var v struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return Check{Name: "state parse", Status: "fail", Detail: "corrupt; restore " + path + ".bak"}
	}
	return Check{Name: "state parse", Status: "ok", Detail: path}
}

// socketCheck distinguishes a live daemon from a stale socket file left
// by a crash. A stale file alone must never read as healthy.
func socketCheck(path string) Check {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return Check{Name: "daemon socket", Status: "ok", Detail: "not running (starts on demand)"}
		}
		return Check{Name: "daemon socket", Status: "warn", Detail: err.Error()}
	}
	conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return Check{Name: "daemon socket", Status: "ok", Detail: path}
	}
	return Check{Name: "daemon socket", Status: "warn", Detail: "stale socket file; removed on next start"}
}

// diskSpaceCheck warns before profile clones can run the disk out.
func diskSpaceCheck(dir string) Check {
	free, err := freeBytes(dir)
	if err != nil {
		return Check{Name: "disk space", Status: "warn", Detail: err.Error()}
	}
	switch {
	case free < 100<<20:
		return Check{Name: "disk space", Status: "fail", Detail: fmt.Sprintf("%d MB free", free>>20)}
	case free < 1<<30:
		return Check{Name: "disk space", Status: "warn", Detail: fmt.Sprintf("%d MB free", free>>20)}
	default:
		return Check{Name: "disk space", Status: "ok", Detail: fmt.Sprintf("%d GB free", free>>30)}
	}
}

func permissionCheck(name, path string, want os.FileMode, missingDetail string) Check {
	info, err := os.Stat(path)
	if err != nil {
		return Check{Name: name, Status: "warn", Detail: missingDetail}
	}
	if runtime.GOOS == "windows" {
		// Unix permission bits are not enforced on Windows (Go
		// always reports 0777/0666); access is governed by ACLs,
		// and the daemon pipe gets an owner-only descriptor at
		// creation. Reporting fail here would be permanently red.
		return Check{Name: name, Status: "ok", Detail: path + " (ACLs apply)"}
	}
	if info.Mode().Perm()&0o077 != 0 {
		return Check{Name: name, Status: "fail", Detail: fmt.Sprintf("permissions %o; want %o or stricter", info.Mode().Perm(), want.Perm())}
	}
	return Check{Name: name, Status: "ok", Detail: path}
}
