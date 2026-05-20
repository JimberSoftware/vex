package vsock

import (
	"net"

	"github.com/mdlayher/vsock"
)

func Listen(cid, port uint32) (net.Listener, error) {
	return vsock.ListenContextID(cid, port, nil)
}
