//go:build !linux && !darwin

package browser

// cloneFile has no copy-on-write fast path on this platform, so it
// always signals "fall back to a byte copy". Windows ReFS block-cloning
// is not exposed portably enough to rely on here.
func cloneFile(src, dst string) (bool, error) {
	return false, nil
}
