package client_test

import (
	"context"
	"log/slog"
	"net"
	"testing"

	"github.com/jimbersoftware/vex/client"
	"github.com/jimbersoftware/vex/internal/agent/commands"
)

func agentClient(t *testing.T) (*client.Client, func()) {
	t.Helper()
	server, conn := net.Pipe()
	go commands.Handle(context.Background(), server, slog.Default())
	c := client.NewFromConn(conn)
	return c, func() { c.Close() }
}

func TestClientPing(t *testing.T) {
	t.Parallel()
	c, done := agentClient(t)
	defer done()

	if err := c.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestClientHostInfo(t *testing.T) {
	t.Parallel()
	c, done := agentClient(t)
	defer done()

	hi, err := c.HostInfo()
	if err != nil {
		t.Fatalf("HostInfo: %v", err)
	}
	if hi.OS == "" {
		t.Error("OS should not be empty")
	}
	if hi.Arch == "" {
		t.Error("Arch should not be empty")
	}
}

func TestClientExec(t *testing.T) {
	t.Parallel()
	c, done := agentClient(t)
	defer done()

	ex, err := c.Exec("echo hello", 0)
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if ex.ExitCode != 0 {
		t.Errorf("exit_code: got %d, want 0", ex.ExitCode)
	}
	if string(ex.Stdout) == "" {
		t.Error("stdout should not be empty")
	}
}
