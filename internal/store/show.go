package store

import (
	"errors"
	"fmt"

	"github.com/tbcrawford/opm/internal/symlink"
)

var (
	ErrShowNotManaged      = errors.New("show: opencode is not managed by opm")
	ErrShowBrokenManaged   = errors.New("show: managed opencode state is broken")
	ErrShowNoActiveProfile = errors.New("show: no active profile")
)

// ShowActiveProfile returns the active profile name or the same user-facing
// errors that `opm show` should surface.
func (s *Store) ShowActiveProfile() (string, error) {
	state, err := s.managedLinkState()
	if err != nil {
		return "", fmt.Errorf("cannot determine opm state: %w", err)
	}
	if !state.Managed {
		st, err := symlink.Inspect(s.OpencodeDir())
		if err != nil {
			return "", fmt.Errorf("inspect active symlink: %w", err)
		}
		if st.Exists {
			return "", ErrShowNotManaged
		}

		current, err := s.GetCurrent()
		if err != nil {
			return "", fmt.Errorf("read current cache: %w", err)
		}
		if current != "" {
			return "", ErrShowBrokenManaged
		}
		return "", ErrShowNoActiveProfile
	}
	if state.Dangling {
		return "", ErrShowBrokenManaged
	}

	if state.ProfileName != "" {
		return state.ProfileName, nil
	}
	return "", ErrShowNoActiveProfile
}
