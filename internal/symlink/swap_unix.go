//go:build !windows

package symlink

import "os"

func swapLink(tmpLink, linkPath string) error {
	return os.Rename(tmpLink, linkPath)
}
