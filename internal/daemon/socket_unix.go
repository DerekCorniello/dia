//go:build linux || darwin

package daemon

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"time"
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
	if _, err := os.Stat(path); err == nil {
		conn, dialErr := net.DialTimeout("unix", path, 100*time.Millisecond)
		if dialErr == nil {
			_ = conn.Close()
			return nil, fmt.Errorf("daemon already running")
		}
		if !errors.Is(dialErr, syscall.ECONNREFUSED) && !errors.Is(dialErr, syscall.ENOENT) {
			return nil, fmt.Errorf("probe existing socket: %w", dialErr)
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		_ = os.Remove(path)
		return nil, err
	}
	return ln, nil
}

// removeSocket cleans up the socket file on server exit.
func removeSocket(path string) {
	_ = os.Remove(path)
}
