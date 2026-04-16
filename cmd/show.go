package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:               "show",
	Short:             "Print the name of the currently active profile",
	Args:              cobra.NoArgs,
	PersistentPreRunE: managedGuard,
	SilenceUsage:      true,
	RunE:              runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	s := newStore()
	if name, err := s.ActiveProfile(); err == nil && name != "" {
		fmt.Fprintln(cmd.OutOrStdout(), name)
		return nil
	}
	if name, err := s.GetCurrent(); err == nil && name != "" {
		fmt.Fprintln(cmd.OutOrStdout(), name)
		return nil
	}
	return fmt.Errorf("no active profile — run 'opm init' first")
}
