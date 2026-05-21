package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jimbersoftware/vex/internal/server"
	"github.com/spf13/cobra"
)

func main() {
	var (
		listen string
		port   uint32
	)

	cmd := &cobra.Command{
		Use:   "vexd",
		Short: "HTTP daemon for remote vex-agent access",
		RunE: func(_ *cobra.Command, _ []string) error {
			log := slog.Default()

			srv := &server.Server{
				Dialer: server.DefaultDialer,
				Port:   port,
				Log:    log,
			}

			httpSrv := &http.Server{
				Addr:              listen,
				Handler:           srv.Handler(),
				ReadHeaderTimeout: 5 * time.Second, //nolint:mnd
			}

			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			go func() {
				<-ctx.Done()
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second) //nolint:mnd
				defer cancel()
				_ = httpSrv.Shutdown(shutdownCtx) //nolint:contextcheck
			}()

			log.Info("listening", "addr", listen, "vsock_port", port)
			if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&listen, "listen", ":8080", "HTTP listen address")
	cmd.Flags().Uint32Var(&port, "port", 1024, "vsock port for agent connections") //nolint:mnd

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
