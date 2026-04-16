package paths

import (
	"os"
	"path/filepath"
)

func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	panic("opm: cannot determine home directory: $HOME is not set and os.UserHomeDir() failed")
}

// OpmDir returns ~/.config/opm — opm's own state directory.
// Uses os.UserHomeDir() + filepath.Join, NOT os.UserConfigDir()
// (UserConfigDir returns ~/Library/Application Support on macOS, not ~/.config).
func OpmDir() string {
	return filepath.Join(homeDir(), ".config", "opm")
}

// OpencodeConfigDir returns ~/.config/opencode — the managed symlink path.
func OpencodeConfigDir() string {
	return filepath.Join(homeDir(), ".config", "opencode")
}

// ProfilesDir returns ~/.config/opm/profiles.
func ProfilesDir() string {
	return filepath.Join(OpmDir(), "profiles")
}

// ProfileDir returns the absolute path to the named profile directory.
func ProfileDir(name string) string {
	return filepath.Join(ProfilesDir(), name)
}

// CurrentFile returns the path to ~/.config/opm/current.
func CurrentFile() string {
	return filepath.Join(OpmDir(), "current")
}
