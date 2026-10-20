//go:build windows

package vsock

import (
	"errors"
	"net"
	"os"
	"testing"
	"time"

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

func TestConnectionDeadlines(t *testing.T) {
	c := newConn(errSock, vsockAddr{}, vsockAddr{})
	if err := c.SetDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if n, err := c.Read(make([]byte, 1)); n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("Read past deadline = %d, %v", n, err)
	}
	if n, err := c.Write([]byte{1}); n != 0 || !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("Write past deadline = %d, %v", n, err)
	}
}

func TestClosedConnectionRejectsIO(t *testing.T) {
	c := newConn(errSock, vsockAddr{}, vsockAddr{})
	_ = c.Close() // The intentionally invalid OS handle cannot be closed.
	if n, err := c.Read(make([]byte, 1)); n != 0 || !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Read after Close = %d, %v", n, err)
	}
	if n, err := c.Write([]byte{1}); n != 0 || !errors.Is(err, net.ErrClosed) {
		t.Fatalf("Write after Close = %d, %v", n, err)
	}
	if err := c.SetDeadline(time.Now()); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("SetDeadline after Close = %v", err)
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
