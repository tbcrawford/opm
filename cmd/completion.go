package cmd

import (
	"github.com/spf13/cobra"
)

// profileNameCompletion is a ValidArgsFunction for commands that take profile name arguments.
// Returns existing (non-dangling) profile names for shell tab completion.
// Works with zsh, bash, and fish via cobra's completion infrastructure.
func profileNameCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeProfileNames(args, -1, true)
}

// singleArgProfileCompletion completes only the first positional argument.
// Use for commands that take exactly one profile name (use, inspect, path).
func singleArgProfileCompletion(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return completeProfileNames(args, 1, false)
}

// completeProfileNames returns profile names for tab completion.
// maxArgs limits how many positional args get completion (-1 = unlimited).
// When excludeSelected is true, already-selected args are filtered out.
func completeProfileNames(args []string, maxArgs int, excludeSelected bool) ([]string, cobra.ShellCompDirective) {
	if maxArgs >= 0 && len(args) >= maxArgs {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var selected []string
	if excludeSelected {
		selected = args
	}

	names, managed, err := newStore().CompletableProfiles(selected)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	if !managed {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
