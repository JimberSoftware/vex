package commands

import (
	"bufio"
	"net"

	"github.com/jimbersoftware/vex/internal/vmp"
)

func Handle(conn net.Conn) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	for {
		req, err := vmp.ReadRequest(br)
		if err != nil {
			return
		}
		resp := dispatch(req)
		resp.Id = req.Id
		if err := vmp.WriteResponse(conn, resp); err != nil {
			return
		}
	}
}

func dispatch(req *vmp.Request) *vmp.Response {
	switch cmd := req.Command.(type) {
	case *vmp.Request_Ping:
		return ping(cmd.Ping)
	case *vmp.Request_HostInfo:
		return hostInfo(cmd.HostInfo)
	case *vmp.Request_Exec:
		return execCommand(cmd.Exec)
	default:
		return &vmp.Response{Error: "unknown command"}
	}
}
