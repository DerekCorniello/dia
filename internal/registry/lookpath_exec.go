//go:build !js

package registry

import "os/exec"

// execLookPath is a thin wrapper so tests can swap out the PATH
// lookup without pulling in os/exec from the test file.
func execLookPath(file string) (string, error) {
	return exec.LookPath(file)
}
