package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
	"github.com/tbcrawford/opm/internal/store"
	"github.com/tbcrawford/opm/internal/symlink"
)

var removeForce bool

var removeCmd = &cobra.Command{
	Use:               "remove <name> [name...]",
	Aliases:           []string{"rm"},
	Short:             "Remove one or more profiles",
	Args:              cobra.MinimumNArgs(1),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runRemove,
}

func init() {
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Force removal of the active profile (auto-switches first)")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	s := newStore()
	w := cmd.OutOrStdout()

	// Validate all names before any filesystem access.
	for _, name := range args {
		if err := store.ValidateName(name); err != nil {
			return err
		}
	}

	seen := make(map[string]bool, len(args))
	for _, name := range args {
		if seen[name] {
			return fmt.Errorf("context %q specified more than once", name)
		}
		seen[name] = true
	}

	// Validate all names exist before removing anything.
	for _, name := range args {
		if _, err := s.GetProfile(name); err != nil {
			return err
		}
	}

	active, err := s.ActiveProfile()
	if err != nil {
		return fmt.Errorf("determine active profile: %w", err)
	}

	// If the active profile is in the list, handle it first.
	for _, name := range args {
		if name != active {
			continue
		}
		if !removeForce {
			return fmt.Errorf("cannot remove the active profile\n\n  Switch first:     opm use <name>\n  Or force remove:  opm remove --force %s", name)
		}
		// Auto-switch away, excluding ALL names being removed.
		switchTarget, err := selectAutoSwitchTarget(s, args)
		if err != nil {
			return err
		}
		targetDir := s.ProfileDir(switchTarget)
		if err := symlink.SetAtomic(targetDir, s.OpencodeDir()); err != nil {
			return fmt.Errorf("switch to %q: %w", switchTarget, err)
		}
		if err := s.SetCurrent(switchTarget); err != nil {
			warnCurrentCacheUpdate(cmd, err)
		}
		output.Success(w, "Switched to "+output.ProfileName(switchTarget), "Auto-switched before removal")
		break
	}

	for _, name := range args {
		if err := s.DeleteProfile(name, true); err != nil {
			return err
		}
		output.Success(w, "Removed profile "+output.ProfileName(name))
	}
	return nil
}

func selectAutoSwitchTarget(s *store.Store, deletingNames []string) (string, error) {
	removing := make(map[string]bool, len(deletingNames))
	for _, n := range deletingNames {
		removing[n] = true
	}

	profiles, err := s.ListProfiles()
	if err != nil {
		return "", err
	}

	var candidates []string
	for _, p := range profiles {
		if !removing[p.Name] {
			candidates = append(candidates, p.Name)
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("cannot remove the only profile\n\n  Create another profile first:\n    opm create <name>")
	}

	for _, c := range candidates {
		if c == "default" {
			return "default", nil
		}
	}

	// candidates is already sorted (ListProfiles returns alphabetical order).
	return candidates[0], nil
}
