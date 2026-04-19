package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
)

var removeCmd = &cobra.Command{
	Use:               "remove <name> [name...]",
	Aliases:           []string{"rm"},
	Short:             "Remove one or more profiles",
	Args:              cobra.MinimumNArgs(1),
	PreRunE:           managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runRemove,
}

func init() {
	removeCmd.Flags().BoolP("force", "f", false, "Force removal of the active profile (auto-switches first)")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	s := newStore()
	w := cmd.OutOrStdout()

	result, err := s.RemoveProfiles(args, force)
	if result.CurrentCacheErr != nil {
		warnCurrentCacheUpdate(cmd, result.CurrentCacheErr)
	}
	if err != nil {
		return err
	}
	if result.SwitchedTo != "" {
		output.Success(w, "Switched to "+output.ProfileName(result.SwitchedTo), "Auto-switched before removal")
	}
	for _, name := range result.Removed {
		output.Success(w, "Removed profile "+output.ProfileName(name))
	}
	return nil
}
