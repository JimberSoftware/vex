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
	upload := &fileUpload{}
	defer upload.close()
	br := bufio.NewReader(conn)
	for {
		req, err := vmp.ReadRequest(br)
		if err != nil {
			return
		}
		resp := dispatch(ctx, log, upload, req)
		resp.Id = req.GetId()
		if err := vmp.WriteResponse(conn, resp); err != nil {
			return
		}
	}
}

func dispatch(ctx context.Context, log *slog.Logger, upload *fileUpload, req *vmp.Request) *vmp.Response {
	switch cmd := req.GetCommand().(type) {
	case *vmp.Request_Ping:
		return ping(cmd.Ping)
	case *vmp.Request_HostInfo:
		return hostInfo(ctx, cmd.HostInfo)
	case *vmp.Request_Exec:
		return execCommand(ctx, log, cmd.Exec)
	case *vmp.Request_UploadStart:
		return upload.start(cmd.UploadStart)
	case *vmp.Request_UploadChunk:
		return upload.write(cmd.UploadChunk)
	case *vmp.Request_UploadFinish:
		return upload.finish()
	default:
		return &vmp.Response{Error: "unknown command"}
	}
}
