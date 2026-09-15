//go:build linux

package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ProcessIdentity returns a token that changes whenever Linux recycles a PID.
// It is deliberately based on procfs starttime rather than executable path,
// because the same binary may legitimately back multiple workspace apps.
func (unixPlatform) ProcessIdentity(pid int) (string, error) {
	if pid <= 0 {
		return "", nil
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	line := string(b)
	end := strings.LastIndex(line, ") ")
	if end < 0 || end+2 >= len(line) {
		return "", fmt.Errorf("invalid proc stat for pid %d", pid)
	}
	fields := strings.Fields(line[end+2:])
	if len(fields) <= 19 {
		return "", fmt.Errorf("short proc stat for pid %d", pid)
	}
	return strconv.Itoa(pid) + ":" + fields[19], nil
}
