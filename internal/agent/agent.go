package agent

import (
	"context"
	"log/slog"
	"net"

	"github.com/jimbersoftware/vex/internal/agent/commands"
)

func Run(ctx context.Context, ln net.Listener, log *slog.Logger) error {
	go func() {
		<-ctx.Done()
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		log.Info("connection accepted", "remote", conn.RemoteAddr())
		go commands.Handle(conn)
	}
}
