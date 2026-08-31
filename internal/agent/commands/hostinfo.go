package commands

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/jimbersoftware/vex/internal/vmp"
)

const (
	unknownVersion        = "unknown"
	windowsVersionTimeout = 5 * time.Second
)

func hostInfo(ctx context.Context, _ *vmp.HostInfoRequest) *vmp.Response {
	return &vmp.Response{
		Result: &vmp.Response_HostInfo{
			HostInfo: &vmp.HostInfoResponse{
				Os:      runtime.GOOS,
				Arch:    runtime.GOARCH,
				Version: osVersion(ctx),
			},
		},
	}
}

func osVersion(ctx context.Context) string {
	switch runtime.GOOS {
	case "linux":
		return linuxVersion()
	case "darwin":
		return darwinVersion(ctx)
	case "windows":
		return windowsVersion(ctx)
	default:
		return runtime.GOOS
	}
}

func darwinVersion(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, windowsVersionTimeout)
	defer cancel()
	output, err := exec.CommandContext(ctx, "/usr/bin/sw_vers", "-productVersion").Output()
	if err != nil {
		return unknownVersion
	}
	return strings.TrimSpace(string(output))
}

func linuxVersion() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return unknownVersion
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
		}
	}
	return unknownVersion
}

func windowsVersion(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, windowsVersionTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx,
		"powershell", "-NoProfile", "-Command",
		"(Get-WmiObject Win32_OperatingSystem).Caption",
	).Output()
	if err != nil {
		return unknownVersion
	}
	return strings.TrimSpace(string(out))
}
