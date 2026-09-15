//go:build linux

package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// SetFloating requests the compositor to float (or tile) the window
// owned by pid. On Wayland the client cannot dictate placement, so
// this shells out to hyprctl when running under Hyprland and is a
// no-op everywhere else. Failures are advisory: the window simply
// opens with the compositor default. Never block the caller; poll
// briefly because the window may not be mapped yet when called.
func SetFloating(pid int, floating bool) error {
	if pid <= 0 {
		return nil
	}
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return nil
	}
	if _, err := exec.LookPath("hyprctl"); err != nil {
		return nil
	}
	want := "tile"
	if floating {
		want = "float"
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		addr, isFloating, found, err := findClient(pid)
		if err != nil {
			return fmt.Errorf("hyprctl clients: %w", err)
		}
		if !found {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		if isFloating == floating {
			return nil
		}
		out, err := exec.Command("hyprctl", "dispatch", "togglefloating", "address:"+addr).CombinedOutput()
		if err != nil {
			return fmt.Errorf("hyprctl %s: %w: %s", want, err, string(out))
		}
		return nil
	}
	return fmt.Errorf("window for pid %d never appeared", pid)
}

type hyprClient struct {
	Address  string `json:"address"`
	PID      int    `json:"pid"`
	Floating bool   `json:"floating"`
}

func findClient(pid int) (addr string, isFloating, found bool, err error) {
	out, err := exec.Command("hyprctl", "clients", "-j").Output()
	if err != nil {
		return "", false, false, err
	}
	var clients []hyprClient
	if err := json.Unmarshal(out, &clients); err != nil {
		return "", false, false, err
	}
	for _, c := range clients {
		if c.PID == pid {
			return c.Address, c.Floating, true, nil
		}
	}
	return "", false, false, nil
}
