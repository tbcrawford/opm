package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/symlink"
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
	s := newStore()
	managed, err := s.IsOpmManaged()
	if err != nil {
		return fmt.Errorf("cannot determine opm state: %w", err)
	}
	if !managed {
		st, err := symlink.Inspect(s.OpencodeDir())
		if err != nil {
			return fmt.Errorf("inspect active symlink: %w", err)
		}
		if st.Exists {
			return fmt.Errorf("~/.config/opencode is not managed by opm\n\n  Run 'opm init' to initialize")
		}
	}

	name, err := s.ActiveProfile()
	if err == nil && name != "" {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), name)
		return nil
	}
	// If we got a real I/O error (not just "symlink absent"), surface it.
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read active profile: %w", err)
	}
	// Symlink is absent or broken — fall back to the current file but warn the user.
	if cached, cerr := s.GetCurrent(); cerr == nil && cached != "" {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: symlink is broken or absent; reporting cached profile name\n")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), cached)
		return nil
	}
	return fmt.Errorf("no active profile — run 'opm init' first")
}
