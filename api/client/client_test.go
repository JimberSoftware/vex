package client_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

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

func TestUpload_Success(t *testing.T) {
	t.Parallel()
	content := []byte("native test binary")
	checksum := sha256.Sum256(content)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.URL.Path != "/vms/3/files" || r.URL.Query().Get("path") != "/tmp/integration.test" {
			t.Errorf("unexpected upload URL: %s", r.URL.String())
		}
		if r.Header.Get(api.UploadModeHeader) != "750" {
			t.Errorf("unexpected mode: %q", r.Header.Get(api.UploadModeHeader))
		}
		if r.Header.Get(api.UploadSHA256Header) != hex.EncodeToString(checksum[:]) {
			t.Errorf("unexpected checksum: %q", r.Header.Get(api.UploadSHA256Header))
		}
		got, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read upload: %v", err)
		}
		if !bytes.Equal(got, content) {
			t.Errorf("unexpected content: %q", got)
		}
		_ = json.NewEncoder(w).Encode(api.UploadResponse{BytesWritten: uint64(len(got))})
	}))
	defer srv.Close()

	cl := vexclient.New(srv.URL)
	result, err := cl.Upload(context.Background(), 3, api.UploadRequest{
		Path:   "/tmp/integration.test",
		Mode:   0o750,
		Size:   uint64(len(content)),
		SHA256: hex.EncodeToString(checksum[:]),
	}, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BytesWritten != uint64(len(content)) {
		t.Errorf("unexpected bytes written: %d", result.BytesWritten)
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

func TestExec_DefaultTimeoutExpires(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			select {
			case <-time.After(35 * time.Second):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"exit_code":0}`)),
				}, nil
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		})
		cl := vexclient.New("http://localhost", vexclient.WithHTTPClient(&http.Client{Transport: transport}))

		_, err := cl.Exec(t.Context(), 3, api.ExecRequest{
			Command: "date",
		})
		if err == nil {
			t.Fatal("expected timeout: default 30s deadline should expire before 35s response")
		}
	})
}

func TestExec_CustomTimeoutDoesNotExpireEarly(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
			select {
			case <-time.After(35 * time.Second):
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/json"}},
					Body:       io.NopCloser(strings.NewReader(`{"exit_code":0}`)),
				}, nil
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
		})
		cl := vexclient.New("http://localhost", vexclient.WithHTTPClient(&http.Client{Transport: transport}))

		_, err := cl.Exec(t.Context(), 3, api.ExecRequest{
			Command:        "date",
			TimeoutSeconds: 35,
		})
		if err != nil {
			t.Fatalf("custom timeout (35s) should not have timed out: %v", err)
		}
	})
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
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
