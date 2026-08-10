//go:build windows

package daemon

import (
	"net"
)

// socketName is a stable component of the named pipe. The consumer
// identity is fixed regardless of state dir; windows named pipes are
// machine-wide, matching the single-user desktop app model.
const socketName = "\\\\.\\pipe\\dia"

// socketPath returns the named pipe path to the daemon socket.
func socketPath(stateDir string) string {
	return socketName
}

// dial attempts to connect to a live daemon named pipe. The client
// half of a named pipe is dynamic; Windows allows one-shot name reuse
// for the client, so dialing a nonexistent pipe yields "file not
// found" and the caller reports it as a dead daemon.
func dialSocket(path string) (net.Conn, error) {
	return net.Dial("pipe", path)
}

// listenSocket binds the named pipe name. The server half of a named
// pipe cannot be unlinked, so there is nothing to clean up beforehand.
func listenSocket(path string) (net.Listener, error) {
	return net.Listen("pipe", path)
}

// removeSocket is a no-op on Windows: named pipes go away when the
// last handle closes.
func removeSocket(path string) {}
