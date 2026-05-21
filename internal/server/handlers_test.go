package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jimbersoftware/vex/api"
	"github.com/jimbersoftware/vex/client"
)

type mockClient struct {
	pingErr     error
	hostInfo    client.HostInfo
	hostInfoErr error
	execResult  client.ExecResult
	execErr     error
}

func (m *mockClient) Ping() error                        { return m.pingErr }
func (m *mockClient) HostInfo() (client.HostInfo, error) { return m.hostInfo, m.hostInfoErr }
func (m *mockClient) Exec(_ string, _ uint32) (client.ExecResult, error) {
	return m.execResult, m.execErr
}
func (m *mockClient) Close() error { return nil }

func mockDialer(mc *mockClient) Dialer {
	return func(_, _ uint32) (AgentClient, error) {
		return mc, nil
	}
}

func failDialer(err error) Dialer {
	return func(_, _ uint32) (AgentClient, error) {
		return nil, err
	}
}

func newTestServer(d Dialer) *Server {
	return &Server{Dialer: d, Port: 1024}
}

func TestHandlePing(t *testing.T) {
	t.Parallel()
	mc := &mockClient{}
	srv := newTestServer(mockDialer(mc))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/vms/3/ping", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "{}\n" {
		t.Fatalf("expected empty JSON object, got %q", rec.Body.String())
	}
}

func TestHandlePing_InvalidCID(t *testing.T) {
	t.Parallel()
	mc := &mockClient{}
	srv := newTestServer(mockDialer(mc))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/vms/notanumber/ping", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandlePing_DialFailure(t *testing.T) {
	t.Parallel()
	srv := &Server{Dialer: failDialer(errors.New("connection refused")), Port: 1024}

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/vms/3/ping", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleHostInfo(t *testing.T) {
	t.Parallel()
	mc := &mockClient{
		hostInfo: client.HostInfo{OS: "linux", Version: "6.1.0", Arch: "x86_64"},
	}
	srv := newTestServer(mockDialer(mc))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/vms/3/host-info", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["os"] != "linux" || resp["version"] != "6.1.0" || resp["arch"] != "x86_64" {
		t.Fatalf("unexpected response: %v", resp)
	}
}

func TestHandleExec(t *testing.T) {
	t.Parallel()
	mc := &mockClient{
		execResult: client.ExecResult{
			Stdout:   []byte("hello\n"),
			Stderr:   []byte(""),
			ExitCode: 0,
			TimedOut: false,
		},
	}
	srv := newTestServer(mockDialer(mc))

	body := strings.NewReader(`{"command":"echo hello","timeout_seconds":5}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/vms/3/exec", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp api.ExecResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Stdout != "hello\n" {
		t.Fatalf("unexpected stdout: %q", resp.Stdout)
	}
	if resp.ExitCode != 0 {
		t.Fatalf("unexpected exit code: %d", resp.ExitCode)
	}
}

func TestHandleExec_MissingCommand(t *testing.T) {
	t.Parallel()
	mc := &mockClient{}
	srv := newTestServer(mockDialer(mc))

	body := strings.NewReader(`{}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/vms/3/exec", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleExec_TimedOut(t *testing.T) {
	t.Parallel()
	mc := &mockClient{
		execResult: client.ExecResult{
			Stdout:   []byte("partial"),
			Stderr:   []byte(""),
			ExitCode: -1,
			TimedOut: true,
		},
	}
	srv := newTestServer(mockDialer(mc))

	body := strings.NewReader(`{"command":"sleep 999","timeout_seconds":1}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/vms/3/exec", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp api.ExecResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.TimedOut {
		t.Fatal("expected timed_out=true")
	}
}

func TestHandleExec_DialFailure(t *testing.T) {
	t.Parallel()
	srv := &Server{Dialer: failDialer(errors.New("connection refused")), Port: 1024}

	body := strings.NewReader(`{"command":"ls"}`)
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/vms/3/exec", body)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", rec.Code, rec.Body.String())
	}
}
