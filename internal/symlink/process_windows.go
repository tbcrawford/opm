//go:build windows

package symlink

import (
	"syscall"

	"golang.org/x/sys/windows"
)

func processExists(pid int) bool {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return err == syscall.ERROR_ACCESS_DENIED
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	return true
}
