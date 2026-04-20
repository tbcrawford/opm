//go:build windows

package symlink

import (
	"errors"
	"os"
	"syscall"

	"golang.org/x/sys/windows"
)

func swapLink(tmpLink, linkPath string) error {
	if swapLinkOverride != nil {
		return swapLinkOverride(tmpLink, linkPath)
	}

	return retrySwap(12, swapLinkSleepOverride, func() error {
		return swapLinkOnce(tmpLink, linkPath)
	}, isRetryableSwapErr)
}

func swapLinkOnce(tmpLink, linkPath string) error {
	info, err := os.Lstat(linkPath)
	if os.IsNotExist(err) {
		return os.Rename(tmpLink, linkPath)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return syscall.ERROR_ACCESS_DENIED
	}
	if err := os.Remove(linkPath); err != nil {
		if os.IsNotExist(err) {
			return os.Rename(tmpLink, linkPath)
		}
		return err
	}
	return os.Rename(tmpLink, linkPath)
}

func isRetryableSwapErr(err error) bool {
	return os.IsPermission(err) || errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
