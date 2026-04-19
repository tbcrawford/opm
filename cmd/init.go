package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
	"github.com/tbcrawford/opm/internal/store"
)

var initCmd = &cobra.Command{
	Use:          "init",
	Short:        "Initialize opm and migrate existing OpenCode config",
	Long:         "Migrates ~/.config/opencode to an opm-managed profile and installs the managing symlink.\n\nThe initial profile is named 'default' unless overridden with --as.",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         runInit,
}

func init() {
	initCmd.Flags().String("as", "default", "name to give the initial profile")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	profileName, _ := cmd.Flags().GetString("as")
	s := newStore()

	result, err := s.Initialize(profileName)
	if err != nil {
		var nameErr store.InitNameError
		if errors.As(err, &nameErr) {
			return fmt.Errorf("--as: %w", nameErr.Unwrap())
		}
		return err
	}

	if result.CurrentCacheErr != nil {
		warnCurrentCacheUpdate(cmd, result.CurrentCacheErr)
	}

	if result.Migrated {
		output.Success(cmd.OutOrStdout(), "Initialized opm", "Migrated ~/.config/opencode → profiles/"+profileName)
		return nil
	}

	output.Success(cmd.OutOrStdout(), "Initialized opm",
		"Created "+profileName+" profile at "+output.ShortenHome(result.ProfileDir)+"/",
	)
	return nil
}
