package client

import (
	"bufio"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/jimbersoftware/vex/internal/vmp"
	"github.com/jimbersoftware/vex/internal/vsock"
)

type Client struct {
	conn net.Conn
	br   *bufio.Reader
	seq  atomic.Uint64
}

func Dial(cid, port uint32) (*Client, error) {
	conn, err := vsock.Dial(cid, port)
	if err != nil {
		return nil, fmt.Errorf("dial cid=%d port=%d: %w", cid, port, err)
	}
	return &Client{conn: conn, br: bufio.NewReader(conn)}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Ping() error {
	resp, err := c.send(&vmp.Request{Command: &vmp.Request_Ping{Ping: &vmp.PingRequest{}}})
	if err != nil {
		return err
	}
	if _, ok := resp.Result.(*vmp.Response_Ping); !ok {
		return fmt.Errorf("unexpected response type")
	}
	return nil
}

func (c *Client) HostInfo() (*vmp.HostInfoResponse, error) {
	resp, err := c.send(&vmp.Request{Command: &vmp.Request_HostInfo{HostInfo: &vmp.HostInfoRequest{}}})
	if err != nil {
		return nil, err
	}
	hi, ok := resp.Result.(*vmp.Response_HostInfo)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}
	return hi.HostInfo, nil
}

func (c *Client) Exec(command string, timeoutSeconds uint32) (*vmp.ExecResponse, error) {
	resp, err := c.send(&vmp.Request{Command: &vmp.Request_Exec{Exec: &vmp.ExecRequest{
		Command:        command,
		TimeoutSeconds: timeoutSeconds,
	}}})
	if err != nil {
		return nil, err
	}
	ex, ok := resp.Result.(*vmp.Response_Exec)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}
	return ex.Exec, nil
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
	if resp.Error != "" {
		return nil, fmt.Errorf("agent: %s", resp.Error)
	}
	return resp, nil
}
