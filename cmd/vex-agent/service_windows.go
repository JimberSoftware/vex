//go:build windows

package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"golang.org/x/sys/windows/svc"
)

const serviceName = "VexAgent"

type agentService struct{}

func (s *agentService) Execute(_ []string, r <-chan svc.ChangeRequest, status chan<- svc.Status) (bool, uint32) {
	status <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- run(ctx)
	}()

	status <- svc.Status{
		State:   svc.Running,
		Accepts: svc.AcceptStop | svc.AcceptShutdown,
	}

	for {
		select {
		case err := <-errCh:
			if err != nil {
				status <- svc.Status{State: svc.StopPending}
				return true, 1
			}
			status <- svc.Status{State: svc.StopPending}
			return false, 0

		case cr := <-r:
			switch cr.Cmd {
			case svc.Stop, svc.Shutdown:
				status <- svc.Status{State: svc.StopPending}
				cancel()
				<-errCh
				return false, 0
			case svc.Interrogate:
				status <- cr.CurrentStatus
			}
		}
	}
}

func runApp() error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return fmt.Errorf("detecting service mode: %w", err)
	}

	if isService {
		return svc.Run(serviceName, &agentService{})
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return run(ctx)
}
