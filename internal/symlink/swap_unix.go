//go:build !windows

package symlink

import "os"

func swapLink(tmpLink, linkPath string) error {
	if swapLinkOverride != nil {
		return swapLinkOverride(tmpLink, linkPath)
	}
	return os.Rename(tmpLink, linkPath)
}
