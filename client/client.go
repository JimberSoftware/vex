package client

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/jimbersoftware/vex/internal/vmp"
	"github.com/jimbersoftware/vex/internal/vsock"
)

type HostInfo struct {
	OS      string
	Version string
	Arch    string
}

type ExecResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int32
	TimedOut bool
}

type Client struct {
	conn net.Conn
	br   *bufio.Reader
	seq  atomic.Uint64
}

func New(cid, port uint32) (*Client, error) {
	conn, err := vsock.Dial(cid, port)
	if err != nil {
		return nil, fmt.Errorf("dial cid=%d port=%d: %w", cid, port, err)
	}
	return NewFromConn(conn), nil
}

func NewFromConn(conn net.Conn) *Client {
	return &Client{conn: conn, br: bufio.NewReader(conn)}
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Ping() error {
	resp, err := c.send(&vmp.Request{Command: &vmp.Request_Ping{Ping: &vmp.PingRequest{}}})
	if err != nil {
		return err
	}
	if _, ok := resp.GetResult().(*vmp.Response_Ping); !ok {
		return errors.New("unexpected response type")
	}
	return nil
}

func (c *Client) HostInfo() (HostInfo, error) {
	resp, err := c.send(&vmp.Request{Command: &vmp.Request_HostInfo{HostInfo: &vmp.HostInfoRequest{}}})
	if err != nil {
		return HostInfo{}, err
	}
	hi, ok := resp.GetResult().(*vmp.Response_HostInfo)
	if !ok {
		return HostInfo{}, errors.New("unexpected response type")
	}
	return HostInfo{
		OS:      hi.HostInfo.GetOs(),
		Version: hi.HostInfo.GetVersion(),
		Arch:    hi.HostInfo.GetArch(),
	}, nil
}

func (c *Client) Exec(command string, args []string, timeoutSeconds uint32, username string) (ExecResult, error) {
	resp, err := c.send(&vmp.Request{Command: &vmp.Request_Exec{Exec: &vmp.ExecRequest{
		Command:        command,
		Arguments:      args,
		TimeoutSeconds: timeoutSeconds,
		Username:       username,
	}}})
	if resp == nil {
		return ExecResult{}, err
	}
	ex, ok := resp.GetResult().(*vmp.Response_Exec)
	if !ok {
		if err != nil {
			return ExecResult{}, err
		}
		return ExecResult{}, errors.New("unexpected response type")
	}
	return ExecResult{
		Stdout:   ex.Exec.GetStdout(),
		Stderr:   ex.Exec.GetStderr(),
		ExitCode: ex.Exec.GetExitCode(),
		TimedOut: ex.Exec.GetTimedOut(),
	}, err
}

func (c *Client) send(req *vmp.Request) (*vmp.Response, error) {
	req.Id = c.seq.Add(1)
	if err := vmp.WriteRequest(c.conn, req); err != nil {
		return nil, err
	}
	resp, err := vmp.ReadResponse(c.br)
	if err != nil {
		return nil, err
	}
	if resp.GetError() != "" {
		return resp, fmt.Errorf("agent: %s", resp.GetError())
	}
	return resp, nil
}
