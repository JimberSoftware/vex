//go:build linux

package vsock

import (
	"net"

	"github.com/mdlayher/vsock"
	"golang.org/x/sys/unix"
)

func Listen(cid, port uint32) (net.Listener, error) {
	return vsock.ListenContextID(cid, port, nil)
}

func Dial(cid, port uint32) (net.Conn, error) {
	conn, err := vsock.Dial(cid, port, nil)
	if err != nil {
		return nil, err
	}
	return &shutdownConn{Conn: conn}, nil
}

// Explicit shutdown lets the Windows VirtIO driver finish peer cleanup before
// the host releases the connection. Close alone can leave guest closes blocked.
type shutdownConn struct{ *vsock.Conn }

func (c *shutdownConn) Close() error {
	raw, err := c.SyscallConn()
	if err == nil {
		_ = raw.Control(func(fd uintptr) { _ = unix.Shutdown(int(fd), unix.SHUT_RDWR) })
	}
	return c.Conn.Close()
}
