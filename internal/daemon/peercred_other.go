//go:build !linux

package daemon

import "net"

// Windows named-pipe ACLs and macOS private socket directories provide the
// platform access boundary. Keep the hook shared so Linux can add kernel
// peer credentials without changing the server loop.
func verifyPeer(net.Conn) error { return nil }
