package agent_test

import (
	"context"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jimbersoftware/vex/internal/agent"
)

type mockListener struct {
	conns  chan net.Conn
	closed chan struct{}
	once   sync.Once
}

func newMockListener() *mockListener {
	return &mockListener{
		conns:  make(chan net.Conn, 1),
		closed: make(chan struct{}),
	}
}

func (m *mockListener) Accept() (net.Conn, error) {
	select {
	case conn := <-m.conns:
		return conn, nil
	case <-m.closed:
		return nil, net.ErrClosed
	}
}

func (m *mockListener) Close() error {
	m.once.Do(func() { close(m.closed) })
	return nil
}

func (m *mockListener) Addr() net.Addr { return &net.TCPAddr{} }

func TestRun_ExitsCleanlyOnContextCancel(t *testing.T) {
	t.Parallel()

	listener := newMockListener()
	logger := slog.New(slog.DiscardHandler)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- agent.Run(ctx, listener, logger) }()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not exit after context cancellation")
	}
}

func TestRun_AcceptsConnection(t *testing.T) {
	t.Parallel()

	listener := newMockListener()
	logger := slog.New(slog.DiscardHandler)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	serverConn, clientConn := net.Pipe()
	listener.conns <- serverConn

	done := make(chan error, 1)
	go func() { done <- agent.Run(ctx, listener, logger) }()

	_, _ = clientConn.Write([]byte("hello"))
	clientConn.Close()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not exit")
	}
}
