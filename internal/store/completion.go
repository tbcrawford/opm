package store

// CompletableProfiles returns non-dangling profile names, excluding any names
// already selected by the caller. The boolean reports whether opm is currently
// managing opencode, which completion requires.
func (s *Store) CompletableProfiles(selected []string) ([]string, bool, error) {
	managed, err := s.IsOpmManaged()
	if err != nil {
		return nil, false, err
	}
	if !managed {
		return nil, false, nil
	}

	selectedSet := make(map[string]bool, len(selected))
	for _, name := range selected {
		selectedSet[name] = true
	}

	profiles, err := s.ListProfiles()
	if err != nil {
		return nil, true, err
	}

	names := make([]string, 0, len(profiles))
	for _, p := range profiles {
		if p.Dangling || selectedSet[p.Name] {
			continue
		}
		names = append(names, p.Name)
	}
	return names, true, nil
}
