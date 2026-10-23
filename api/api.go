// Package api defines the public HTTP request and response types for vexd.
package api

const (
	UploadModeHeader   = "X-Vex-File-Mode"
	UploadSHA256Header = "X-Vex-File-Sha256"
)

type ExecRequest struct {
	Command        string   `json:"command"`
	Arguments      []string `json:"arguments,omitempty"`
	TimeoutSeconds uint32   `json:"timeout_seconds,omitempty"`
	Username       string   `json:"username,omitempty"`
	Detach         bool     `json:"detach,omitempty"`
}

type ExecResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int32  `json:"exit_code"`
	TimedOut bool   `json:"timed_out"`
	Error    string `json:"error,omitempty"`
	PID      int32  `json:"pid,omitempty"`
}

type HostInfoResponse struct {
	OS      string `json:"os"`
	Version string `json:"version"`
	Arch    string `json:"arch"`
}

type UploadRequest struct {
	Path   string
	Mode   uint32
	Size   uint64
	SHA256 string
}

type UploadResponse struct {
	BytesWritten uint64 `json:"bytes_written"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
