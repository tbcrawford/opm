package cmd

import (
	"fmt"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
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
	RunE:              runContextUse,
}

func init() {
	contextCmd.AddCommand(useCmd)
}

func runContextUse(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := newStore()

	profileDir, err := s.GetProfile(name)
	if err != nil {
		return err
	}

	opencodeDir := paths.OpencodeConfigDir()
	if err := symlink.SetAtomic(profileDir, opencodeDir); err != nil {
		return fmt.Errorf("switch profile: %w", err)
	}

	if err := s.SetCurrent(name); err != nil {
		return fmt.Errorf("update current: %w", err)
	}

	output.Success(cmd.OutOrStdout(), "Switched to "+output.ProfileName(name),
		output.ShortenHome(opencodeDir)+" → profiles/"+name)
	return nil
}
