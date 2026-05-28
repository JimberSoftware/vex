//go:build windows

package commands

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	modWtsapi32                  = windows.NewLazySystemDLL("wtsapi32.dll")
	modUserenv                   = windows.NewLazySystemDLL("userenv.dll")
	procWTSEnumerateSessionsW    = modWtsapi32.NewProc("WTSEnumerateSessionsW")
	procWTSQuerySessionInfoW     = modWtsapi32.NewProc("WTSQuerySessionInformationW")
	procWTSQueryUserToken        = modWtsapi32.NewProc("WTSQueryUserToken")
	procWTSFreeMemory            = modWtsapi32.NewProc("WTSFreeMemory")
	procCreateEnvironmentBlock   = modUserenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvironmentBlock  = modUserenv.NewProc("DestroyEnvironmentBlock")
	procGetUserProfileDirectoryW = modUserenv.NewProc("GetUserProfileDirectoryW")
)

const wtsActive = 0

type wtsSessionInfo struct {
	SessionID      uint32
	WinStationName *uint16
	State          uint32
}

func configureRunAs(cmd *exec.Cmd, username string) error {
	token, err := tokenForUser(username)
	if err != nil {
		return err
	}

	env, err := createEnvBlock(token)
	if err != nil {
		windows.CloseHandle(windows.Handle(token))
		return fmt.Errorf("creating environment for %q: %w", username, err)
	}

	profileDir, err := getUserProfileDir(token)
	if err != nil {
		windows.CloseHandle(windows.Handle(token))
		return fmt.Errorf("getting profile dir for %q: %w", username, err)
	}

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Token: syscall.Token(token),
	}
	cmd.Dir = profileDir
	cmd.Env = env

	return nil
}

func tokenForUser(username string) (windows.Token, error) {
	var sessionInfo *wtsSessionInfo
	var count uint32

	r1, _, err := procWTSEnumerateSessionsW.Call(
		0,
		0,
		1,
		uintptr(unsafe.Pointer(&sessionInfo)),
		uintptr(unsafe.Pointer(&count)),
	)
	if r1 == 0 {
		return 0, fmt.Errorf("WTSEnumerateSessions: %w", err)
	}
	defer procWTSFreeMemory.Call(uintptr(unsafe.Pointer(sessionInfo)))

	sessions := unsafe.Slice(sessionInfo, count)
	for _, s := range sessions {
		if s.State != wtsActive {
			continue
		}
		sessionUser, err := querySessionUser(s.SessionID)
		if err != nil {
			continue
		}
		if !strings.EqualFold(sessionUser, username) {
			continue
		}

		var token windows.Token
		r1, _, err := procWTSQueryUserToken.Call(
			uintptr(s.SessionID),
			uintptr(unsafe.Pointer(&token)),
		)
		if r1 == 0 {
			return 0, fmt.Errorf("WTSQueryUserToken for session %d: %w", s.SessionID, err)
		}

		var dupToken windows.Token
		err2 := windows.DuplicateTokenEx(
			token,
			windows.TOKEN_ALL_ACCESS,
			nil,
			windows.SecurityImpersonation,
			windows.TokenPrimary,
			&dupToken,
		)
		windows.CloseHandle(windows.Handle(token))
		if err2 != nil {
			return 0, fmt.Errorf("DuplicateTokenEx: %w", err2)
		}
		return dupToken, nil
	}

	return 0, fmt.Errorf("user %q has no active session", username)
}

func querySessionUser(sessionID uint32) (string, error) {
	var buffer *uint16
	var bytesReturned uint32

	r1, _, err := procWTSQuerySessionInfoW.Call(
		0,
		uintptr(sessionID),
		5,
		uintptr(unsafe.Pointer(&buffer)),
		uintptr(unsafe.Pointer(&bytesReturned)),
	)
	if r1 == 0 {
		return "", err
	}
	defer procWTSFreeMemory.Call(uintptr(unsafe.Pointer(buffer)))

	return windows.UTF16PtrToString(buffer), nil
}

func createEnvBlock(token windows.Token) ([]string, error) {
	var envBlock *uint16
	r1, _, err := procCreateEnvironmentBlock.Call(
		uintptr(unsafe.Pointer(&envBlock)),
		uintptr(token),
		0,
	)
	if r1 == 0 {
		return nil, err
	}
	defer procDestroyEnvironmentBlock.Call(uintptr(unsafe.Pointer(envBlock)))

	var env []string
	ptr := unsafe.Pointer(envBlock)
	for {
		entry := windows.UTF16PtrToString((*uint16)(ptr))
		if entry == "" {
			break
		}
		env = append(env, entry)

		offset := 0
		for *(*uint16)(unsafe.Add(ptr, offset*2)) != 0 {
			offset++
		}
		ptr = unsafe.Add(ptr, (offset+1)*2)
	}
	return env, nil
}

func releaseRunAs(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil || cmd.SysProcAttr.Token == 0 {
		return
	}
	windows.CloseHandle(windows.Handle(cmd.SysProcAttr.Token))
	cmd.SysProcAttr.Token = 0
}

func getUserProfileDir(token windows.Token) (string, error) {
	var size uint32
	procGetUserProfileDirectoryW.Call(uintptr(token), 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return "", fmt.Errorf("GetUserProfileDirectory returned size 0")
	}

	buf := make([]uint16, size)
	r1, _, err := procGetUserProfileDirectoryW.Call(
		uintptr(token),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r1 == 0 {
		return "", err
	}
	return windows.UTF16ToString(buf), nil
}
