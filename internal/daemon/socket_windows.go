//go:build windows

package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"net"

	winio "github.com/Microsoft/go-winio"
)

// socketNamePrefix is the fixed part of the daemon named pipe below.
// The pipe name is machine-wide on Windows, so the state dir is hashed
// into it: that isolates one user's daemon from another's and keeps
// concurrent daemons (as in the test suite) from colliding, matching
// the per-state-dir isolation the unix socket gives.
const socketNamePrefix = `\\.\pipe\dia-`

// socketPath returns the named pipe path to the daemon socket for a
// given state dir.
func socketPath(stateDir string) string {
	sum := sha256.Sum256([]byte(stateDir))
	return socketNamePrefix + hex.EncodeToString(sum[:8])
}

// dial attempts to connect to a live daemon named pipe. The dial fails
// with a "file not found" style error when no daemon is listening; the
// caller reports it as a dead daemon.
func dialSocket(path string) (net.Conn, error) {
	return winio.DialPipe(path, nil)
}

// listenSocket binds the named pipe name. The server half of a named
// pipe cannot be unlinked, so there is nothing to clean up beforehand.
func listenSocket(path string) (net.Listener, error) {
	return winio.ListenPipe(path, nil)
}

// removeSocket is a no-op on Windows: named pipes go away when the
// last handle closes.
func removeSocket(path string) {}
