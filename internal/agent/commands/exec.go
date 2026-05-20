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

func execCommand(req *vmp.ExecRequest) *vmp.Response {
	ctx := context.Background()
	if req.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-Command", req.Command) //nolint:gosec
	} else {
		cmd = exec.CommandContext(ctx, "/bin/bash", "-c", req.Command) //nolint:gosec
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	execResp := &vmp.ExecResponse{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}

	if runErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			execResp.TimedOut = true
			execResp.ExitCode = killedExitCode
		} else {
			var exitErr *exec.ExitError
			if errors.As(runErr, &exitErr) {
				execResp.ExitCode = int32(exitErr.ExitCode())
			} else {
				return &vmp.Response{Error: runErr.Error()}
			}
		}
	}

	return &vmp.Response{
		Result: &vmp.Response_Exec{Exec: execResp},
	}
}
