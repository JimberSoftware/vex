//go:build windows

package commands

import (
	"fmt"
	"os/exec"
	"runtime"
)

func configureRunAs(_ *exec.Cmd, _ string) error {
	return fmt.Errorf("exec as user is not supported on %s", runtime.GOOS)
}
