//go:build darwin

package browser

import "golang.org/x/sys/unix"

// cloneFile attempts an APFS clonefile of src to dst. clonefile fails if
// dst already exists, so the caller guarantees a fresh target. A
// filesystem that cannot clone (or a cross-volume copy) returns
// (false, nil) so the caller byte-copies instead.
func cloneFile(src, dst string) (bool, error) {
	if err := unix.Clonefile(src, dst, 0); err != nil {
		switch err {
		case unix.ENOTSUP, unix.EXDEV, unix.EINVAL, unix.ENOTDIR:
			return false, nil
		default:
			return false, nil
		}
	}
	// clonefile copies mode bits too; nothing else to do.
	return true, nil
}
