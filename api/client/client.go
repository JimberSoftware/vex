package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jimbersoftware/vex/api"
)

const defaultTimeout = 30 * time.Second

type Client struct {
	baseURL    string
	httpClient *http.Client
	headers    map[string]string
}

func New(baseURL string, opts ...Option) *Client {
	cl := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: defaultTimeout},
		headers:    make(map[string]string),
	}
	for _, opt := range opts {
		opt(cl)
	}
	return cl
}

func (cl *Client) Ping(ctx context.Context, cid uint32) error {
	resp, err := cl.do(ctx, http.MethodPost, cl.vmPath(cid, "ping"), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (cl *Client) HostInfo(ctx context.Context, cid uint32) (*api.HostInfoResponse, error) {
	resp, err := cl.do(ctx, http.MethodPost, cl.vmPath(cid, "host-info"), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result api.HostInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode host-info response: %w", err)
	}
	return &result, nil
}

func (cl *Client) Exec(ctx context.Context, cid uint32, req api.ExecRequest) (*api.ExecResponse, error) {
	if req.TimeoutSeconds > 0 {
		deadline := time.Now().Add(time.Duration(req.TimeoutSeconds)*time.Second + 30*time.Second)
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}

	resp, err := cl.do(ctx, http.MethodPost, cl.vmPath(cid, "exec"), req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result api.ExecResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode exec response: %w", err)
	}
	return &result, nil
}

func (cl *Client) vmPath(cid uint32, endpoint string) string {
	return cl.baseURL + "/vms/" + strconv.FormatUint(uint64(cid), 10) + "/" + endpoint
}

func (cl *Client) do(ctx context.Context, method, url string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, val := range cl.headers {
		req.Header.Set(key, val)
	}

	resp, err := cl.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var errResp api.ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Error != "" {
			apiErr.Message = errResp.Error
		} else {
			apiErr.Message = http.StatusText(resp.StatusCode)
		}
		return nil, apiErr
	}

	return resp, nil
}
