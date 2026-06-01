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
	log.Info("exec command received", "command", req.GetCommand(), "detach", req.GetDetach())

	if !req.GetDetach() && req.GetTimeoutSeconds() > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(req.GetTimeoutSeconds())*time.Second)
		defer cancel()
	}

	cmd, err := buildCommand(ctx, req)
	if err != nil {
		return &vmp.Response{
			Error:  err.Error(),
			Result: &vmp.Response_Exec{Exec: &vmp.ExecResponse{ExitCode: killedExitCode}},
		}
	}
	defer releaseRunAs(cmd)

	if req.GetDetach() {
		return runCommandDetached(log, cmd, req.GetCommand())
	}

	return runCommand(ctx, log, cmd, req)
}

func buildCommand(ctx context.Context, req *vmp.ExecRequest) (*exec.Cmd, error) {
	var cmd *exec.Cmd
	if req.GetDetach() {
		cmd = exec.Command(req.GetCommand(), req.GetArguments()...) //nolint:gosec
	} else {
		cmd = exec.CommandContext(ctx, req.GetCommand(), req.GetArguments()...) //nolint:gosec
	}
	if username := req.GetUsername(); username != "" {
		if err := configureRunAs(cmd, username); err != nil {
			return nil, err
		}
	}
	return cmd, nil
}

func runCommand(ctx context.Context, log *slog.Logger, cmd *exec.Cmd, req *vmp.ExecRequest) *vmp.Response {
	start := time.Now()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	elapsed := time.Since(start)

	execResp := &vmp.ExecResponse{
		Stdout: stdout.Bytes(),
		Stderr: stderr.Bytes(),
	}

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

func runCommandDetached(log *slog.Logger, cmd *exec.Cmd, command string) *vmp.Response {
	if err := cmd.Start(); err != nil {
		log.Error("exec detach failed to start", "command", command, "error", err)
		return &vmp.Response{
			Error:  err.Error(),
			Result: &vmp.Response_Exec{Exec: &vmp.ExecResponse{ExitCode: killedExitCode}},
		}
	}

	pid := int32(cmd.Process.Pid) //nolint:gosec // pids fit in int32
	log.Info("exec detached process started", "command", command, "pid", pid)

	go func() {
		_ = cmd.Wait()
	}()

	return &vmp.Response{Result: &vmp.Response_Exec{Exec: &vmp.ExecResponse{Pid: pid}}}
}
