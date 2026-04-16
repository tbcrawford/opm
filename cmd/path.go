package cmd

import (
	"fmt"

	"github.com/tbcrawford/opm/internal/store"
	"github.com/spf13/cobra"
)

var pathCmd = &cobra.Command{
	Use:               "path <name>",
	Short:             "Print the filesystem path to a profile directory",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runPath,
}

func init() {
	rootCmd.AddCommand(pathCmd)
}

func runPath(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := store.ValidateName(name); err != nil {
		return err
	}
	s := newStore()
	profilePath, err := s.GetProfile(name)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), profilePath)
	return nil
}
