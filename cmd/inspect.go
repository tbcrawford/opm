package cmd

import (
	"fmt"
	"os"

	"github.com/tbcrawford/opm/internal/output"
	"github.com/tbcrawford/opm/internal/store"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:               "inspect <name>",
	Short:             "Show detailed information about a profile",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := store.ValidateName(name); err != nil {
		return err
	}
	s := newStore()

	profilePath, err := s.GetProfile(name)
	if err != nil {
		return err
	}

	active, _ := s.ActiveProfile()
	isActive := active == name

	entries, err := os.ReadDir(profilePath)
	if err != nil {
		return fmt.Errorf("read profile contents: %w", err)
	}

	output.InspectProfile(cmd.OutOrStdout(), name, profilePath, isActive, entries)
	return nil
}
