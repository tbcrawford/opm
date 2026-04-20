//go:build windows

package symlink

import (
	"errors"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

func swapLink(tmpLink, linkPath string) error {
	if swapLinkOverride != nil {
		return swapLinkOverride(tmpLink, linkPath)
	}

	for attempt := 0; attempt < 5; attempt++ {
		if err := swapLinkOnce(tmpLink, linkPath); err == nil {
			return nil
		} else if !isRetryableSwapErr(err) || attempt == 4 {
			return err
		}
		time.Sleep(time.Duration(attempt+1) * 25 * time.Millisecond)
	}

	return nil
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
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}
