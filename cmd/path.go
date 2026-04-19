package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pathCmd = &cobra.Command{
	Use:               "path <name>",
	Short:             "Print the absolute path to a profile directory",
	Args:              cobra.ExactArgs(1),
	PreRunE:           managedGuard,
	ValidArgsFunction: singleArgProfileCompletion,
	SilenceUsage:      true,
	RunE:              runPath,
}

func init() {
	rootCmd.AddCommand(pathCmd)
}

func runPath(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := newStore()
	profilePath, err := s.GetProfile(name)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), profilePath)
	return nil
}
