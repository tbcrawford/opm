package cmd

import (
	"github.com/tbcrawford/opm/internal/output"
	"github.com/tbcrawford/opm/internal/paths"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:               "reset",
	Short:             "Restore ~/.config/opencode to a plain directory",
	Long:              "Removes opm's symlink and copies the active profile back to ~/.config/opencode as a real directory.\nAll profiles in ~/.config/opm/profiles/ are left intact.",
	Args:              cobra.NoArgs,
	PersistentPreRunE: managedGuard,
	SilenceUsage:      true,
	RunE:              runReset,
}

func init() {
	rootCmd.AddCommand(resetCmd)
}

func runReset(cmd *cobra.Command, args []string) error {
	s := newStore()
	if err := s.Reset(); err != nil {
		return err
	}
	output.Success(cmd.OutOrStdout(),
		"Reset complete — ~/.config/opencode is now a plain directory",
		"Profiles left intact at "+output.ShortenHome(paths.ProfilesDir())+"  •  remove with: rm -rf "+output.ShortenHome(paths.OpmDir()),
	)
	return nil
}
