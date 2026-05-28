//go:build !linux && !windows

package vsock

import (
	"fmt"
	"net"
	"runtime"
)

func Listen(_, _ uint32) (net.Listener, error) {
	return nil, fmt.Errorf("vsock is not supported on %s", runtime.GOOS)
}

func Dial(_, _ uint32) (net.Conn, error) {
	return nil, fmt.Errorf("vsock is not supported on %s", runtime.GOOS)
}
