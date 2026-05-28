// Package api defines the public HTTP request and response types for vexd.
package api

type ExecRequest struct {
	Command        string   `json:"command"`
	Arguments      []string `json:"arguments,omitempty"`
	TimeoutSeconds uint32   `json:"timeout_seconds,omitempty"`
	Username       string   `json:"username,omitempty"`
}

type ExecResponse struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int32  `json:"exit_code"`
	TimedOut bool   `json:"timed_out"`
	Error    string `json:"error,omitempty"`
}

type HostInfoResponse struct {
	OS      string `json:"os"`
	Version string `json:"version"`
	Arch    string `json:"arch"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
