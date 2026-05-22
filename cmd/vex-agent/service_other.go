//go:build !windows

package main

import (
	"context"
	"os/signal"
	"syscall"
)

func runApp() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return run(ctx)
}
