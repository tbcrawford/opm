package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tbcrawford/opm/internal/output"
	"github.com/tbcrawford/opm/internal/paths"
	"github.com/tbcrawford/opm/internal/symlink"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:          "init",
	Short:        "Initialize opm and migrate existing OpenCode config",
	Long:         "Migrates ~/.config/opencode to a 'default' opm profile and installs the managing symlink.",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	opencodeDir := paths.OpencodeConfigDir()
	defaultProfileDir := paths.ProfileDir("default")
	profilesDir := paths.ProfilesDir()

	st, err := symlink.Inspect(opencodeDir)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", opencodeDir, err)
	}

	if st.IsSymlink && strings.HasPrefix(st.Target, profilesDir) {
		activeName := filepath.Base(st.Target)
		return fmt.Errorf("Already initialized (active: %s)", activeName)
	}

	if st.IsSymlink && !strings.HasPrefix(st.Target, profilesDir) {
		return fmt.Errorf("~/.config/opencode is an unrecognized symlink\n\n  Back it up and remove it, then run 'opm init' again.")
	}

	if _, statErr := os.Lstat(defaultProfileDir); statErr == nil {
		tmpSym := opencodeDir + ".opm-new"
		if _, tmpErr := os.Lstat(tmpSym); tmpErr == nil {
			if err := os.Rename(tmpSym, opencodeDir); err != nil {
				return fmt.Errorf("resume: atomic rename symlink: %w", err)
			}
			s := newStore()
			if err := s.SetCurrent("default"); err != nil {
				return fmt.Errorf("set current: %w", err)
			}
			output.Success(cmd.OutOrStdout(), "Initialized opm", "Migrated ~/.config/opencode → profiles/default")
			return nil
		}
		// profiles/default exists — only truly initialized if the symlink is also in place.
		s := newStore()
		managed, mErr := s.IsOpmManaged()
		if mErr == nil && managed {
			return fmt.Errorf("Already initialized (active: default)")
		}
		return fmt.Errorf(
			"partial initialization detected: profiles/default exists but ~/.config/opencode is not managed by opm\n\n" +
				"  To recover:\n" +
				"    rm -rf ~/.config/opm/profiles/default\n" +
				"    opm init",
		)
	}

	s := newStore()
	if err := s.Init(); err != nil {
		return fmt.Errorf("create opm dirs: %w", err)
	}

	w := cmd.OutOrStdout()

	if st.IsDir {
		tmpSym := opencodeDir + ".opm-new"
		_ = os.Remove(tmpSym)
		if err := symlink.SetAtomic(defaultProfileDir, tmpSym); err != nil {
			return fmt.Errorf("step 1 — create temp symlink: %w", err)
		}
		if err := os.Rename(opencodeDir, defaultProfileDir); err != nil {
			_ = os.Remove(tmpSym)
			return fmt.Errorf("step 2 — move opencode dir to profile: %w", err)
		}
		if err := os.Rename(tmpSym, opencodeDir); err != nil {
			return fmt.Errorf("step 3 — install symlink: %w", err)
		}
		if err := s.SetCurrent("default"); err != nil {
			return fmt.Errorf("set current: %w", err)
		}
		output.Success(w, "Initialized opm", "Migrated ~/.config/opencode → profiles/default")
	} else {
		if err := os.MkdirAll(defaultProfileDir, 0o755); err != nil {
			return fmt.Errorf("create default profile: %w", err)
		}
		if err := symlink.SetAtomic(defaultProfileDir, opencodeDir); err != nil {
			return fmt.Errorf("install symlink: %w", err)
		}
		if err := s.SetCurrent("default"); err != nil {
			return fmt.Errorf("set current: %w", err)
		}
		output.Success(w, "Initialized opm",
			"Created default profile at "+output.ShortenHome(defaultProfileDir)+"/")
	}

	return nil
}
