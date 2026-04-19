package cmd

import (
	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
)

var createCmd = &cobra.Command{
	Use:          "create <name>",
	Short:        "Create a new profile",
	Args:         cobra.ExactArgs(1),
	PreRunE:      managedGuard,
	SilenceUsage: true,
	RunE:         runCreate,
}

func init() {
	createCmd.Flags().String("from", "", "Copy an existing profile as the starting point")
	markRootHelpGroup(createCmd, helpGroupProfiles)
	markRootHelpOrder(createCmd, 10)
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	createFrom, _ := cmd.Flags().GetString("from")
	s := newStore()

	if createFrom != "" {
		if err := s.CopyProfile(createFrom, name); err != nil {
			return err
		}
		output.Success(cmd.OutOrStdout(),
			"Created profile "+output.ProfileName(name)+" from "+output.ProfileName(createFrom),
			output.ShortenHome(s.ProfileDir(name))+"/",
		)
		return nil
	}

	if err := s.CreateProfile(name); err != nil {
		return err
	}
	output.Success(cmd.OutOrStdout(), "Created profile "+output.ProfileName(name),
		output.ShortenHome(s.ProfileDir(name))+"/")
	return nil
}
