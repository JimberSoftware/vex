package client

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"

	"github.com/jimbersoftware/vex/internal/vmp"
	"github.com/jimbersoftware/vex/internal/vsock"
)

const uploadChunkSize = 1024 * 1024

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
	PID      int32
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

func (c *Client) Exec(command string, args []string, timeoutSeconds uint32, username string, detach bool) (ExecResult, error) {
	resp, err := c.send(&vmp.Request{Command: &vmp.Request_Exec{Exec: &vmp.ExecRequest{
		Command:        command,
		Arguments:      args,
		TimeoutSeconds: timeoutSeconds,
		Username:       username,
		Detach:         detach,
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
		PID:      ex.Exec.GetPid(),
	}, err
}

func (c *Client) Upload(path string, content io.Reader, size uint64, mode uint32, checksum []byte) (uint64, error) {
	if _, err := c.sendUpload(&vmp.Request{Command: &vmp.Request_UploadStart{UploadStart: &vmp.UploadStartRequest{
		Path:   path,
		Mode:   mode,
		Size:   size,
		Sha256: checksum,
	}}}); err != nil {
		return 0, err
	}
	if err := c.uploadChunks(content); err != nil {
		return 0, err
	}
	upload, err := c.sendUpload(&vmp.Request{Command: &vmp.Request_UploadFinish{UploadFinish: &vmp.UploadFinishRequest{}}})
	if err != nil {
		return 0, err
	}
	return upload.GetBytesWritten(), nil
}

func (c *Client) uploadChunks(content io.Reader) error {
	buf := make([]byte, uploadChunkSize)
	for {
		n, readErr := io.ReadFull(content, buf)
		if n > 0 {
			chunk := append([]byte(nil), buf[:n]...)
			if _, err := c.sendUpload(&vmp.Request{Command: &vmp.Request_UploadChunk{UploadChunk: &vmp.UploadChunkRequest{
				Data: chunk,
			}}}); err != nil {
				return err
			}
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
				break
			}
			return fmt.Errorf("read upload content: %w", readErr)
		}
	}
	return nil
}

func (c *Client) sendUpload(req *vmp.Request) (*vmp.UploadResponse, error) {
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	upload, ok := resp.GetResult().(*vmp.Response_Upload)
	if !ok {
		return nil, errors.New("unexpected response type")
	}
	return upload.Upload, nil
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
