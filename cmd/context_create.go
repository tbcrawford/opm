package cmd

import (
	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:               "create <name>",
	Short:             "Create a new profile",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	SilenceUsage:      true,
	RunE:              runContextCreate,
}

func init() {
	contextCmd.AddCommand(createCmd)
}

func runContextCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := newStore()
	if err := s.CreateProfile(name); err != nil {
		return err
	}
	profileDir := paths.ProfileDir(name)
	output.Success(cmd.OutOrStdout(), "Created profile "+output.ProfileName(name),
		output.ShortenHome(profileDir)+"/")
	return nil
}
