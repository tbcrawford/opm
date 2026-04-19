package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
)

var listCmd = &cobra.Command{
	Use:          "list",
	Aliases:      []string{"ls"},
	Short:        "List all profiles",
	Args:         cobra.NoArgs,
	PreRunE:      managedGuard,
	SilenceUsage: true,
	RunE:         runList,
}

func init() {
	listCmd.Flags().BoolP("long", "l", false, "Show profile paths")
	markRootHelpGroup(listCmd, helpGroupProfiles)
	markRootHelpOrder(listCmd, 40)
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	long, _ := cmd.Flags().GetBool("long")
	s := newStore()
	profiles, err := s.ListProfiles()
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No profiles found. Run 'opm create <name>' to create one.")
		return nil
	}

	if long {
		output.ProfileTableLong(cmd.OutOrStdout(), profiles)
	} else {
		output.ProfileTable(cmd.OutOrStdout(), profiles)
	}
	return nil
}
