// Package agent implements the guest-side command execution logic.
package agent

import (
	"context"
	"io"
	"log/slog"
	"net"
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
		go drain(conn)
	}
}

func drain(conn net.Conn) {
	defer conn.Close()
	_, _ = io.Copy(io.Discard, conn)
}
