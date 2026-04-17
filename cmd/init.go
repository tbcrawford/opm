package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
	"github.com/tbcrawford/opm/internal/paths"
	"github.com/tbcrawford/opm/internal/symlink"
)

var initProfileName string

var initCmd = &cobra.Command{
	Use:          "init",
	Short:        "Initialize opm and migrate existing OpenCode config",
	Long:         "Migrates ~/.config/opencode to an opm-managed profile and installs the managing symlink.\n\nThe initial profile is named 'default' unless overridden with --as.",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         runInit,
}

func init() {
	initCmd.Flags().StringVar(&initProfileName, "as", "default", "name to give the initial profile")
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	profileName := initProfileName
	opencodeDir := paths.OpencodeConfigDir()
	profileDir := paths.ProfileDir(profileName)
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

	if _, statErr := os.Lstat(profileDir); statErr == nil {
		tmpSym := opencodeDir + ".opm-new"
		if _, tmpErr := os.Lstat(tmpSym); tmpErr == nil {
			if err := os.Rename(tmpSym, opencodeDir); err != nil {
				return fmt.Errorf("resume: atomic rename symlink: %w", err)
			}
			s := newStore()
			if err := s.SetCurrent(profileName); err != nil {
				return fmt.Errorf("set current: %w", err)
			}
			output.Success(cmd.OutOrStdout(), "Initialized opm", "Migrated ~/.config/opencode → profiles/"+profileName)
			return nil
		}
		// profile dir exists — only truly initialized if the symlink is also in place.
		s := newStore()
		managed, mErr := s.IsOpmManaged()
		if mErr == nil && managed {
			return fmt.Errorf("Already initialized (active: %s)", profileName)
		}
		return fmt.Errorf(
			"partial initialization detected: profiles/%s exists but ~/.config/opencode is not managed by opm\n\n"+
				"  To recover:\n"+
				"    rm -rf ~/.config/opm/profiles/%s\n"+
				"    opm init",
			profileName, profileName,
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
		if err := symlink.SetAtomic(profileDir, tmpSym); err != nil {
			return fmt.Errorf("step 1 — create temp symlink: %w", err)
		}
		if err := os.Rename(opencodeDir, profileDir); err != nil {
			_ = os.Remove(tmpSym)
			return fmt.Errorf("step 2 — move opencode dir to profile: %w", err)
		}
		if err := os.Rename(tmpSym, opencodeDir); err != nil {
			return fmt.Errorf("step 3 — install symlink: %w", err)
		}
		if err := s.SetCurrent(profileName); err != nil {
			return fmt.Errorf("set current: %w", err)
		}
		output.Success(w, "Initialized opm", "Migrated ~/.config/opencode → profiles/"+profileName)
	} else {
		if err := os.MkdirAll(profileDir, 0o755); err != nil {
			return fmt.Errorf("create %s profile: %w", profileName, err)
		}
		if err := symlink.SetAtomic(profileDir, opencodeDir); err != nil {
			return fmt.Errorf("install symlink: %w", err)
		}
		if err := s.SetCurrent(profileName); err != nil {
			return fmt.Errorf("set current: %w", err)
		}
		output.Success(w, "Initialized opm",
			"Created "+profileName+" profile at "+output.ShortenHome(profileDir)+"/")
	}

	return nil
}
