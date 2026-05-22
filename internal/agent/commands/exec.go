package commands

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"runtime"
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
		log.Error("exec command failed", "command", req.GetCommand(), "duration", elapsed, "error", runErr)
		return &vmp.Response{Error: runErr.Error()}
	}
	execResp.ExitCode = int32(exitErr.ExitCode()) //nolint:gosec // exit codes fit in int32
	log.Info("exec command completed", "command", req.GetCommand(), "duration", elapsed, "exitCode", execResp.ExitCode)
	return &vmp.Response{Result: &vmp.Response_Exec{Exec: execResp}}
}
