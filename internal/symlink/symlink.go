package symlink

import (
	"fmt"
	"os"
	"path/filepath"
)

// Status describes the state of a filesystem path that may be a symlink.
type Status struct {
	Exists    bool
	IsSymlink bool
	IsDir     bool
	Target    string // populated when IsSymlink is true
	Dangling  bool   // true when IsSymlink but target does not exist
}

// Inspect examines path using os.Lstat (never os.Stat — must not follow symlinks).
func Inspect(path string) (Status, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return Status{Exists: false}, nil
	}
	if err != nil {
		return Status{}, fmt.Errorf("lstat %s: %w", path, err)
	}

	isSymlink := info.Mode()&os.ModeSymlink != 0
	s := Status{
		Exists:    true,
		IsSymlink: isSymlink,
		IsDir:     info.IsDir(),
	}

	if isSymlink {
		target, err := os.Readlink(path)
		if err != nil {
			return Status{}, fmt.Errorf("readlink %s: %w", path, err)
		}
		s.Target = target
		// Check if target actually exists (detect dangling symlink).
		// os.Stat follows the symlink — if it errors with NotExist, the link is dangling.
		if _, err := os.Stat(path); os.IsNotExist(err) {
			s.Dangling = true
		}
	}
	return s, nil
}

// SetAtomic atomically installs or replaces a symlink at linkPath pointing to target.
// Uses create-tmp + rename pattern: never a remove-then-create window.
// Per D-13: os.Symlink(target, tmp) then os.Rename(tmp, dst).
func SetAtomic(target, linkPath string) error {
	dir := filepath.Dir(linkPath)
	tmpLink := filepath.Join(dir, ".opm-tmp-"+filepath.Base(linkPath))

	// Clean up any leftover tmp from a prior crash.
	_ = os.Remove(tmpLink)

	if err := os.Symlink(target, tmpLink); err != nil {
		return fmt.Errorf("create temp symlink: %w", err)
	}

	if err := os.Rename(tmpLink, linkPath); err != nil {
		_ = os.Remove(tmpLink) // best-effort cleanup
		return fmt.Errorf("atomic swap: %w", err)
	}
	return nil
}
