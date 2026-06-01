package commands_test

import (
	"bufio"
	"context"
	"log/slog"
	"net"
	"os"
	"os/user"
	"strings"
	"testing"

	"github.com/jimbersoftware/vex/internal/agent/commands"
	"github.com/jimbersoftware/vex/internal/vmp"
)

func sendAndReceive(t *testing.T, req *vmp.Request) *vmp.Response {
	t.Helper()

	server, client := net.Pipe()
	defer client.Close()

	go commands.Handle(context.Background(), server, slog.New(slog.DiscardHandler))

	if err := vmp.WriteRequest(client, req); err != nil {
		t.Fatalf("WriteRequest: %v", err)
	}

	resp, err := vmp.ReadResponse(bufio.NewReader(client))
	if err != nil {
		t.Fatalf("ReadResponse: %v", err)
	}
	return resp
}

func TestHandle_Ping(t *testing.T) {
	t.Parallel()

	resp := sendAndReceive(t, &vmp.Request{
		Id:      1,
		Command: &vmp.Request_Ping{Ping: &vmp.PingRequest{}},
	})
	if resp.GetError() != "" {
		t.Fatalf("unexpected error: %s", resp.GetError())
	}
	if _, ok := resp.GetResult().(*vmp.Response_Ping); !ok {
		t.Error("expected PingResponse")
	}
	if resp.GetId() != 1 {
		t.Errorf("id: got %d, want 1", resp.GetId())
	}
}

func TestHandle_HostInfo(t *testing.T) {
	t.Parallel()

	resp := sendAndReceive(t, &vmp.Request{
		Id:      2,
		Command: &vmp.Request_HostInfo{HostInfo: &vmp.HostInfoRequest{}},
	})
	if resp.GetError() != "" {
		t.Fatalf("unexpected error: %s", resp.GetError())
	}
	hi, ok := resp.GetResult().(*vmp.Response_HostInfo)
	if !ok {
		t.Fatal("expected HostInfoResponse")
	}
	if hi.HostInfo.GetOs() == "" {
		t.Error("Os should not be empty")
	}
	if hi.HostInfo.GetArch() == "" {
		t.Error("Arch should not be empty")
	}
}

func TestHandle_Exec(t *testing.T) {
	t.Parallel()

	resp := sendAndReceive(t, &vmp.Request{
		Id: 3,
		Command: &vmp.Request_Exec{Exec: &vmp.ExecRequest{
			Command:   "echo",
			Arguments: []string{"hello"},
		}},
	})
	if resp.GetError() != "" {
		t.Fatalf("unexpected error: %s", resp.GetError())
	}
	ex, ok := resp.GetResult().(*vmp.Response_Exec)
	if !ok {
		t.Fatal("expected ExecResponse")
	}
	if ex.Exec.GetExitCode() != 0 {
		t.Errorf("exit_code: got %d, want 0", ex.Exec.GetExitCode())
	}
	if got := strings.TrimSpace(string(ex.Exec.GetStdout())); got != "hello" {
		t.Errorf("stdout: got %q, want %q", got, "hello")
	}
}

func TestHandle_ExecAsUser(t *testing.T) {
	t.Parallel()

	currentUser, err := user.Current()
	if err != nil {
		t.Skip("cannot determine current user")
	}
	if os.Getuid() != 0 {
		t.Skip("requires root to switch user")
	}

	resp := sendAndReceive(t, &vmp.Request{
		Id: 7,
		Command: &vmp.Request_Exec{Exec: &vmp.ExecRequest{
			Command:  "whoami",
			Username: currentUser.Username,
		}},
	})
	if resp.GetError() != "" {
		t.Fatalf("unexpected error: %s", resp.GetError())
	}
	ex, ok := resp.GetResult().(*vmp.Response_Exec)
	if !ok {
		t.Fatal("expected ExecResponse")
	}
	got := strings.TrimSpace(string(ex.Exec.GetStdout()))
	if got != currentUser.Username {
		t.Errorf("whoami: got %q, want %q", got, currentUser.Username)
	}
}

func TestHandle_ExecAsUser_NotFound(t *testing.T) {
	t.Parallel()

	resp := sendAndReceive(t, &vmp.Request{
		Id: 8,
		Command: &vmp.Request_Exec{Exec: &vmp.ExecRequest{
			Command:  "whoami",
			Username: "nonexistent_user_xyzzy",
		}},
	})
	if resp.GetError() == "" {
		t.Fatal("expected error for nonexistent user")
	}
	if !strings.Contains(resp.GetError(), "not found") {
		t.Errorf("error should mention 'not found', got: %s", resp.GetError())
	}
}

func TestHandle_ExecNonZeroExit(t *testing.T) {
	t.Parallel()

	resp := sendAndReceive(t, &vmp.Request{
		Id: 4,
		Command: &vmp.Request_Exec{Exec: &vmp.ExecRequest{
			Command:   "/bin/bash",
			Arguments: []string{"-c", "exit 1"},
		}},
	})
	if resp.GetError() != "" {
		t.Fatalf("unexpected error: %s", resp.GetError())
	}
	ex, ok := resp.GetResult().(*vmp.Response_Exec)
	if !ok {
		t.Fatal("expected ExecResponse")
	}
	if ex.Exec.GetExitCode() != 1 {
		t.Errorf("exit_code: got %d, want 1", ex.Exec.GetExitCode())
	}
}

func TestHandle_ExecTimeout(t *testing.T) {
	t.Parallel()

	resp := sendAndReceive(t, &vmp.Request{
		Id: 5,
		Command: &vmp.Request_Exec{Exec: &vmp.ExecRequest{
			Command:        "sleep",
			Arguments:      []string{"10"},
			TimeoutSeconds: 1,
		}},
	})
	if resp.GetError() != "" {
		t.Fatalf("unexpected error: %s", resp.GetError())
	}
	ex, ok := resp.GetResult().(*vmp.Response_Exec)
	if !ok {
		t.Fatal("expected ExecResponse")
	}
	if !ex.Exec.GetTimedOut() {
		t.Error("expected timed_out = true")
	}
	if ex.Exec.GetExitCode() != -1 {
		t.Errorf("exit_code: got %d, want -1", ex.Exec.GetExitCode())
	}
}

func TestHandle_UnknownCommand(t *testing.T) {
	t.Parallel()

	resp := sendAndReceive(t, &vmp.Request{Id: 6})
	if resp.GetError() == "" {
		t.Error("expected error for nil command")
	}
}

func TestHandle_ExecDetach(t *testing.T) {
	t.Parallel()

	resp := sendAndReceive(t, &vmp.Request{
		Id: 9,
		Command: &vmp.Request_Exec{Exec: &vmp.ExecRequest{
			Command:   "sleep",
			Arguments: []string{"60"},
			Detach:    true,
		}},
	})
	if resp.GetError() != "" {
		t.Fatalf("unexpected error: %s", resp.GetError())
	}
	ex, ok := resp.GetResult().(*vmp.Response_Exec)
	if !ok {
		t.Fatal("expected ExecResponse")
	}
	if ex.Exec.GetPid() <= 0 {
		t.Errorf("expected positive pid, got %d", ex.Exec.GetPid())
	}
	if ex.Exec.GetExitCode() != 0 {
		t.Errorf("exit_code: got %d, want 0", ex.Exec.GetExitCode())
	}
}
