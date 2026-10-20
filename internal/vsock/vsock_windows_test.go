//go:build windows

package vsock

import (
	"errors"
	"net"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestSocketErrorWidth(t *testing.T) {
	for _, result := range []uintptr{uintptr(^uint32(0)), ^uintptr(0)} {
		if !isSocketError(result) {
			t.Fatalf("SOCKET_ERROR %#x was treated as a byte count", result)
		}
	}
	for _, result := range []uintptr{0, 1, 4096} {
		if isSocketError(result) {
			t.Fatalf("valid byte count %d was treated as SOCKET_ERROR", result)
		}
	}
}

// Exercise a delayed request through the installed driver, including several
// receive timeouts. These timeouts must not end an otherwise healthy connection.
func TestAcceptedReadRetriesDriverTimeout(t *testing.T) {
	ln, err := Listen(^uint32(0), 32802)
	if errors.Is(err, windows.WSAEAFNOSUPPORT) || errors.Is(err, windows.WSAEPROTONOSUPPORT) {
		t.Skipf("VirtIO socket driver unavailable: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	var local rawSockaddrVM
	length := int32(unsafe.Sizeof(local))
	r, _, e := modws2_32.NewProc("getsockname").Call(ln.(*vsockListener).sock, uintptr(unsafe.Pointer(&local)), uintptr(unsafe.Pointer(&length)))
	if isSocketError(r) {
		t.Fatal(e)
	}
	accepted := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- c
	}()
	peer, err := Dial(local.CID, 32802)
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	var conn net.Conn
	select {
	case conn = <-accepted:
	case err := <-acceptErr:
		t.Fatal(err)
	case <-time.After(2 * time.Second):
		t.Fatal("loopback connection was not accepted")
	}
	defer conn.Close()
	done := make(chan error, 1)
	go func() {
		b := make([]byte, 1)
		n, err := conn.Read(b)
		if err == nil && (n != 1 || b[0] != 42) {
			err = errors.New("unexpected payload after driver read timeout")
		}
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("idle Read returned before the delayed payload: %v", err)
	case <-time.After(3 * readPollInterval):
	}
	if _, err := peer.Write([]byte{42}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Read did not receive the delayed payload")
	}
	// Finish both directions before closing loopback driver handles.
	_ = windows.Shutdown(windows.Handle(peer.(*vsockConn).sock), windows.SHUT_RDWR)
	_ = windows.Shutdown(windows.Handle(conn.(*vsockConn).sock), windows.SHUT_RDWR)
}

func TestClosedConnectionRejectsIO(t *testing.T) {
	c := &vsockConn{sock: errSock}
	_ = c.Close() // The intentionally invalid OS handle cannot be closed.
	if n, err := c.Read(make([]byte, 1)); n != 0 || !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Read after Close = %d, %v", n, err)
	}
	if n, err := c.Write([]byte{1}); n != 0 || !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Write after Close = %d, %v", n, err)
	}
}

// Run on a guest with the VirtIO socket driver to exercise the real accept
// path. Ordinary Windows CI hosts do not expose AF_VSOCK.
func TestListenerCloseUnblocksAccept(t *testing.T) {
	ln, err := Listen(^uint32(0), 32800)
	if errors.Is(err, windows.WSAEAFNOSUPPORT) || errors.Is(err, windows.WSAEPROTONOSUPPORT) {
		t.Skipf("VirtIO socket driver unavailable: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	done := make(chan error, 1)
	go func() {
		_, err := ln.Accept()
		done <- err
	}()
	// Leave Accept idle in the driver before closing it.
	time.Sleep(3 * acceptPollInterval)
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, net.ErrClosed) {
			t.Fatalf("Accept after Close = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Close left Accept stuck")
	}
	if _, err := ln.Accept(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Accept on a closed listener = %v", err)
	}
}

func TestPendingDriverAcceptCanBeCancelled(t *testing.T) {
	ln, err := Listen(^uint32(0), 32801)
	if errors.Is(err, windows.WSAEAFNOSUPPORT) || errors.Is(err, windows.WSAEPROTONOSUPPORT) {
		t.Skipf("VirtIO socket driver unavailable: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	listener := ln.(*vsockListener)
	var peer rawSockaddrVM
	peerLen := int32(16)
	done := make(chan error, 1)
	go func() { _, err := acceptWithCancellation(listener.sock, &peer, &peerLen); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("empty listener accept unexpectedly succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("pending driver accept was not cancelled")
	}
}
