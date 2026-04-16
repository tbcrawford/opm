package cmd

import "github.com/spf13/cobra"

// profileNameCompletion is a ValidArgsFunction for commands that take a profile name argument.
// Returns existing (non-dangling) profile names for shell tab completion.
// Works with zsh, bash, and fish via cobra's completion infrastructure.
func profileNameCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete the first positional argument.
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	s := newStore()
	managed, err := s.IsOpmManaged()
	if err != nil || !managed {
		// Not initialized — no names to offer; fail silently (not a UX error).
		return nil, cobra.ShellCompDirectiveError
	}

	profiles, err := s.ListProfiles()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var names []string
	for _, p := range profiles {
		if !p.Dangling {
			names = append(names, p.Name)
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
