//go:build windows

package diag

import (
	"golang.org/x/sys/windows"
)

// freeBytes reports available bytes on the volume holding dir via
// GetDiskFreeSpaceEx.
func freeBytes(dir string) (uint64, error) {
	path, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		return 0, err
	}
	var freeBytesAvailable uint64
	if err := windows.GetDiskFreeSpaceEx(path, &freeBytesAvailable, nil, nil); err != nil {
		return 0, err
	}
	return freeBytesAvailable, nil
}
