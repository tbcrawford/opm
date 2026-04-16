package cmd

import (
	"fmt"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/opm-cli/opm/internal/symlink"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:               "rename <old> <new>",
	Short:             "Rename a profile",
	Args:              cobra.ExactArgs(2),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runRename,
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	oldName, newName := args[0], args[1]
	s := newStore()

	active, err := s.ActiveProfile()
	if err != nil {
		return fmt.Errorf("determine active profile: %w", err)
	}
	wasActive := active == oldName

	if err := s.RenameProfile(oldName, newName); err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	msg := "Renamed " + output.ProfileName(oldName) + " → " + output.ProfileName(newName)

	if wasActive {
		newDir := s.ProfileDir(newName)
		if err := symlink.SetAtomic(newDir, paths.OpencodeConfigDir()); err != nil {
			return fmt.Errorf("update active symlink: %w", err)
		}
		if err := s.SetCurrent(newName); err != nil {
			return fmt.Errorf("update current: %w", err)
		}
		output.Success(w, msg, "Active profile updated")
	} else {
		output.Success(w, msg)
	}
	return nil
}
