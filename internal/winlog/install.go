//go:build windows

package winlog

import (
	"golang.org/x/sys/windows/svc/eventlog"
)

func EnsureSource(source string) {
	_ = eventlog.InstallAsEventCreate(source, eventlog.Info|eventlog.Warning|eventlog.Error)
}
