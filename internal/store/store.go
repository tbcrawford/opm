package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tbcrawford/opm/internal/symlink"
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

// ProfilesDir returns the absolute path to the profiles directory.
func (s *Store) ProfilesDir() string { return s.profilesDir() }

// OpmDir returns the absolute path to the opm state directory (the store root).
func (s *Store) OpmDir() string { return s.root }

// OpencodeDir returns the path to the managed opencode symlink (~/.config/opencode).
func (s *Store) OpencodeDir() string { return s.opencodeDir }

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

type managedLinkState struct {
	Exists         bool
	IsSymlink      bool
	Managed        bool
	ResolvedTarget string
	ProfileName    string
	Dangling       bool
}

func (s *Store) managedLinkState() (managedLinkState, error) {
	st, err := symlink.Inspect(s.opencodeDir)
	if err != nil {
		return managedLinkState{}, err
	}

	state := managedLinkState{
		Exists:    st.Exists,
		IsSymlink: st.IsSymlink,
		Dangling:  st.Dangling,
	}
	if !st.IsSymlink {
		return state, nil
	}

	// Resolve the symlink target to an absolute, clean path.
	target := st.Target
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(s.opencodeDir), target)
	}
	target = filepath.Clean(target)
	state.ResolvedTarget = target
	state.ProfileName = filepath.Base(target)

	// Guard against degenerate or nested paths masquerading as profile names.
	if !isValidProfileBasename(state.ProfileName) {
		return state, nil
	}

	// isDirectChildOf reports whether target is a direct child of profilesDir
	// without descending further. It handles both raw and symlink-resolved paths.
	if st.Dangling {
		return s.resolveDanglingLink(state)
	}
	return s.resolveActiveLink(state)
}

// isValidProfileBasename returns false for empty, dot, dotdot, or paths
// containing a separator — all of which cannot be a valid profile directory name.
func isValidProfileBasename(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.Contains(name, string(filepath.Separator))
}

// resolveDanglingLink checks whether a dangling symlink's target is a direct
// child of the profiles directory, making it a managed (but missing) profile.
func (s *Store) resolveDanglingLink(state managedLinkState) (managedLinkState, error) {
	target := state.ResolvedTarget

	// Resolve the profiles directory through any of its own symlinks. If it
	// doesn't exist (opm never initialized), the link cannot be managed.
	resolvedProfilesDir, err := filepath.EvalSymlinks(s.profilesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return managedLinkState{}, fmt.Errorf("resolve profiles dir: %w", err)
	}

	// Fast path: raw rel check (string math, no I/O).
	rawRel, rawErr := filepath.Rel(s.profilesDir(), target)
	rawDirectChild := rawErr == nil && rawRel == state.ProfileName

	if !rawDirectChild {
		// Slow path: compare resolved parent against the resolved profiles dir.
		resolvedParent, err := filepath.EvalSymlinks(filepath.Dir(target))
		if err == nil && resolvedParent == resolvedProfilesDir {
			rawDirectChild = true
		}
	}

	if !rawDirectChild {
		return state, nil
	}

	// The target's parent resolves to the profiles dir — confirm the target
	// entry itself is not another symlink (which would be invalid).
	profileInfo, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			state.Managed = true
			return state, nil
		}
		return managedLinkState{}, fmt.Errorf("stat profile entry: %w", err)
	}
	if profileInfo.Mode()&os.ModeSymlink != 0 {
		return state, nil
	}
	state.Managed = true
	return state, nil
}

// resolveActiveLink checks whether a non-dangling symlink resolves to a
// directory that is a direct child of the profiles directory.
func (s *Store) resolveActiveLink(state managedLinkState) (managedLinkState, error) {
	target := state.ResolvedTarget

	profileInfo, err := os.Lstat(target)
	if err != nil {
		return managedLinkState{}, fmt.Errorf("stat profile entry: %w", err)
	}
	if !profileInfo.IsDir() {
		return state, nil
	}

	effectiveTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return managedLinkState{}, fmt.Errorf("resolve symlink target: %w", err)
	}
	resolvedProfilesDir, err := filepath.EvalSymlinks(s.profilesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return managedLinkState{}, fmt.Errorf("resolve profiles dir: %w", err)
	}

	resolvedRel, err := filepath.Rel(resolvedProfilesDir, effectiveTarget)
	if err != nil {
		return managedLinkState{}, fmt.Errorf("resolve managed target: %w", err)
	}

	// Must be a direct child — no nesting, no dotdot, and name must match.
	if !isValidProfileBasename(resolvedRel) || resolvedRel == "." ||
		strings.HasPrefix(resolvedRel, ".."+string(filepath.Separator)) ||
		strings.Contains(resolvedRel, string(filepath.Separator)) {
		return state, nil
	}
	if resolvedRel != state.ProfileName {
		return state, nil
	}

	state.ResolvedTarget = effectiveTarget
	state.ProfileName = resolvedRel
	state.Managed = true
	return state, nil
}

// ActiveProfile derives the active profile name from os.Readlink on the managed symlink.
// Per D-12: authoritative source is the actual symlink, not the current file.
func (s *Store) ActiveProfile() (string, error) {
	state, err := s.managedLinkState()
	if err != nil {
		return "", err
	}
	if !state.Managed || state.Dangling {
		return "", nil
	}
	return state.ProfileName, nil
}

// ListProfiles scans the profiles directory and returns all profiles.
// Active profile is determined by Readlink on the managed symlink (per D-12).
func (s *Store) ListProfiles() ([]Profile, error) {
	state, err := s.managedLinkState()
	if err != nil {
		return nil, err
	}

	active := ""
	if state.Managed && !state.Dangling {
		active = state.ProfileName
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

	if state.Managed && state.Dangling {
		profiles = append(profiles, Profile{
			Name:     state.ProfileName,
			Path:     state.ResolvedTarget,
			Active:   true,
			Dangling: true,
		})
		sort.Slice(profiles, func(i, j int) bool {
			return profiles[i].Name < profiles[j].Name
		})
	}

	return profiles, nil
}

// GetProfile returns the profile directory path if the profile exists.
func (s *Store) GetProfile(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
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

// CopyProfile creates a new profile by copying an existing one.
// Validates both src and dst names, checks src exists and dst does not, then copies the directory tree.
func (s *Store) CopyProfile(src, dst string) error {
	if err := ValidateName(src); err != nil {
		return err
	}
	if err := ValidateName(dst); err != nil {
		return err
	}
	srcDir := s.ProfileDir(src)
	dstDir := s.ProfileDir(dst)

	srcInfo, err := os.Lstat(srcDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("context %q does not exist", src)
	}
	if err != nil {
		return fmt.Errorf("stat profile %q: %w", src, err)
	}
	if srcInfo.Mode()&os.ModeSymlink != 0 || !srcInfo.IsDir() {
		return fmt.Errorf("context %q is not a directory", src)
	}
	if _, err := os.Lstat(dstDir); err == nil {
		return fmt.Errorf("context %q already exists", dst)
	}

	return copyDir(srcDir, dstDir)
}

// DeleteProfile removes a profile directory.
// If force is false, refuses to delete the active profile.
// If force is true, the caller is responsible for switching the symlink first (per D-01/D-02/D-03).
func (s *Store) DeleteProfile(name string, force bool) error {
	if err := ValidateName(name); err != nil {
		return err
	}
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
	state, err := s.managedLinkState()
	if err != nil {
		return false, err
	}
	return state.Managed, nil
}

// RenameProfile renames a profile directory from oldName to newName.
// Validates newName with the allowlist before touching the filesystem.
// The caller is responsible for updating the active symlink and current file
// if oldName was the active profile (per D-04).
func (s *Store) RenameProfile(oldName, newName string) error {
	if err := ValidateName(oldName); err != nil {
		return err
	}
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

// Reset removes opm's management of opencodeDir by:
//  1. Verifying opencodeDir is an opm-managed symlink.
//  2. Copying the active profile directory to opencodeDir as a real directory.
//  3. Removing the current file.
//
// All profile data under the store root is left intact.
func (s *Store) Reset() error {
	state, err := s.managedLinkState()
	if err != nil {
		return fmt.Errorf("inspect %s: %w", s.opencodeDir, err)
	}
	if !state.Managed {
		return fmt.Errorf("%s is not managed by opm", s.opencodeDir)
	}
	if state.Dangling {
		return fmt.Errorf("%s points to a missing managed profile", s.opencodeDir)
	}

	profileDir := state.ResolvedTarget
	tmpDir := s.opencodeDir + ".opm-reset-tmp"

	// Clean up any leftover tmp from a prior crash.
	_ = os.RemoveAll(tmpDir)

	if err := copyDir(profileDir, tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("copy profile: %w", err)
	}

	if err := os.Remove(s.opencodeDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("remove symlink: %w", err)
	}

	if err := os.Rename(tmpDir, s.opencodeDir); err != nil {
		// Best-effort rollback: restore the symlink so opencodeDir isn't left absent.
		if rerr := symlink.SetAtomic(profileDir, s.opencodeDir); rerr != nil {
			return fmt.Errorf("install directory: %w; rollback also failed: %v — opencodeDir may be absent", err, rerr)
		}
		return fmt.Errorf("install directory: %w", err)
	}

	// Best-effort: remove the current file. Non-fatal if absent.
	_ = os.Remove(s.currentFile())
	return nil
}

// copyDir recursively copies src into dst.
// Regular files are copied byte-for-byte. Symlinks are re-created.
// dst must not exist before calling.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return err
			}
			continue
		}

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		if !entry.Type().IsRegular() {
			// Skip special files (sockets, pipes, devices) — not meaningful in a config directory.
			continue
		}

		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies a single regular file from src to dst, preserving permissions.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}

	_, err = io.Copy(dstFile, srcFile)
	if cerr := dstFile.Close(); err == nil {
		err = cerr
	}
	return err
}
