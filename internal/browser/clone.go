package browser

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// cloner copies a profile tree. Implementations try a copy-on-write
// clone first (near-instant, near-zero space on btrfs/APFS) and fall
// back to a byte copy per file. The exclude set holds profile-relative
// paths (slash-separated, matched against any path component prefix)
// that are safe to skip -- caches and crash dumps -- so a plain copy
// stays as small as possible.
type cloner interface {
	// CloneTree copies src into dst (which must not already exist),
	// skipping any path under an excluded directory. reflink reports
	// whether the fast copy-on-write path was used for the bulk of
	// the data, for logging.
	CloneTree(src, dst string, exclude []string) (reflink bool, err error)
}

// cloneFile performs a single-file copy-on-write clone if the platform
// supports it, returning ok=false (no error) when it must fall back.
// Defined per-OS in clone_linux.go / clone_darwin.go / clone_other.go.
//
// It is only called for regular files; the caller handles dirs and
// symlinks.

// treeCloner is the shared CloneTree walk. It defers the per-file
// copy-on-write attempt to cloneFile and byte-copies on fallback.
type treeCloner struct{}

func newCloner() cloner { return treeCloner{} }

func (treeCloner) CloneTree(src, dst string, exclude []string) (bool, error) {
	info, err := os.Stat(src)
	if err != nil {
		return false, fmt.Errorf("stat seed %s: %w", src, err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("seed %s is not a directory", src)
	}
	if _, err := os.Stat(dst); err == nil {
		return false, fmt.Errorf("clone target %s already exists", dst)
	}

	usedReflink := false
	walkErr := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(dst, 0o700)
		}
		if isExcluded(rel, exclude) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		target := filepath.Join(dst, rel)

		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o700)
		case d.Type()&os.ModeSymlink != 0:
			// Copy the link itself, not its target: a profile may
			// contain relative symlinks and we must not dereference
			// into unrelated files.
			link, lerr := os.Readlink(path)
			if lerr != nil {
				return lerr
			}
			return os.Symlink(link, target)
		case d.Type().IsRegular():
			if ok, cerr := cloneFile(path, target); cerr != nil {
				return cerr
			} else if ok {
				usedReflink = true
				return nil
			}
			return copyFileBytes(path, target)
		default:
			// Sockets, devices, fifos: skip. A browser profile has
			// no legitimate need of them and copying them is unsafe.
			return nil
		}
	})
	if walkErr != nil {
		// Leave no half-populated clone behind for the caller to
		// mistake for a good profile.
		_ = os.RemoveAll(dst)
		return false, fmt.Errorf("clone %s -> %s: %w", src, dst, walkErr)
	}
	return usedReflink, nil
}

// isExcluded reports whether rel (a slash- or OS-separated relative
// path) falls under any excluded directory. Excludes are matched
// against whole path components so "Cache" excludes "Default/Cache/..."
// without also matching a file literally named "CacheKeys".
func isExcluded(rel string, exclude []string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, ex := range exclude {
		ex = strings.Trim(filepath.ToSlash(ex), "/")
		if ex == "" {
			continue
		}
		exParts := strings.Split(ex, "/")
		if containsSeq(parts, exParts) {
			return true
		}
	}
	return false
}

// containsSeq reports whether seq appears as a contiguous run at the
// start of, or anywhere within, parts. This lets an exclude of
// "Default/Cache" match "Default/Cache/data" while a bare "Cache"
// matches any Cache directory at any depth.
func containsSeq(parts, seq []string) bool {
	if len(seq) == 0 || len(seq) > len(parts) {
		return false
	}
	for i := 0; i+len(seq) <= len(parts); i++ {
		match := true
		for j := range seq {
			if parts[i+j] != seq[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func copyFileBytes(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, info.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}

// dirSize sums the on-disk size of the regular files under root that a
// plain copy would write, honoring the same excludes as CloneTree. Used
// for the free-space guard before a non-reflink copy.
func dirSize(root string, exclude []string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		if rel != "." && isExcluded(rel, exclude) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type().IsRegular() {
			info, ierr := d.Info()
			if ierr != nil {
				return ierr
			}
			total += info.Size()
		}
		return nil
	})
	return total, err
}
