//go:build windows

package vsock

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	afVSock    = 40          // AF_VSOCK — exposed by virtio-win vsock driver
	sockStream = 1           // SOCK_STREAM
	errSock    = ^uintptr(0) // INVALID_SOCKET / SOCKET_ERROR
)

var (
	modws2_32       = windows.NewLazySystemDLL("ws2_32.dll")
	procWSASocket   = modws2_32.NewProc("WSASocketW")
	procBind        = modws2_32.NewProc("bind")
	procConnect     = modws2_32.NewProc("connect")
	procListen      = modws2_32.NewProc("listen")
	procAccept      = modws2_32.NewProc("accept")
	procClosesocket = modws2_32.NewProc("closesocket")
	procRecv        = modws2_32.NewProc("recv")
	procSend        = modws2_32.NewProc("send")
)

// rawSockaddrVM mirrors struct sockaddr_vm (16 bytes, same layout as Linux).
type rawSockaddrVM struct {
	Family   uint16
	Reserved uint16
	Port     uint32
	CID      uint32
	Flags    uint8
	_        [3]byte
}

type vsockAddr struct {
	CID  uint32
	Port uint32
}

func (a *vsockAddr) Network() string { return "vsock" }
func (a *vsockAddr) String() string  { return fmt.Sprintf("%d:%d", a.CID, a.Port) }

func newSocket() (uintptr, error) {
	r1, _, e := procWSASocket.Call(afVSock, sockStream, 0, 0, 0, 0)
	if r1 == errSock {
		return 0, os.NewSyscallError("WSASocket", e)
	}
	return r1, nil
}

func closeSocket(s uintptr) {
	procClosesocket.Call(s) //nolint:errcheck
}

// Listen binds and listens on the given CID and port.
func Listen(cid, port uint32) (net.Listener, error) {
	s, err := newSocket()
	if err != nil {
		return nil, err
	}
	sa := rawSockaddrVM{Family: afVSock, CID: cid, Port: port}
	r1, _, e := procBind.Call(s, uintptr(unsafe.Pointer(&sa)), unsafe.Sizeof(sa))
	if r1 == errSock {
		closeSocket(s)
		return nil, os.NewSyscallError("bind", e)
	}
	r1, _, e = procListen.Call(s, 128)
	if r1 == errSock {
		closeSocket(s)
		return nil, os.NewSyscallError("listen", e)
	}
	return &vsockListener{sock: s, addr: vsockAddr{CID: cid, Port: port}}, nil
}

// Dial connects to the given CID and port.
func Dial(cid, port uint32) (net.Conn, error) {
	s, err := newSocket()
	if err != nil {
		return nil, err
	}
	sa := rawSockaddrVM{Family: afVSock, CID: cid, Port: port}
	r1, _, e := procConnect.Call(s, uintptr(unsafe.Pointer(&sa)), unsafe.Sizeof(sa))
	if r1 == errSock {
		closeSocket(s)
		return nil, os.NewSyscallError("connect", e)
	}
	return &vsockConn{sock: s, remote: vsockAddr{CID: cid, Port: port}}, nil
}

// vsockListener implements net.Listener.
type vsockListener struct {
	sock uintptr
	addr vsockAddr
}

func (l *vsockListener) Accept() (net.Conn, error) {
	var peer rawSockaddrVM
	peerLen := int32(unsafe.Sizeof(peer))
	r1, _, e := procAccept.Call(l.sock, uintptr(unsafe.Pointer(&peer)), uintptr(unsafe.Pointer(&peerLen)))
	if r1 == errSock {
		return nil, os.NewSyscallError("accept", e)
	}
	return &vsockConn{
		sock:   r1,
		local:  l.addr,
		remote: vsockAddr{CID: peer.CID, Port: peer.Port},
	}, nil
}

func (l *vsockListener) Close() error {
	r1, _, e := procClosesocket.Call(l.sock)
	if r1 == errSock {
		return os.NewSyscallError("closesocket", e)
	}
	return nil
}

func (l *vsockListener) Addr() net.Addr { return &l.addr }

// vsockConn implements net.Conn.
type vsockConn struct {
	sock   uintptr
	local  vsockAddr
	remote vsockAddr
}

func (c *vsockConn) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	r1, _, e := procRecv.Call(c.sock, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0)
	if r1 == errSock {
		return 0, os.NewSyscallError("recv", e)
	}
	if r1 == 0 {
		return 0, io.EOF
	}
	return int(r1), nil
}

func (c *vsockConn) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	r1, _, e := procSend.Call(c.sock, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0)
	if r1 == errSock {
		return 0, os.NewSyscallError("send", e)
	}
	return int(r1), nil
}

func (c *vsockConn) Close() error {
	r1, _, e := procClosesocket.Call(c.sock)
	if r1 == errSock {
		return os.NewSyscallError("closesocket", e)
	}
	return nil
}

func (c *vsockConn) LocalAddr() net.Addr              { return &c.local }
func (c *vsockConn) RemoteAddr() net.Addr             { return &c.remote }
func (c *vsockConn) SetDeadline(_ time.Time) error      { return nil }
func (c *vsockConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *vsockConn) SetWriteDeadline(_ time.Time) error { return nil }
