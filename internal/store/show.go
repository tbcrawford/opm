package store

import (
	"errors"
	"fmt"

	"github.com/tbcrawford/opm/internal/symlink"
)

var (
	ErrShowNotManaged      = errors.New("show: opencode is not managed by opm")
	ErrShowNoActiveProfile = errors.New("show: no active profile")
)

// ShowActiveProfile returns the active profile name or the same user-facing
// errors that `opm show` should surface.
func (s *Store) ShowActiveProfile() (string, error) {
	managed, err := s.IsOpmManaged()
	if err != nil {
		return "", fmt.Errorf("cannot determine opm state: %w", err)
	}
	if !managed {
		st, err := symlink.Inspect(s.OpencodeDir())
		if err != nil {
			return "", fmt.Errorf("inspect active symlink: %w", err)
		}
		if st.Exists {
			return "", ErrShowNotManaged
		}
	}

	name, err := s.ActiveProfile()
	if err == nil && name != "" {
		return name, nil
	}
	if err != nil {
		return "", fmt.Errorf("read active profile: %w", err)
	}
	return "", ErrShowNoActiveProfile
}
