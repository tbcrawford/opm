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
	current, err := s.GetCurrent()
	if err != nil {
		return err
	}
	if current == "" {
		current, err = s.ActiveProfile()
		if err != nil || current == "" {
			return fmt.Errorf("no active profile — run 'opm init' first")
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), current)
	return nil
}
