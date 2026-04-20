//go:build windows

package symlink

import (
	"os"

	"golang.org/x/sys/windows"
)

func swapLink(tmpLink, linkPath string) error {
	tmpPtr, err := windows.UTF16PtrFromString(tmpLink)
	if err != nil {
		return err
	}
	dstPtr, err := windows.UTF16PtrFromString(linkPath)
	if err != nil {
		return err
	}

	err = windows.MoveFileEx(tmpPtr, dstPtr, windows.MOVEFILE_REPLACE_EXISTING)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpLink, linkPath)
}
