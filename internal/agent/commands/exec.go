package commands

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"time"

	"github.com/jimbersoftware/vex/internal/vmp"
)

const killedExitCode int32 = -1

func execCommand(ctx context.Context, log *slog.Logger, req *vmp.ExecRequest) *vmp.Response {
	log.Info("exec command received", "command", req.GetCommand())
	start := time.Now()

	if req.GetTimeoutSeconds() > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.GetTimeoutSeconds())*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, req.GetCommand(), req.GetArguments()...) //nolint:gosec

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	execResp := &vmp.ExecResponse{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}

	runErr := cmd.Run()
	elapsed := time.Since(start)
	execResp.Stdout = stdout.Bytes()
	execResp.Stderr = stderr.Bytes()

	if runErr == nil {
		log.Info("exec command completed", "command", req.GetCommand(), "duration", elapsed)
		return &vmp.Response{Result: &vmp.Response_Exec{Exec: execResp}}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		execResp.TimedOut = true
		execResp.ExitCode = killedExitCode
		log.Warn("exec command timed out", "command", req.GetCommand(), "duration", elapsed)
		return &vmp.Response{Result: &vmp.Response_Exec{Exec: execResp}}
	}
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) {
		execResp.ExitCode = killedExitCode
		log.Error("exec command failed", "command", req.GetCommand(), "duration", elapsed, "error", runErr)
		return &vmp.Response{
			Error:  runErr.Error(),
			Result: &vmp.Response_Exec{Exec: execResp},
		}
	}
	execResp.ExitCode = int32(exitErr.ExitCode()) //nolint:gosec // exit codes fit in int32
	log.Info("exec command completed", "command", req.GetCommand(), "duration", elapsed, "exitCode", execResp.GetExitCode())
	return &vmp.Response{Result: &vmp.Response_Exec{Exec: execResp}}
}
