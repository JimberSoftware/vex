package commands

import (
	"bufio"
	"context"
	"log/slog"
	"net"

	"github.com/jimbersoftware/vex/internal/vmp"
)

func Handle(ctx context.Context, conn net.Conn, log *slog.Logger) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	for {
		req, err := vmp.ReadRequest(br)
		if err != nil {
			return
		}
		resp := dispatch(ctx, log, req)
		resp.Id = req.GetId()
		if err := vmp.WriteResponse(conn, resp); err != nil {
			return
		}
	}
}

func dispatch(ctx context.Context, log *slog.Logger, req *vmp.Request) *vmp.Response {
	switch cmd := req.GetCommand().(type) {
	case *vmp.Request_Ping:
		return ping(cmd.Ping)
	case *vmp.Request_HostInfo:
		return hostInfo(ctx, cmd.HostInfo)
	case *vmp.Request_Exec:
		return execCommand(ctx, log, cmd.Exec)
	default:
		return &vmp.Response{Error: "unknown command"}
	}
}
