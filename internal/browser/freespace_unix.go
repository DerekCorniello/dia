//go:build linux || darwin

package browser

import "golang.org/x/sys/unix"

// freeBytes returns the bytes available to an unprivileged writer at
// path's filesystem.
func freeBytes(path string) (uint64, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}
