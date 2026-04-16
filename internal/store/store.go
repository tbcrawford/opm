package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/opm-cli/opm/internal/symlink"
)

// validName enforces profile name safety — per D-17.
// Mirrors docker context naming: alphanumeric start, allows _ . - , max 63 chars.
var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.\-]{0,62}$`)

// Profile represents a discovered opm profile.
type Profile struct {
	Name     string
	Path     string
	Active   bool
	Dangling bool // true when the active symlink points to this profile but its directory is missing
}

// Store manages all opm state under a configurable root directory.
// In production, root = paths.OpmDir(). In tests, root = t.TempDir().
type Store struct {
	root        string
	opencodeDir string // path to ~/.config/opencode (the managed symlink)
}

// New creates a Store backed by the given root directory.
// opencodeDir is the path to ~/.config/opencode (for active-profile detection).
func New(root, opencodeDir string) *Store {
	return &Store{root: root, opencodeDir: opencodeDir}
}

func (s *Store) profilesDir() string { return filepath.Join(s.root, "profiles") }
func (s *Store) currentFile() string { return filepath.Join(s.root, "current") }

// ProfileDir returns the absolute path for a named profile.
func (s *Store) ProfileDir(name string) string {
	return filepath.Join(s.profilesDir(), name)
}

// Init creates the opm directory structure. Idempotent.
func (s *Store) Init() error {
	return os.MkdirAll(s.profilesDir(), 0o755)
}

// ValidateName checks whether name passes the allowlist. Returns nil if valid.
func ValidateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: must match [a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}", name)
	}
	return nil
}

// GetCurrent reads the active profile name from the current file.
// Returns "" (no error) if the file doesn't exist yet.
func (s *Store) GetCurrent() (string, error) {
	data, err := os.ReadFile(s.currentFile())
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read current: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

// SetCurrent writes the active profile name to the current file.
func (s *Store) SetCurrent(name string) error {
	if err := os.WriteFile(s.currentFile(), []byte(name+"\n"), 0o644); err != nil {
		return fmt.Errorf("write current: %w", err)
	}
	return nil
}

// ActiveProfile derives the active profile name from os.Readlink on the managed symlink.
// Per D-12: authoritative source is the actual symlink, not the current file.
func (s *Store) ActiveProfile() (string, error) {
	target, err := os.Readlink(s.opencodeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("readlink: %w", err)
	}
	// Target is an absolute path like ~/.config/opm/profiles/work.
	// Extract the profile name as the last path component.
	return filepath.Base(target), nil
}

// ListProfiles scans the profiles directory and returns all profiles.
// Active profile is determined by Readlink on the managed symlink (per D-12).
func (s *Store) ListProfiles() ([]Profile, error) {
	active, err := s.ActiveProfile()
	if err != nil {
		// Non-fatal: just means we can't determine active profile.
		active = ""
	}

	entries, err := os.ReadDir(s.profilesDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}

	var profiles []Profile
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		profiles = append(profiles, Profile{
			Name:   name,
			Path:   s.ProfileDir(name),
			Active: name == active,
		})
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	// Check for dangling active profile: symlink points to a dir that no longer exists in profiles/.
	// This happens when a user manually deletes a profile directory.
	activeTarget, readlinkErr := os.Readlink(s.opencodeDir)
	if readlinkErr == nil {
		activeName := filepath.Base(activeTarget)
		found := false
		for _, p := range profiles {
			if p.Name == activeName {
				found = true
				break
			}
		}
		if !found {
			// Synthesize a dangling entry so ls can surface it.
			profiles = append(profiles, Profile{
				Name:     activeName,
				Path:     activeTarget,
				Active:   true,
				Dangling: true,
			})
			sort.Slice(profiles, func(i, j int) bool {
				return profiles[i].Name < profiles[j].Name
			})
		}
	}

	return profiles, nil
}

// GetProfile returns the profile directory path if the profile exists.
func (s *Store) GetProfile(name string) (string, error) {
	dir := s.ProfileDir(name)
	fi, err := os.Lstat(dir)
	if os.IsNotExist(err) || (err == nil && !fi.IsDir()) {
		return "", fmt.Errorf("context %q does not exist", name)
	}
	if err != nil {
		return "", fmt.Errorf("stat profile %q: %w", name, err)
	}
	return dir, nil
}

// CreateProfile creates a new empty profile directory.
// Validates the name before touching the filesystem.
func (s *Store) CreateProfile(name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}
	dir := s.ProfileDir(name)
	if _, err := os.Lstat(dir); err == nil {
		return fmt.Errorf("context %q already exists", name)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create profile %q: %w", name, err)
	}
	return nil
}

// DeleteProfile removes a profile directory.
// If force is false, refuses to delete the active profile.
// If force is true, the caller is responsible for switching the symlink first (per D-01/D-02/D-03).
func (s *Store) DeleteProfile(name string, force bool) error {
	if !force {
		active, err := s.ActiveProfile()
		if err == nil && active == name {
			return fmt.Errorf("cannot remove active context %q — switch to another context first, or use --force", name)
		}
	}
	dir := s.ProfileDir(name)
	if _, err := os.Lstat(dir); os.IsNotExist(err) {
		return fmt.Errorf("context %q does not exist", name)
	}
	return os.RemoveAll(dir)
}

// IsOpmManaged returns true if opencodeDir is a symlink whose target is inside
// this store's profiles directory. Used by commands to gate access (per D-08/D-09).
func (s *Store) IsOpmManaged() (bool, error) {
	st, err := symlink.Inspect(s.opencodeDir)
	if err != nil {
		return false, err
	}
	if !st.IsSymlink {
		return false, nil
	}
	return strings.HasPrefix(st.Target, s.profilesDir()), nil
}

// RenameProfile renames a profile directory from oldName to newName.
// Validates newName with the allowlist before touching the filesystem.
// The caller is responsible for updating the active symlink and current file
// if oldName was the active profile (per D-04).
func (s *Store) RenameProfile(oldName, newName string) error {
	if err := ValidateName(newName); err != nil {
		return err
	}
	oldDir := s.ProfileDir(oldName)
	newDir := s.ProfileDir(newName)

	if _, err := os.Lstat(oldDir); os.IsNotExist(err) {
		return fmt.Errorf("context %q does not exist", oldName)
	}
	if _, err := os.Lstat(newDir); err == nil {
		return fmt.Errorf("context %q already exists", newName)
	}

	if err := os.Rename(oldDir, newDir); err != nil {
		return fmt.Errorf("rename profile %q to %q: %w", oldName, newName, err)
	}
	return nil
}
