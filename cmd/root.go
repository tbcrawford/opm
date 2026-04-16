package cmd

import (
	"fmt"
	"os"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/opm-cli/opm/internal/store"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "opm",
	Short:         "OpenCode profile manager",
	Long:          "opm manages multiple OpenCode configurations by switching symlinked profile directories.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute is the binary entry point called by main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		output.Error(os.Stderr, err.Error())
		os.Exit(1)
	}
}

// SetVersionInfo sets the version string displayed by `opm --version`.
func SetVersionInfo(version, commit string) {
	rootCmd.Version = version + " (" + commit + ")"
}

// newStore returns a production Store wired to real config paths.
func newStore() *store.Store {
	return store.New(paths.OpmDir(), paths.OpencodeConfigDir())
}

// managedGuard blocks context subcommands if ~/.config/opencode is not managed by opm.
func managedGuard(cmd *cobra.Command, args []string) error {
	s := newStore()
	managed, err := s.IsOpmManaged()
	if err != nil {
		return fmt.Errorf("cannot determine opm state: %w", err)
	}
	if !managed {
		return fmt.Errorf("~/.config/opencode is not managed by opm\n\n  Run 'opm init' to initialize.")
	}
	return nil
}
