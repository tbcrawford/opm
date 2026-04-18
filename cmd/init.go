package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
	"github.com/tbcrawford/opm/internal/store"
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

func alreadyInitializedError(s *store.Store) error {
	st, err := symlink.Inspect(s.OpencodeDir())
	if err != nil {
		return fmt.Errorf("inspect %s: %w", s.OpencodeDir(), err)
	}
	activeName := filepath.Base(st.Target)
	return fmt.Errorf("already initialized (active: %s)", activeName)
}

func runInit(cmd *cobra.Command, args []string) error {
	profileName := initProfileName
	if err := store.ValidateName(profileName); err != nil {
		return fmt.Errorf("--as: %w", err)
	}

	s := newStore()
	opencodeDir := s.OpencodeDir()
	profileDir := s.ProfileDir(profileName)

	st, err := symlink.Inspect(opencodeDir)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", opencodeDir, err)
	}

	if st.IsSymlink {
		managed, err := s.IsOpmManaged()
		if err != nil {
			return fmt.Errorf("cannot determine opm state: %w", err)
		}
		if managed {
			return alreadyInitializedError(s)
		}
		return fmt.Errorf("~/.config/opencode is an unrecognized symlink\n\n  Back it up and remove it, then run 'opm init' again")
	}

	if st.Exists && !st.IsDir && !st.IsSymlink {
		return fmt.Errorf("~/.config/opencode is not a directory or symlink\n\n  Back it up and remove it, then run 'opm init' again")
	}

	tmpSym := opencodeDir + ".opm-new"
	profileIsDir := false
	if _, tmpErr := os.Lstat(tmpSym); tmpErr == nil {
		tmpSt, err := symlink.Inspect(tmpSym)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", tmpSym, err)
		}
		profileExists := false
		if profileInfo, err := os.Lstat(profileDir); err == nil {
			profileExists = true
			profileIsDir = profileInfo.IsDir()
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("inspect %s: %w", profileDir, err)
		}
		if !profileExists || !tmpSt.IsSymlink || tmpSt.Target != profileDir {
			return fmt.Errorf(
				"stale interrupted init state detected\n\n  To recover:\n    remove ~/.config/opencode.opm-new\n    opm init",
			)
		}
	}

	if _, statErr := os.Lstat(profileDir); statErr == nil {
		if _, tmpErr := os.Lstat(tmpSym); tmpErr == nil {
			if !profileIsDir {
				return fmt.Errorf(
					"partial initialization detected: profiles/%s exists but is not a directory\n\n"+
						"  To recover:\n"+
						"    rm -rf ~/.config/opm/profiles/%s\n"+
						"    opm init",
					profileName, profileName,
				)
			}
			if st.Exists {
				return fmt.Errorf(
					"partial initialization detected: profiles/%s exists but ~/.config/opencode still exists\n\n"+
						"  To recover:\n"+
						"    inspect and back up ~/.config/opencode\n"+
						"    remove ~/.config/opencode\n"+
						"    remove ~/.config/opencode.opm-new\n"+
						"    opm init",
					profileName,
				)
			}
			if err := os.Rename(tmpSym, opencodeDir); err != nil {
				return fmt.Errorf("resume: atomic rename symlink: %w", err)
			}
			if err := s.SetCurrent(profileName); err != nil {
				warnCurrentCacheUpdate(cmd, err)
			}
			output.Success(cmd.OutOrStdout(), "Initialized opm", "Migrated ~/.config/opencode → profiles/"+profileName)
			return nil
		}
		// profile dir exists — only truly initialized if the symlink is also in place.
		managed, mErr := s.IsOpmManaged()
		if mErr == nil && managed {
			return alreadyInitializedError(s)
		}
		return fmt.Errorf(
			"partial initialization detected: profiles/%s exists but ~/.config/opencode is not managed by opm\n\n"+
				"  To recover:\n"+
				"    rm -rf ~/.config/opm/profiles/%s\n"+
				"    opm init",
			profileName, profileName,
		)
	}

	if err := s.Init(); err != nil {
		return fmt.Errorf("create opm dirs: %w", err)
	}

	w := cmd.OutOrStdout()

	if st.IsDir {
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
			warnCurrentCacheUpdate(cmd, err)
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
			warnCurrentCacheUpdate(cmd, err)
		}
		output.Success(w, "Initialized opm",
			"Created "+profileName+" profile at "+output.ShortenHome(profileDir)+"/")
	}

	return nil
}
