package store

import (
	"fmt"
	"os"

	"github.com/tbcrawford/opm/internal/symlink"
)

// RenameProfileResult captures the outcome of a profile rename.
type RenameProfileResult struct {
	WasActive       bool
	CurrentCacheErr error
}

// RenameProfileAndRetarget renames a profile and, if it was active, retargets
// the managed symlink and updates the current cache.
func (s *Store) RenameProfileAndRetarget(oldName, newName string) (RenameProfileResult, error) {
	if renameProfileAndRetargetOverride != nil {
		return renameProfileAndRetargetOverride(s, oldName, newName)
	}

	active, err := s.ActiveProfile()
	if err != nil {
		return RenameProfileResult{}, fmt.Errorf("determine active profile: %w", err)
	}

	result := RenameProfileResult{WasActive: active == oldName}

	if err := s.RenameProfile(oldName, newName); err != nil {
		return RenameProfileResult{}, err
	}

	if !result.WasActive {
		return result, nil
	}

	newDir := s.ProfileDir(newName)
	if err := symlink.SetAtomic(newDir, s.OpencodeDir()); err != nil {
		if rerr := os.Rename(s.ProfileDir(newName), s.ProfileDir(oldName)); rerr != nil {
			return RenameProfileResult{}, fmt.Errorf("update active symlink: %w; rollback also failed: %v — profile directory is at %q", err, rerr, newName)
		}
		return RenameProfileResult{}, fmt.Errorf("update active symlink: %w (rolled back directory rename)", err)
	}

	result.CurrentCacheErr = s.SetCurrent(newName)
	return result, nil
}
