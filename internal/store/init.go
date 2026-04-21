package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tbcrawford/opm/internal/symlink"
)

// InitResult captures the outcome of a successful store initialization.
type InitResult struct {
	ProfileDir      string
	Migrated        bool
	Reinstated      bool // true when an existing profile was reconnected after opm reset
	CurrentCacheErr error
}

// InitNameError reports an invalid initial profile name passed to Initialize.
type InitNameError struct {
	Err error
}

func (e InitNameError) Error() string { return e.Err.Error() }

func (e InitNameError) Unwrap() error { return e.Err }

// Initialize bootstraps opm management for opencodeDir, optionally migrating an
// existing directory into the requested profile.
func (s *Store) Initialize(profileName string) (InitResult, error) {
	if err := ValidateName(profileName); err != nil {
		return InitResult{}, InitNameError{Err: err}
	}

	opencodeDir := s.OpencodeDir()
	profileDir := s.ProfileDir(profileName)

	st, err := symlink.Inspect(opencodeDir)
	if err != nil {
		return InitResult{}, fmt.Errorf("inspect %s: %w", opencodeDir, err)
	}

	if st.IsSymlink {
		managed, err := s.IsOpmManaged()
		if err != nil {
			return InitResult{}, fmt.Errorf("cannot determine opm state: %w", err)
		}
		if managed {
			return InitResult{}, s.alreadyInitializedError()
		}
		return InitResult{}, fmt.Errorf("%s is an unrecognized symlink\n\n  Back it up and remove it, then run 'opm init' again", s.displayPath(s.OpencodeDir()))
	}

	if st.Exists && !st.IsDir && !st.IsSymlink {
		return InitResult{}, fmt.Errorf("%s is not a directory or symlink\n\n  Back it up and remove it, then run 'opm init' again", s.displayPath(s.OpencodeDir()))
	}

	result, resumed, err := s.checkForPartialInit(profileName, opencodeDir+".opm-new", profileDir, st.Exists)
	if err != nil {
		return InitResult{}, err
	}
	if resumed {
		return result, nil
	}

	if err := s.Init(); err != nil {
		return InitResult{}, fmt.Errorf("create opm dirs: %w", err)
	}

	if st.IsDir {
		return s.migrateExistingOpencodeDir(profileName, profileDir, opencodeDir+".opm-new")
	}

	return s.createInitialProfile(profileName, profileDir)
}

func (s *Store) alreadyInitializedError() error {
	st, err := symlink.Inspect(s.OpencodeDir())
	if err != nil {
		return fmt.Errorf("inspect %s: %w", s.OpencodeDir(), err)
	}
	activeName := filepath.Base(st.Target)
	return fmt.Errorf("already initialized (active: %s)", activeName)
}

func (s *Store) checkForPartialInit(profileName, tmpSym, profileDir string, opencodeExists bool) (InitResult, bool, error) {
	_, tmpErr := os.Lstat(tmpSym)
	tmpExists := tmpErr == nil

	if !tmpExists {
		if _, statErr := os.Lstat(profileDir); statErr == nil {
			managed, mErr := s.IsOpmManaged()
			if mErr == nil && managed {
				return InitResult{}, false, s.alreadyInitializedError()
			}
			// opencodeDir exists as a plain directory: this is the post-reset state
			// (opm reset copied the active profile back to opencodeDir and left profiles
			// intact). Remove the plain directory and reinstate the symlink.
			if opencodeExists {
				if err := os.RemoveAll(s.OpencodeDir()); err != nil {
					return InitResult{}, false, fmt.Errorf("remove plain opencode dir: %w", err)
				}
				if err := symlink.SetAtomic(profileDir, s.OpencodeDir()); err != nil {
					return InitResult{}, false, fmt.Errorf("reinstate symlink: %w", err)
				}
				return InitResult{
					ProfileDir:      profileDir,
					Migrated:        false,
					Reinstated:      true,
					CurrentCacheErr: s.SetCurrent(profileName),
				}, true, nil
			}
			// opencodeDir does not exist but the profile dir does — genuinely unexpected
			// partial state that requires manual recovery.
			return InitResult{}, false, fmt.Errorf(
				"partial initialization detected: %s exists but %s is not managed by opm\n\n"+
					"  To recover:\n"+
					"    rm -rf %s\n"+
					"    opm init",
				s.displayPath(profileDir), s.displayPath(s.OpencodeDir()), s.displayPath(profileDir),
			)
		}
		return InitResult{}, false, nil
	}

	tmpSt, err := symlink.Inspect(tmpSym)
	if err != nil {
		return InitResult{}, false, fmt.Errorf("inspect %s: %w", tmpSym, err)
	}

	var profileIsDir bool
	profileInfo, profileStatErr := os.Lstat(profileDir)
	profileExists := profileStatErr == nil
	if profileExists {
		profileIsDir = profileInfo.IsDir()
	} else if !os.IsNotExist(profileStatErr) {
		return InitResult{}, false, fmt.Errorf("inspect %s: %w", profileDir, profileStatErr)
	}

	if !profileExists || !tmpSt.IsSymlink || tmpSt.Target != profileDir {
		return InitResult{}, false, fmt.Errorf(
			"stale interrupted init state detected\n\n  To recover:\n    remove %s\n    opm init",
			s.displayPath(tmpSym),
		)
	}

	if !profileIsDir {
		return InitResult{}, false, fmt.Errorf(
			"partial initialization detected: %s exists but is not a directory\n\n"+
				"  To recover:\n"+
				"    rm -rf %s\n"+
				"    opm init",
			s.displayPath(profileDir), s.displayPath(profileDir),
		)
	}
	if opencodeExists {
		return InitResult{}, false, fmt.Errorf(
			"partial initialization detected: %s exists but %s still exists\n\n"+
				"  To recover:\n"+
				"    inspect and back up %s\n"+
				"    remove %s\n"+
				"    remove %s\n"+
				"    opm init",
			s.displayPath(profileDir), s.displayPath(s.OpencodeDir()), s.displayPath(s.OpencodeDir()), s.displayPath(s.OpencodeDir()), s.displayPath(tmpSym),
		)
	}

	if err := os.Rename(tmpSym, s.OpencodeDir()); err != nil {
		return InitResult{}, false, fmt.Errorf("resume: atomic rename symlink: %w", err)
	}

	return InitResult{
		ProfileDir:      profileDir,
		Migrated:        true,
		CurrentCacheErr: s.SetCurrent(profileName),
	}, true, nil
}

func (s *Store) migrateExistingOpencodeDir(profileName, profileDir, tmpSym string) (InitResult, error) {
	if err := symlink.SetAtomic(profileDir, tmpSym); err != nil {
		return InitResult{}, fmt.Errorf("step 1 — create temp symlink: %w", err)
	}
	if err := os.Rename(s.OpencodeDir(), profileDir); err != nil {
		_ = os.Remove(tmpSym)
		return InitResult{}, fmt.Errorf("step 2 — move opencode dir to profile: %w", err)
	}
	if err := os.Rename(tmpSym, s.OpencodeDir()); err != nil {
		return InitResult{}, fmt.Errorf("step 3 — install symlink: %w", err)
	}

	return InitResult{
		ProfileDir:      profileDir,
		Migrated:        true,
		CurrentCacheErr: s.SetCurrent(profileName),
	}, nil
}

func (s *Store) createInitialProfile(profileName, profileDir string) (InitResult, error) {
	if err := os.MkdirAll(profileDir, 0o755); err != nil {
		return InitResult{}, fmt.Errorf("create %s profile: %w", profileName, err)
	}
	if err := symlink.SetAtomic(profileDir, s.OpencodeDir()); err != nil {
		return InitResult{}, fmt.Errorf("install symlink: %w", err)
	}

	return InitResult{
		ProfileDir:      profileDir,
		Migrated:        false,
		CurrentCacheErr: s.SetCurrent(profileName),
	}, nil
}

func (s *Store) displayPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if len(path) > len(home) && path[:len(home)] == home && os.IsPathSeparator(path[len(home)]) {
		return "~" + path[len(home):]
	}
	return path
}
