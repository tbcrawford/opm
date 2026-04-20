//go:build !windows

package symlink

import "golang.org/x/sys/unix"

func processExists(pid int) bool {
	err := unix.Kill(pid, 0)
	return err == nil || err == unix.EPERM
}
