package store

import (
	"fmt"

	"github.com/tbcrawford/opm/internal/symlink"
)

// RemoveProfilesResult captures the outcome of a successful bulk remove.
type RemoveProfilesResult struct {
	SwitchedTo      string
	Removed         []string
	CurrentCacheErr error
}

// RemoveProfiles removes one or more profiles with optional forced removal of
// the active profile by switching away first.
func (s *Store) RemoveProfiles(names []string, force bool) (RemoveProfilesResult, error) {
	for _, name := range names {
		if err := ValidateName(name); err != nil {
			return RemoveProfilesResult{}, err
		}
	}

	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if seen[name] {
			return RemoveProfilesResult{}, fmt.Errorf("profile %q specified more than once", name)
		}
		seen[name] = true
	}

	result := RemoveProfilesResult{Removed: append([]string(nil), names...)}

	for _, name := range names {
		if _, err := s.GetProfile(name); err != nil {
			return result, err
		}
	}

	active, err := s.ActiveProfile()
	if err != nil {
		return result, fmt.Errorf("determine active profile: %w", err)
	}

	for _, name := range names {
		if name != active {
			continue
		}
		if !force {
			return result, fmt.Errorf("cannot remove the active profile\n\n  Switch first:     opm use <name>\n  Or force remove:  opm remove --force %s", name)
		}

		switchTarget, err := s.selectAutoSwitchTarget(names)
		if err != nil {
			return result, err
		}
		if err := symlink.SetAtomic(s.ProfileDir(switchTarget), s.OpencodeDir()); err != nil {
			return result, fmt.Errorf("switch to %q: %w", switchTarget, err)
		}

		result.SwitchedTo = switchTarget
		result.CurrentCacheErr = s.SetCurrent(switchTarget)
		break
	}

	for _, name := range names {
		if err := s.DeleteProfile(name, true); err != nil {
			return result, err
		}
	}

	return result, nil
}

func (s *Store) selectAutoSwitchTarget(deletingNames []string) (string, error) {
	removing := make(map[string]bool, len(deletingNames))
	for _, n := range deletingNames {
		removing[n] = true
	}

	profiles, err := s.ListProfiles()
	if err != nil {
		return "", err
	}

	var candidates []string
	for _, p := range profiles {
		if !removing[p.Name] {
			candidates = append(candidates, p.Name)
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("cannot remove the only profile\n\n  Create another profile first:\n    opm create <name>")
	}

	for _, c := range candidates {
		if c == "default" {
			return "default", nil
		}
	}

	return candidates[0], nil
}
