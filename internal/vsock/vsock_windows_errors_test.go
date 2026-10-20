//go:build windows

package vsock

import "testing"

// This test also builds against the previous socket implementation, so the
// Windows error-width regression can be verified without the listener changes.
func TestConnectionErrorsDoNotBecomeByteCounts(t *testing.T) {
	for _, operation := range []string{"read", "write"} {
		t.Run(operation, func(t *testing.T) {
			c := &vsockConn{sock: errSock}
			var n int
			var err error
			if operation == "read" {
				n, err = c.Read(make([]byte, 16))
			} else {
				n, err = c.Write([]byte("test"))
			}
			if n != 0 || err == nil {
				t.Fatalf("%s on invalid socket = %d, %v; want 0 and an error", operation, n, err)
			}
		})
	}
}
