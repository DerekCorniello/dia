//go:build darwin

package platform

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// ProcessIdentity returns the Darwin process start time together with the
// PID. The kernel start time changes when a PID is recycled, so a recovered
// instance cannot accidentally signal a new process with the old PID.
func (unixPlatform) ProcessIdentity(pid int) (string, error) {
	if pid <= 0 {
		return "", nil
	}
	kp, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d:%d", pid, kp.Proc.P_starttime.Sec, kp.Proc.P_starttime.Usec), nil
}
