package cmd

import (
	"fmt"
	"sort"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/opm-cli/opm/internal/store"
	"github.com/opm-cli/opm/internal/symlink"
	"github.com/spf13/cobra"
)

var rmForce bool

var rmCmd = &cobra.Command{
	Use:               "rm <name>",
	Short:             "Remove a profile",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runContextRm,
}

func init() {
	rmCmd.Flags().BoolVarP(&rmForce, "force", "f", false, "Force removal of the active profile (auto-switches first)")
	contextCmd.AddCommand(rmCmd)
}

func runContextRm(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := newStore()

	if _, err := s.GetProfile(name); err != nil {
		return err
	}

	active, err := s.ActiveProfile()
	if err != nil {
		return fmt.Errorf("determine active profile: %w", err)
	}

	isActive := active == name
	w := cmd.OutOrStdout()

	if isActive && !rmForce {
		return fmt.Errorf("cannot remove the active profile\n\n  Switch first:     opm context use <name>\n  Or force remove:  opm context rm --force %s", name)
	}

	if isActive && rmForce {
		switchTarget, err := selectAutoSwitchTarget(s, name)
		if err != nil {
			return err
		}

		targetDir := s.ProfileDir(switchTarget)
		if err := symlink.SetAtomic(targetDir, paths.OpencodeConfigDir()); err != nil {
			return fmt.Errorf("switch to %q: %w", switchTarget, err)
		}
		if err := s.SetCurrent(switchTarget); err != nil {
			return fmt.Errorf("update current: %w", err)
		}
		output.Success(w, "Switched to "+output.ProfileName(switchTarget), "Auto-switched before removal")
	}

	if err := s.DeleteProfile(name, true); err != nil {
		return err
	}
	output.Success(w, "Removed profile "+output.ProfileName(name))
	return nil
}

func selectAutoSwitchTarget(s *store.Store, deletingName string) (string, error) {
	profiles, err := s.ListProfiles()
	if err != nil {
		return "", err
	}

	var candidates []string
	for _, p := range profiles {
		if p.Name != deletingName {
			candidates = append(candidates, p.Name)
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("cannot remove the only profile\n\n  Create another profile first:\n    opm context create <name>")
	}

	for _, c := range candidates {
		if c == "default" {
			return "default", nil
		}
	}

	sort.Strings(candidates)
	return candidates[0], nil
}
