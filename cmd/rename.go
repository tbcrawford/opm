package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
	"github.com/tbcrawford/opm/internal/store"
	"github.com/tbcrawford/opm/internal/symlink"
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
	if err := store.ValidateName(oldName); err != nil {
		return err
	}
	if err := store.ValidateName(newName); err != nil {
		return err
	}
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
		if err := symlink.SetAtomic(newDir, s.OpencodeDir()); err != nil {
			// Rollback: move the directory back to its original name so OpenCode isn't broken.
			if rerr := os.Rename(s.ProfileDir(newName), s.ProfileDir(oldName)); rerr != nil {
				return fmt.Errorf("update active symlink: %w; rollback also failed: %v — profile directory is at %q", err, rerr, newName)
			}
			return fmt.Errorf("update active symlink: %w (rolled back directory rename)", err)
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
