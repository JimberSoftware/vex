package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jimbersoftware/vex/internal/agent"
	"github.com/jimbersoftware/vex/internal/vsock"
	"github.com/spf13/cobra"
)

const (
	defaultPort uint32 = 1024
	defaultCID  uint32 = ^uint32(0) // VMADDR_CID_ANY
)

func main() {
	var (
		port uint32
		cid  uint32
	)

	cmd := &cobra.Command{
		Use:   "vex-agent",
		Short: "Guest-side vex agent",
		RunE: func(_ *cobra.Command, _ []string) error {
			log := slog.Default()

			ln, err := vsock.Listen(cid, port)
			if err != nil {
				return err
			}
			log.Info("listening", "cid", cid, "port", port)

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			return agent.Run(ctx, ln, log)
		},
	}

	cmd.Flags().Uint32Var(&port, "port", defaultPort, "vsocket port to listen on")
	cmd.Flags().Uint32Var(&cid, "cid", defaultCID, "vsocket context ID to bind (default: VMADDR_CID_ANY)")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
