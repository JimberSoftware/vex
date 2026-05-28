//go:build linux

package commands

import (
	"fmt"
	"os/exec"
	"os/user"
	"strconv"
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
	for _, g := range groups {
		gidVal, err := strconv.ParseUint(g, 10, 32)
		if err != nil {
			continue
		}
		supplementary = append(supplementary, uint32(gidVal))
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:    uint32(uid),
			Gid:    uint32(gid),
			Groups: supplementary,
		},
	}
	cmd.Dir = usr.HomeDir
	cmd.Env = []string{
		"HOME=" + usr.HomeDir,
		"USER=" + usr.Username,
		"LOGNAME=" + usr.Username,
		"PATH=/usr/local/bin:/usr/bin:/bin",
	}

	return nil
}

func releaseRunAs(_ *exec.Cmd) {}
