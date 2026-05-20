package commands

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/jimbersoftware/vex/internal/vmp"
)

func hostInfo(_ *vmp.HostInfoRequest) *vmp.Response {
	return &vmp.Response{
		Result: &vmp.Response_HostInfo{
			HostInfo: &vmp.HostInfoResponse{
				Os:      runtime.GOOS,
				Arch:    runtime.GOARCH,
				Version: osVersion(),
			},
		},
	}
}

func osVersion() string {
	switch runtime.GOOS {
	case "linux":
		return linuxVersion()
	case "windows":
		return windowsVersion()
	default:
		return runtime.GOOS
	}
}

func linuxVersion() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
		}
	}
	return "unknown"
}

func windowsVersion() string {
	out, err := exec.Command( //nolint:gosec
		"powershell", "-NoProfile", "-Command",
		"(Get-WmiObject Win32_OperatingSystem).Caption",
	).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
