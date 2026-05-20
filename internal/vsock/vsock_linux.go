//go:build linux

package vsock

import (
	"net"

	"github.com/mdlayher/vsock"
)

func Listen(cid, port uint32) (net.Listener, error) {
	return vsock.ListenContextID(cid, port, nil)
}

func Dial(cid, port uint32) (net.Conn, error) {
	return vsock.Dial(cid, port, nil)
}
