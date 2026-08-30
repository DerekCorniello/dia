//go:build linux

package browser

import (
	"os"

	"golang.org/x/sys/unix"
)

// cloneFile attempts a FICLONE (copy-on-write) clone of src to dst. It
// returns (true, nil) when the clone succeeded, (false, nil) when the
// filesystem does not support reflinks so the caller should byte-copy,
// and (false, err) only on an error worth surfacing.
func cloneFile(src, dst string) (bool, error) {
	in, err := os.Open(src)
	if err != nil {
		return false, err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return false, err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return false, err
	}
	if err := unix.IoctlFileClone(int(out.Fd()), int(in.Fd())); err != nil {
		// EOPNOTSUPP/ENOTSUP: not a reflink-capable fs. EXDEV: src and
		// dst on different filesystems. EINVAL: fs cannot clone this
		// file. All mean "fall back to a byte copy", not "fail".
		out.Close()
		_ = os.Remove(dst)
		// EOPNOTSUPP (== ENOTSUP on Linux): not reflink-capable. EXDEV:
		// different filesystems. EINVAL: fs cannot clone this file. Any
		// other error: still fall back to a byte copy rather than abort.
		_ = err
		return false, nil
	}
	return true, out.Close()
}
