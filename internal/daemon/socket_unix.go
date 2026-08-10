//go:build linux || darwin

package daemon

import (
	"net"
	"os"
	"path/filepath"
)

// socketName is the basename of the daemon's unix socket, kept under
// the state dir so different users/$XDG_STATE_HOME get different
// daemons.
const socketName = "serve.sock"

// socketPath returns the absolute path to the daemon socket for a
// given state dir.
func socketPath(stateDir string) string {
	return filepath.Join(stateDir, socketName)
}

// dial attempts to connect to a live daemon socket. The connection is
// refused when the daemon is not running or the socket is stale.
func dialSocket(path string) (net.Conn, error) {
	return net.Dial("unix", path)
}

// listenSocket binds the unix socket, removing any stale file first
// so a crash that left the path behind does not prevent a restart.
func listenSocket(path string) (net.Listener, error) {
	// Best-effort: if the file exists it is either a live socket (see
	// below) or stale. Unlink is only safe immediately before bind;
	// the daemon is single-instance per state dir, so a race with a
	// second daemon is acceptable.
	_ = os.Remove(path)
	return net.Listen("unix", path)
}

// removeSocket cleans up the socket file on server exit.
func removeSocket(path string) {
	_ = os.Remove(path)
}
