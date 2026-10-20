//go:build windows

package vsock

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	afVSock            = 40          // AF_VSOCK — exposed by virtio-win vsock driver
	sockStream         = 1           // SOCK_STREAM
	errSock            = ^uintptr(0) // pointer-sized INVALID_SOCKET
	acceptPollInterval = 10 * time.Millisecond
)

var (
	modws2_32               = windows.NewLazySystemDLL("ws2_32.dll")
	procWSASocket           = modws2_32.NewProc("WSASocketW")
	procBind                = modws2_32.NewProc("bind")
	procConnect             = modws2_32.NewProc("connect")
	procListen              = modws2_32.NewProc("listen")
	procAccept              = modws2_32.NewProc("accept")
	procClosesocket         = modws2_32.NewProc("closesocket")
	procRecv                = modws2_32.NewProc("recv")
	procSend                = modws2_32.NewProc("send")
	procSelect              = modws2_32.NewProc("select")
	procCancelSynchronousIo = windows.NewLazySystemDLL("kernel32.dll").NewProc("CancelSynchronousIo")
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

// Winsock int results are 32-bit even on 64-bit Windows, where SOCKET_ERROR
// may be zero-extended to 0x00000000ffffffff. Socket handles are pointer-sized.
func isSocketError(result uintptr) bool { return int32(result) == -1 }

// Winsock fd_set uses pointer-sized SOCKET entries after a 32-bit count.
// Go inserts the required alignment padding on 64-bit Windows.
type socketSet struct {
	count   uint32
	sockets [64]uintptr
}
type socketTimeval struct {
	seconds      int32
	microseconds int32
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
	if isSocketError(r1) {
		closeSocket(s)
		return nil, os.NewSyscallError("connect", e)
	}
	return &vsockConn{sock: s, remote: vsockAddr{CID: cid, Port: port}}, nil
}

// vsockListener implements net.Listener.
type vsockListener struct {
	mu     sync.Mutex
	sock   uintptr
	addr   vsockAddr
	closed bool
}

// A reset can consume readiness before accept opens its new driver handle.
// Cancel that pending file-create request on its issuing OS thread.
func acceptWithCancellation(sock uintptr, peer *rawSockaddrVM, peerLen *int32) (uintptr, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	thread, err := windows.OpenThread(windows.THREAD_TERMINATE, false, windows.GetCurrentThreadId())
	if err != nil {
		return errSock, err
	}
	defer func() { _ = windows.CloseHandle(thread) }()
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_, _, _ = procCancelSynchronousIo.Call(uintptr(thread))
			}
		}
	}()
	r1, _, e := procAccept.Call(sock, uintptr(unsafe.Pointer(peer)), uintptr(unsafe.Pointer(peerLen)))
	close(done)
	<-stopped
	return r1, e
}

func (l *vsockListener) Accept() (net.Conn, error) {
	for {
		l.mu.Lock()
		if l.closed {
			l.mu.Unlock()
			return nil, net.ErrClosed
		}
		ready := socketSet{count: 1}
		ready.sockets[0] = l.sock
		timeout := socketTimeval{microseconds: int32(acceptPollInterval / time.Microsecond)}
		// Do not leave an idle accept pending in the VirtIO driver. Keep sockets
		// blocking: the older driver cannot reliably switch accepted sockets back
		// to blocking mode, and its nonblocking send counter can become stuck.
		r1, _, e := procSelect.Call(0, uintptr(unsafe.Pointer(&ready)), 0, 0, uintptr(unsafe.Pointer(&timeout)))
		if isSocketError(r1) {
			l.mu.Unlock()
			// Older VirtIO providers report a select timeout as an error.
			if errors.Is(e, windows.WSAETIMEDOUT) {
				continue
			}
			return nil, os.NewSyscallError("select", e)
		}
		if r1 == 0 {
			l.mu.Unlock()
			continue
		}
		var peer rawSockaddrVM
		peerLen := int32(unsafe.Sizeof(peer))
		r1, e = acceptWithCancellation(l.sock, &peer, &peerLen)
		l.mu.Unlock()
		if r1 == errSock {
			if errors.Is(e, windows.WSAECONNRESET) || errors.Is(e, windows.WSAECONNABORTED) || errors.Is(e, windows.ERROR_OPERATION_ABORTED) || errors.Is(e, windows.WSAEINTR) || errors.Is(e, syscall.Errno(windows.STATUS_CANCELLED)) {
				continue
			}
			return nil, os.NewSyscallError("accept", e)
		}
		return &vsockConn{sock: r1, local: l.addr, remote: vsockAddr{CID: peer.CID, Port: peer.Port}}, nil
	}
}

func (l *vsockListener) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return net.ErrClosed
	}
	l.closed = true
	r1, _, e := procClosesocket.Call(l.sock)
	if isSocketError(r1) {
		return os.NewSyscallError("closesocket", e)
	}
	return nil
}

func (l *vsockListener) Addr() net.Addr { return &l.addr }

// vsockConn implements net.Conn.
type vsockConn struct {
	sock    uintptr
	local   vsockAddr
	remote  vsockAddr
	closed  atomic.Bool
	writeMu sync.Mutex
}

func (c *vsockConn) Read(b []byte) (int, error) {
	if c.closed.Load() {
		return 0, net.ErrClosed
	}
	if len(b) == 0 {
		return 0, nil
	}
	r1, _, e := procRecv.Call(c.sock, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0)
	if isSocketError(r1) {
		return 0, os.NewSyscallError("recv", e)
	}
	if r1 == 0 {
		return 0, io.EOF
	}
	return int(r1), nil
}

func (c *vsockConn) Write(b []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	n := 0
	for n < len(b) {
		if c.closed.Load() {
			return n, net.ErrClosed
		}
		r1, _, e := procSend.Call(c.sock, uintptr(unsafe.Pointer(&b[n])), uintptr(len(b)-n), 0)
		if isSocketError(r1) {
			return n, os.NewSyscallError("send", e)
		}
		if r1 == 0 {
			return n, io.ErrShortWrite
		}
		n += int(r1)
	}
	return n, nil
}

func (c *vsockConn) Close() error {
	if c.closed.Swap(true) {
		return net.ErrClosed
	}
	r1, _, e := procClosesocket.Call(c.sock)
	if isSocketError(r1) {
		return os.NewSyscallError("closesocket", e)
	}
	return nil
}

func (c *vsockConn) LocalAddr() net.Addr                { return &c.local }
func (c *vsockConn) RemoteAddr() net.Addr               { return &c.remote }
func (c *vsockConn) SetDeadline(_ time.Time) error      { return nil }
func (c *vsockConn) SetReadDeadline(_ time.Time) error  { return nil }
func (c *vsockConn) SetWriteDeadline(_ time.Time) error { return nil }
