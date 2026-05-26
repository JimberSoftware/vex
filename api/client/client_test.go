package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jimbersoftware/vex/api"
	vexclient "github.com/jimbersoftware/vex/api/client"
)

func TestPing_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/vms/3/ping" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	cl := vexclient.New(srv.URL)
	err := cl.Ping(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPing_APIError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "connection refused"})
	}))
	defer srv.Close()

	cl := vexclient.New(srv.URL)
	err := cl.Ping(context.Background(), 5)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := vexclient.IsAPIError(err)
	if !ok {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadGateway {
		t.Errorf("expected 502, got %d", apiErr.StatusCode)
	}
	if apiErr.Message != "connection refused" {
		t.Errorf("unexpected message: %q", apiErr.Message)
	}
}

func TestHostInfo_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vms/7/host-info" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.HostInfoResponse{
			OS: "linux", Version: "6.1.0", Arch: "x86_64",
		})
	}))
	defer srv.Close()

	cl := vexclient.New(srv.URL)
	info, err := cl.HostInfo(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.OS != "linux" || info.Version != "6.1.0" || info.Arch != "x86_64" {
		t.Errorf("unexpected response: %+v", info)
	}
}

func TestExec_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/vms/3/exec" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req api.ExecRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Command != "echo" {
			t.Errorf("unexpected command: %q", req.Command)
		}
		if len(req.Arguments) != 1 || req.Arguments[0] != "hello" {
			t.Errorf("unexpected arguments: %v", req.Arguments)
		}
		if req.TimeoutSeconds != 5 {
			t.Errorf("unexpected timeout: %d", req.TimeoutSeconds)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.ExecResponse{
			Stdout:   "hello\n",
			Stderr:   "",
			ExitCode: 0,
			TimedOut: false,
		})
	}))
	defer srv.Close()

	cl := vexclient.New(srv.URL)
	result, err := cl.Exec(context.Background(), 3, api.ExecRequest{
		Command:        "echo",
		Arguments:      []string{"hello"},
		TimeoutSeconds: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("unexpected stdout: %q", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Errorf("unexpected exit code: %d", result.ExitCode)
	}
}

func TestExec_TimedOut(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(api.ExecResponse{
			Stdout:   "partial",
			Stderr:   "",
			ExitCode: -1,
			TimedOut: true,
		})
	}))
	defer srv.Close()

	cl := vexclient.New(srv.URL)
	result, err := cl.Exec(context.Background(), 3, api.ExecRequest{
		Command:        "sleep",
		Arguments:      []string{"999"},
		TimeoutSeconds: 1,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.TimedOut {
		t.Error("expected timed_out=true")
	}
}

func TestExec_APIError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "command is required"})
	}))
	defer srv.Close()

	cl := vexclient.New(srv.URL)
	_, err := cl.Exec(context.Background(), 3, api.ExecRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := vexclient.IsAPIError(err)
	if !ok {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", apiErr.StatusCode)
	}
	if apiErr.Message != "command is required" {
		t.Errorf("unexpected message: %q", apiErr.Message)
	}
}

func TestWithHeader(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("expected auth header, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	cl := vexclient.New(srv.URL, vexclient.WithHeader("Authorization", "Bearer secret"))
	err := cl.Ping(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithHTTPClient(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	custom := &http.Client{}
	cl := vexclient.New(srv.URL, vexclient.WithHTTPClient(custom))
	err := cl.Ping(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNetworkError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}))
	srv.Close()

	cl := vexclient.New(srv.URL)
	err := cl.Ping(context.Background(), 1)
	if err == nil {
		t.Fatal("expected error")
	}
	_, ok := vexclient.IsAPIError(err)
	if ok {
		t.Error("network errors should not be APIError")
	}
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cl := vexclient.New(srv.URL)
	err := cl.Ping(ctx, 1)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}
