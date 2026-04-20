package symlink

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	base := filepath.Base(linkPath)
	legacyTmp := filepath.Join(dir, ".opm-tmp-"+base)
	currentPrefix := ".opm-tmp-" + base + "-" + strconv.Itoa(os.Getpid()) + "-"

	// Clean up the legacy shared tmp name from older runs.
	_ = removeTempSymlinkIfSafe(legacyTmp)

	matches, err := filepath.Glob(filepath.Join(dir, ".opm-tmp-"+base+"-*"))
	if err != nil {
		return fmt.Errorf("match temp symlinks: %w", err)
	}
	for _, match := range matches {
		if !shouldRemoveTemp(filepath.Base(match), base) {
			continue
		}
		_ = removeTempSymlinkIfSafe(match)
	}

	tmpFile, err := os.CreateTemp(dir, currentPrefix+"*")
	if err != nil {
		return fmt.Errorf("create temp placeholder: %w", err)
	}
	tmpLink := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpLink)
		return fmt.Errorf("close temp placeholder: %w", err)
	}
	if err := os.Remove(tmpLink); err != nil {
		return fmt.Errorf("remove temp placeholder: %w", err)
	}

	if err := os.Symlink(target, tmpLink); err != nil {
		return fmt.Errorf("create temp symlink: %w", err)
	}

	if err := swapLink(tmpLink, linkPath); err != nil {
		_ = os.Remove(tmpLink) // best-effort cleanup
		return fmt.Errorf("atomic swap: %w", err)
	}
	return nil
}

func removeTempSymlinkIfSafe(path string) error {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return nil
	}
	return os.Remove(path)
}

func shouldRemoveTemp(name, base string) bool {
	prefix := ".opm-tmp-" + base + "-"
	if !strings.HasPrefix(name, prefix) {
		return false
	}

	rest := strings.TrimPrefix(name, prefix)
	pidPart, _, ok := strings.Cut(rest, "-")
	if !ok || pidPart == "" {
		return true
	}

	pid, err := strconv.Atoi(pidPart)
	if err != nil || pid <= 0 {
		return true
	}

	return !processExists(pid)
}
