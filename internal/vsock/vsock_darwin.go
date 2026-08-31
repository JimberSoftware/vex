//go:build darwin

package vsock

import (
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

const listenBacklog = 128

type darwinAddr struct {
	cid  uint32
	port uint32
}

func (a *darwinAddr) Network() string { return "vsock" }
func (a *darwinAddr) String() string  { return fmt.Sprintf("%d:%d", a.cid, a.port) }

type darwinConn struct {
	file   *os.File
	local  net.Addr
	remote net.Addr
}

func (c *darwinConn) Read(p []byte) (int, error)         { return c.file.Read(p) }
func (c *darwinConn) Write(p []byte) (int, error)        { return c.file.Write(p) }
func (c *darwinConn) Close() error                       { return c.file.Close() }
func (c *darwinConn) LocalAddr() net.Addr                { return c.local }
func (c *darwinConn) RemoteAddr() net.Addr               { return c.remote }
func (c *darwinConn) SetDeadline(t time.Time) error      { return c.file.SetDeadline(t) }
func (c *darwinConn) SetReadDeadline(t time.Time) error  { return c.file.SetReadDeadline(t) }
func (c *darwinConn) SetWriteDeadline(t time.Time) error { return c.file.SetWriteDeadline(t) }

type darwinListener struct {
	file *os.File
	addr net.Addr
}

func (l *darwinListener) Accept() (net.Conn, error) {
	raw, err := l.file.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("access vsock listener: %w", err)
	}
	var fd int
	var remote unix.Sockaddr
	var acceptErr error
	err = raw.Read(func(listenerFD uintptr) bool {
		for {
			fd, remote, acceptErr = unix.Accept(int(listenerFD))
			if acceptErr == unix.EINTR {
				continue
			}
			if acceptErr == unix.EAGAIN || acceptErr == unix.EWOULDBLOCK {
				return false
			}
			return true
		}
	})
	if err != nil {
		return nil, fmt.Errorf("wait for vsock connection: %w", err)
	}
	if acceptErr != nil {
		return nil, fmt.Errorf("accept vsock connection: %w", acceptErr)
	}
	unix.CloseOnExec(fd)
	if err := unix.SetNonblock(fd, true); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("configure accepted vsock connection: %w", err)
	}
	local, err := unix.Getsockname(fd)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("get local vsock address: %w", err)
	}
	return &darwinConn{
		file:   os.NewFile(uintptr(fd), "vex-vsock-connection"),
		local:  addrFromSockaddr(local),
		remote: addrFromSockaddr(remote),
	}, nil
}

func (l *darwinListener) Close() error   { return l.file.Close() }
func (l *darwinListener) Addr() net.Addr { return l.addr }

func Listen(cid, port uint32) (net.Listener, error) {
	fd, err := newDarwinSocket(true)
	if err != nil {
		return nil, err
	}
	addr := &unix.SockaddrVM{CID: cid, Port: port}
	if err := unix.Bind(fd, addr); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("bind vsock %d:%d: %w", cid, port, err)
	}
	if err := unix.Listen(fd, listenBacklog); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("listen on vsock %d:%d: %w", cid, port, err)
	}
	return &darwinListener{
		file: os.NewFile(uintptr(fd), "vex-vsock-listener"),
		addr: &darwinAddr{cid: cid, port: port},
	}, nil
}

func Dial(cid, port uint32) (net.Conn, error) {
	fd, err := newDarwinSocket(false)
	if err != nil {
		return nil, err
	}
	remote := &unix.SockaddrVM{CID: cid, Port: port}
	if err := unix.Connect(fd, remote); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("connect to vsock %d:%d: %w", cid, port, err)
	}
	if err := unix.SetNonblock(fd, true); err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("configure vsock connection: %w", err)
	}
	local, err := unix.Getsockname(fd)
	if err != nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("get local vsock address: %w", err)
	}
	return &darwinConn{
		file:   os.NewFile(uintptr(fd), "vex-vsock-connection"),
		local:  addrFromSockaddr(local),
		remote: &darwinAddr{cid: cid, port: port},
	}, nil
}

func newDarwinSocket(nonblocking bool) (int, error) {
	fd, err := unix.Socket(unix.AF_VSOCK, unix.SOCK_STREAM, 0)
	if err != nil {
		return 0, fmt.Errorf("create vsock socket: %w", err)
	}
	unix.CloseOnExec(fd)
	if nonblocking {
		if err := unix.SetNonblock(fd, true); err != nil {
			_ = unix.Close(fd)
			return 0, fmt.Errorf("configure vsock socket: %w", err)
		}
	}
	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_NOSIGPIPE, 1); err != nil {
		_ = unix.Close(fd)
		return 0, fmt.Errorf("configure vsock socket: %w", err)
	}
	return fd, nil
}

func addrFromSockaddr(addr unix.Sockaddr) net.Addr {
	vmAddr, ok := addr.(*unix.SockaddrVM)
	if !ok {
		return nil
	}
	return &darwinAddr{cid: vmAddr.CID, port: vmAddr.Port}
}
