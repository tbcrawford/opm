package cmd

import (
	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/spf13/cobra"
)

var createFrom string

var createCmd = &cobra.Command{
	Use:               "create <name>",
	Short:             "Create a new profile",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	SilenceUsage:      true,
	RunE:              runCreate,
}

func init() {
	createCmd.Flags().StringVar(&createFrom, "from", "", "Copy an existing profile as the starting point")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := newStore()

	if createFrom != "" {
		if err := s.CopyProfile(createFrom, name); err != nil {
			return err
		}
		dstDir := paths.ProfileDir(name)
		output.Success(cmd.OutOrStdout(),
			"Created profile "+output.ProfileName(name)+" from "+output.ProfileName(createFrom),
			output.ShortenHome(dstDir)+"/",
		)
		return nil
	}

	if err := s.CreateProfile(name); err != nil {
		return err
	}
	profileDir := paths.ProfileDir(name)
	output.Success(cmd.OutOrStdout(), "Created profile "+output.ProfileName(name),
		output.ShortenHome(profileDir)+"/")
	return nil
}
