package cmd

import (
	"fmt"

	"github.com/tbcrawford/opm/internal/output"
	"github.com/spf13/cobra"
)

var listLong bool

var listCmd = &cobra.Command{
	Use:               "list",
	Aliases:           []string{"ls"},
	Short:             "List all profiles",
	Args:              cobra.NoArgs,
	PersistentPreRunE: managedGuard,
	SilenceUsage:      true,
	RunE:              runList,
}

func init() {
	listCmd.Flags().BoolVarP(&listLong, "long", "l", false, "Show profile paths")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	s := newStore()
	profiles, err := s.ListProfiles()
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No profiles found. Run 'opm create <name>' to create one.")
		return nil
	}

	if listLong {
		output.ProfileTableLong(cmd.OutOrStdout(), profiles)
	} else {
		output.ProfileTable(cmd.OutOrStdout(), profiles)
	}
	return nil
}
