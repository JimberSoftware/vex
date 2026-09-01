//go:build darwin

package commands

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"syscall"
)

func configureRunAs(cmd *exec.Cmd, username string) error {
	usr, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("user %q not found", username)
	}
	uid, err := strconv.ParseUint(usr.Uid, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid uid %q: %w", usr.Uid, err)
	}
	gid, err := strconv.ParseUint(usr.Gid, 10, 32)
	if err != nil {
		return fmt.Errorf("invalid gid %q: %w", usr.Gid, err)
	}
	groups, err := usr.GroupIds()
	if err != nil {
		return fmt.Errorf("resolving groups for %q: %w", username, err)
	}
	supplementary := make([]uint32, 0, len(groups))
	for _, group := range groups {
		groupID, err := strconv.ParseUint(group, 10, 32)
		if err == nil {
			supplementary = append(supplementary, uint32(groupID))
		}
	}

	credential := &syscall.Credential{
		Uid:    uint32(uid),
		Gid:    uint32(gid),
		Groups: supplementary,
	}
	tempDir, err := darwinUserTempDir(credential)
	if err != nil {
		return fmt.Errorf("resolving temporary directory for %q: %w", username, err)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: credential,
	}
	cmd.Dir = usr.HomeDir
	cmd.Env = []string{
		"HOME=" + usr.HomeDir,
		"USER=" + usr.Username,
		"LOGNAME=" + usr.Username,
		"PATH=/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin",
		"TMPDIR=" + tempDir,
	}
	return nil
}

func darwinUserTempDir(credential *syscall.Credential) (string, error) {
	cmd := exec.Command("/usr/bin/getconf", "DARWIN_USER_TEMP_DIR") //nolint:gosec,noctx // fixed system utility
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	tempDir := strings.TrimSpace(string(output))
	if tempDir == "" {
		return "", fmt.Errorf("getconf returned an empty path")
	}
	return tempDir, nil
}

func releaseRunAs(_ *exec.Cmd) {}
