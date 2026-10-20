//go:build windows

package vsock

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	afVSock            = 40          // AF_VSOCK — exposed by virtio-win vsock driver
	sockStream         = 1           // SOCK_STREAM
	errSock            = ^uintptr(0) // pointer-sized INVALID_SOCKET
	fionbio            = 0x8004667e
	acceptPollInterval = 10 * time.Millisecond
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
	procIoctlsocket = modws2_32.NewProc("ioctlsocket")
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

// Winsock's int results are 32-bit even on 64-bit Windows. Proc.Call returns
// uintptr, where SOCKET_ERROR may be zero-extended to 0x00000000ffffffff.
// Socket handles (WSASocket/accept) instead use pointer-sized INVALID_SOCKET.
func isSocketError(result uintptr) bool { return int32(result) == -1 }

func setNonblocking(s uintptr) error {
	mode := uint32(1)
	r1, _, e := procIoctlsocket.Call(s, fionbio, uintptr(unsafe.Pointer(&mode)))
	if isSocketError(r1) {
		return os.NewSyscallError("ioctlsocket", e)
	}
	return nil
}

// Listen binds and listens on the given CID and port.
func Listen(cid, port uint32) (net.Listener, error) {
	s, err := newSocket()
	if err != nil {
		return nil, err
	}
	sa := rawSockaddrVM{Family: afVSock, CID: cid, Port: port}
	r1, _, e := procBind.Call(s, uintptr(unsafe.Pointer(&sa)), unsafe.Sizeof(sa))
	if isSocketError(r1) {
		closeSocket(s)
		return nil, os.NewSyscallError("bind", e)
	}
	r1, _, e = procListen.Call(s, 128)
	if isSocketError(r1) {
		closeSocket(s)
		return nil, os.NewSyscallError("listen", e)
	}
	// Avoid a pending blocking accept in the VirtIO Windows driver. A stalled
	// request can otherwise leave the service running but unable to accept any
	// more host connections, including the request needed to diagnose it.
	if err := setNonblocking(s); err != nil {
		closeSocket(s)
		return nil, err
	}
	return &vsockListener{sock: s, addr: vsockAddr{CID: cid, Port: port}, closed: make(chan struct{})}, nil
}

// Dial connects to the given CID and port.
func Dial(cid, port uint32) (net.Conn, error) {
	s, err := newSocket()
	if err != nil {
		return nil, err
	}
	sa := rawSockaddrVM{Family: afVSock, CID: cid, Port: port}
	r1, _, e := procConnect.Call(s, uintptr(unsafe.Pointer(&sa)), unsafe.Sizeof(sa))
	if isSocketError(r1) {
		closeSocket(s)
		return nil, os.NewSyscallError("connect", e)
	}
	if err := setNonblocking(s); err != nil {
		closeSocket(s)
		return nil, err
	}
	return newConn(s, vsockAddr{}, vsockAddr{CID: cid, Port: port}), nil
}

// vsockListener implements net.Listener.
type vsockListener struct {
	mu     sync.Mutex
	sock   uintptr
	addr   vsockAddr
	closed chan struct{}
}

func (l *vsockListener) Accept() (net.Conn, error) {
	ticker := time.NewTicker(acceptPollInterval)
	defer ticker.Stop()
	for {
		// Keep Close from releasing and reusing the socket handle during accept.
		l.mu.Lock()
		select {
		case <-l.closed:
			l.mu.Unlock()
			return nil, net.ErrClosed
		default:
		}
		var peer rawSockaddrVM
		peerLen := int32(unsafe.Sizeof(peer))
		r1, _, e := procAccept.Call(l.sock, uintptr(unsafe.Pointer(&peer)), uintptr(unsafe.Pointer(&peerLen)))
		if r1 != errSock {
			l.mu.Unlock()
			// VirtIO inherits FIONBIO and some driver versions cannot clear it.
			// Keep the connection nonblocking and retry in Read/Write instead.
			return newConn(r1, l.addr, vsockAddr{CID: peer.CID, Port: peer.Port}), nil
		}
		l.mu.Unlock()
		if !errors.Is(e, windows.WSAEWOULDBLOCK) && !errors.Is(e, windows.WSAECONNRESET) && !errors.Is(e, windows.WSAECONNABORTED) {
			return nil, os.NewSyscallError("accept", e)
		}
		select {
		case <-l.closed:
			return nil, net.ErrClosed
		case <-ticker.C:
		}
	}
}

func (l *vsockListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	select {
	case <-l.closed:
		return net.ErrClosed
	default:
		close(l.closed)
	}
	r1, _, e := procClosesocket.Call(l.sock)
	if isSocketError(r1) {
		return os.NewSyscallError("closesocket", e)
	}
	return nil
}

func (l *vsockListener) Addr() net.Addr { return &l.addr }

// vsockConn implements net.Conn using nonblocking driver calls. A driver wait
// cannot be interrupted reliably by Close; polling also lets deadlines work.
type vsockConn struct {
	mu            sync.Mutex
	readMu        sync.Mutex
	writeMu       sync.Mutex
	sock          uintptr
	local         vsockAddr
	remote        vsockAddr
	closed        bool
	done          chan struct{}
	changed       chan struct{}
	readDeadline  time.Time
	writeDeadline time.Time
}

func newConn(sock uintptr, local, remote vsockAddr) *vsockConn {
	return &vsockConn{sock: sock, local: local, remote: remote, done: make(chan struct{}), changed: make(chan struct{})}
}

func (c *vsockConn) checkLocked(write bool) error {
	if c.closed {
		return net.ErrClosed
	}
	deadline := c.readDeadline
	if write {
		deadline = c.writeDeadline
	}
	if !deadline.IsZero() && !time.Now().Before(deadline) {
		return os.ErrDeadlineExceeded
	}
	return nil
}

func (c *vsockConn) wait(write bool) error {
	c.mu.Lock()
	if err := c.checkLocked(write); err != nil {
		c.mu.Unlock()
		return err
	}
	deadline, changed := c.readDeadline, c.changed
	if write {
		deadline = c.writeDeadline
	}
	delay := acceptPollInterval
	if !deadline.IsZero() && time.Until(deadline) < delay {
		delay = time.Until(deadline)
	}
	c.mu.Unlock()
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-c.done:
		return net.ErrClosed
	case <-changed:
	case <-timer.C:
	}
	return nil
}

func (c *vsockConn) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	c.readMu.Lock()
	defer c.readMu.Unlock()
	for {
		c.mu.Lock()
		if err := c.checkLocked(false); err != nil {
			c.mu.Unlock()
			return 0, err
		}
		r1, _, e := procRecv.Call(c.sock, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0)
		c.mu.Unlock()
		if !isSocketError(r1) {
			if int32(r1) == 0 {
				return 0, io.EOF
			}
			return int(int32(r1)), nil
		}
		if !errors.Is(e, windows.WSAEWOULDBLOCK) {
			return 0, os.NewSyscallError("recv", e)
		}
		if err := c.wait(false); err != nil {
			return 0, err
		}
	}
}

func (c *vsockConn) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	n := 0
	for n < len(b) {
		c.mu.Lock()
		if err := c.checkLocked(true); err != nil {
			c.mu.Unlock()
			return n, err
		}
		r1, _, e := procSend.Call(c.sock, uintptr(unsafe.Pointer(&b[n])), uintptr(len(b)-n), 0)
		c.mu.Unlock()
		if !isSocketError(r1) {
			if int32(r1) == 0 {
				return n, io.ErrShortWrite
			}
			n += int(int32(r1))
			continue
		}
		if !errors.Is(e, windows.WSAEWOULDBLOCK) {
			return n, os.NewSyscallError("send", e)
		}
		if err := c.wait(true); err != nil {
			return n, err
		}
	}
	return n, nil
}

func (c *vsockConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return net.ErrClosed
	}
	c.closed = true
	close(c.done)
	r1, _, e := procClosesocket.Call(c.sock)
	if isSocketError(r1) {
		return os.NewSyscallError("closesocket", e)
	}
	return nil
}

func (c *vsockConn) LocalAddr() net.Addr                { return &c.local }
func (c *vsockConn) RemoteAddr() net.Addr               { return &c.remote }
func (c *vsockConn) SetDeadline(t time.Time) error      { return c.setDeadline(t, true, true) }
func (c *vsockConn) SetReadDeadline(t time.Time) error  { return c.setDeadline(t, true, false) }
func (c *vsockConn) SetWriteDeadline(t time.Time) error { return c.setDeadline(t, false, true) }
func (c *vsockConn) setDeadline(t time.Time, read, write bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return net.ErrClosed
	}
	if read {
		c.readDeadline = t
	}
	if write {
		c.writeDeadline = t
	}
	close(c.changed)
	c.changed = make(chan struct{})
	return nil
}
