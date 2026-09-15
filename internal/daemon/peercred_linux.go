//go:build linux

package daemon

import (
	"fmt"
	"net"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// verifyPeer enforces same-user access independently of filesystem modes.
// The socket is a local control plane with process-launch privileges, so a
// compromised directory mode must not turn it into a cross-user API.
func verifyPeer(conn net.Conn) error {
	sc, ok := conn.(syscall.Conn)
	if !ok {
		return nil
	}
	raw, err := sc.SyscallConn()
	if err != nil {
		return err
	}
	var cred *unix.Ucred
	var sockErr error
	if err := raw.Control(func(fd uintptr) {
		cred, sockErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return err
	}
	if sockErr != nil {
		return sockErr
	}
	if cred == nil || int(cred.Uid) != os.Getuid() {
		if cred == nil {
			return fmt.Errorf("missing peer credentials")
		}
		return fmt.Errorf("peer uid %d is not daemon uid %d", cred.Uid, os.Getuid())
	}
	return nil
}
