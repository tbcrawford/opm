package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
)

var renameCmd = &cobra.Command{
	Use:               "rename <old> <new>",
	Short:             "Rename a profile",
	Args:              cobra.ExactArgs(2),
	PreRunE:           managedGuard,
	ValidArgsFunction: singleArgProfileCompletion,
	SilenceUsage:      true,
	RunE:              runRename,
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	oldName, newName := args[0], args[1]
	s := newStore()
	result, err := s.RenameProfileAndRetarget(oldName, newName)
	if err != nil {
		return err
	}
	if result.CurrentCacheErr != nil {
		warnCurrentCacheUpdate(cmd, result.CurrentCacheErr)
	}

	w := cmd.OutOrStdout()
	msg := "Renamed " + output.ProfileName(oldName) + " → " + output.ProfileName(newName)

	if result.WasActive {
		output.Success(w, msg, "Active profile updated")
	} else {
		output.Success(w, msg)
	}
	return nil
}
