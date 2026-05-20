package commands

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"runtime"
	"time"

	"github.com/jimbersoftware/vex/internal/vmp"
)

const killedExitCode int32 = -1

func execCommand(ctx context.Context, req *vmp.ExecRequest) *vmp.Response {
	if req.GetTimeoutSeconds() > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.GetTimeoutSeconds())*time.Second)
		defer cancel()
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-Command", req.GetCommand()) //nolint:gosec
	} else {
		cmd = exec.CommandContext(ctx, "/bin/bash", "-c", req.GetCommand()) //nolint:gosec
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	execResp := &vmp.ExecResponse{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}

	runErr := cmd.Run()
	execResp.Stdout = stdout.Bytes()
	execResp.Stderr = stderr.Bytes()

	if runErr == nil {
		return &vmp.Response{Result: &vmp.Response_Exec{Exec: execResp}}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		execResp.TimedOut = true
		execResp.ExitCode = killedExitCode
		return &vmp.Response{Result: &vmp.Response_Exec{Exec: execResp}}
	}
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		return &vmp.Response{Error: runErr.Error()}
	}
	execResp.ExitCode = int32(exitErr.ExitCode()) //nolint:gosec // exit codes fit in int32
	return &vmp.Response{Result: &vmp.Response_Exec{Exec: execResp}}
}
