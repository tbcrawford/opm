package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/store"
)

var showCmd = &cobra.Command{
	Use:          "show",
	Short:        "Print the name of the currently active profile",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	name, err := newStore().ShowActiveProfile()
	if err != nil {
		if errors.Is(err, store.ErrShowNotManaged) {
			return fmt.Errorf("~/.config/opencode is not managed by opm\n\n  Run 'opm init' to initialize")
		}
		if errors.Is(err, store.ErrShowNoActiveProfile) {
			return fmt.Errorf("no active profile — run 'opm init' first")
		}
		return err
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), name)
	return nil
}
