//go:build !linux && !darwin

package browser

import "math"

// freeBytes cannot query free space portably on this platform, so it
// reports "effectively unlimited" and lets the copy proceed. The plain
// copy will still fail loudly if the disk genuinely fills.
func freeBytes(path string) (uint64, error) {
	return math.MaxUint64, nil
}
