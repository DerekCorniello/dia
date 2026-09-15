//go:build windows

package platform

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// ProcessIdentity returns the kernel creation time together with the PID.
// Windows exposes this through the process handle, and the creation time
// changes whenever the system recycles a PID.
func (*winPlatform) ProcessIdentity(pid int) (string, error) {
	if pid <= 0 {
		return "", nil
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d", pid, created.Nanoseconds()), nil
}
