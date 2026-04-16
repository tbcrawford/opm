package cmd

import (
	"fmt"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/opm-cli/opm/internal/store"
	"github.com/opm-cli/opm/internal/symlink"
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:               "use <name>",
	Short:             "Switch to a profile",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runUse,
}

func init() {
	rootCmd.AddCommand(useCmd)
}

func runUse(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := store.ValidateName(name); err != nil {
		return err
	}
	s := newStore()

	profileDir, err := s.GetProfile(name)
	if err != nil {
		return err
	}

	// Capture the current active profile before switching.
	fromName, _ := s.ActiveProfile()

	if fromName == name {
		output.Success(cmd.OutOrStdout(), "Already on profile "+output.ProfileName(name))
		return nil
	}

	opencodeDir := paths.OpencodeConfigDir()
	if err := symlink.SetAtomic(profileDir, opencodeDir); err != nil {
		return fmt.Errorf("switch profile: %w", err)
	}

	if err := s.SetCurrent(name); err != nil {
		return fmt.Errorf("update current: %w", err)
	}

	var msg string
	if fromName != "" && fromName != name {
		msg = output.ProfileName(fromName) + " → " + output.ProfileName(name)
	} else {
		msg = "Switched to " + output.ProfileName(name)
	}
	output.Success(cmd.OutOrStdout(), msg,
		output.ShortenHome(opencodeDir)+" → profiles/"+name)
	return nil
}
