package cmd

import "github.com/spf13/cobra"

// profileNameCompletion is a ValidArgsFunction for commands that take profile name arguments.
// Returns existing (non-dangling) profile names for shell tab completion.
// Works with zsh, bash, and fish via cobra's completion infrastructure.
func profileNameCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeProfileNames(args, -1)
}

// singleArgProfileCompletion completes only the first positional argument.
// Use for commands that take exactly one profile name (use, inspect, path).
func singleArgProfileCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeProfileNames(args, 1)
}

// completeProfileNames returns profile names for tab completion.
// maxArgs limits how many positional args get completion (-1 = unlimited).
func completeProfileNames(args []string, maxArgs int) ([]string, cobra.ShellCompDirective) {
	if maxArgs >= 0 && len(args) >= maxArgs {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	s := newStore()
	managed, err := s.IsOpmManaged()
	if err != nil || !managed {
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
