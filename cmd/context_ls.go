package cmd

import (
	"fmt"

	"github.com/opm-cli/opm/internal/output"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:               "ls",
	Short:             "List all profiles",
	Args:              cobra.NoArgs,
	PersistentPreRunE: managedGuard,
	SilenceUsage:      true,
	RunE:              runContextLs,
}

func init() {
	contextCmd.AddCommand(lsCmd)
}

func runContextLs(cmd *cobra.Command, args []string) error {
	s := newStore()
	profiles, err := s.ListProfiles()
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No profiles found. Run 'opm context create <name>' to create one.")
		return nil
	}

	output.ProfileTable(cmd.OutOrStdout(), profiles)
	return nil
}
