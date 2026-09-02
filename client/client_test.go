package client_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"log/slog"
	"net"
	"os"
	"path/filepath"
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

	ex, err := c.Exec("echo", []string{"hello"}, 0, "", false)
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

func TestClientExecDetach(t *testing.T) {
	t.Parallel()
	c, done := agentClient(t)
	defer done()

	ex, err := c.Exec("sleep", []string{"60"}, 0, "", true)
	if err != nil {
		t.Fatalf("Exec detach: %v", err)
	}
	if ex.PID <= 0 {
		t.Errorf("expected positive PID, got %d", ex.PID)
	}
}

func TestClientUpload(t *testing.T) {
	t.Parallel()
	agent, done := agentClient(t)
	defer done()

	content := bytes.Repeat([]byte("native test binary"), 150000)
	checksum := sha256.Sum256(content)
	destination := filepath.Join(t.TempDir(), "integration.test")

	written, err := agent.Upload(destination, bytes.NewReader(content), uint64(len(content)), 0o750, checksum[:])
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if written != uint64(len(content)) {
		t.Fatalf("bytes written: got %d, want %d", written, len(content))
	}
	got, err := os.ReadFile(destination)
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("uploaded content: got %q, want %q", got, content)
	}
}

func TestClientUploadRejectsChecksumMismatch(t *testing.T) {
	t.Parallel()
	c, done := agentClient(t)
	defer done()

	content := []byte("native test binary")
	destination := filepath.Join(t.TempDir(), "integration.test")
	_, err := c.Upload(destination, bytes.NewReader(content), uint64(len(content)), 0o750, make([]byte, sha256.Size))
	if err == nil {
		t.Fatal("expected checksum mismatch")
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatalf("destination should not exist after failed upload: %v", statErr)
	}
}
