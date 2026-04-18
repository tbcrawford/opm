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
	name, err := s.ActiveProfile()
	if err == nil && name != "" {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), name)
		return nil
	}
	// Symlink is absent or broken — fall back to the current file but warn the user.
	if cached, cerr := s.GetCurrent(); cerr == nil && cached != "" {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: symlink is broken or absent; reporting cached profile name\n")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), cached)
		return nil
	}
	return fmt.Errorf("no active profile — run 'opm init' first")
}
