package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/output"
	"github.com/tbcrawford/opm/internal/symlink"
)

var doctorCmd = &cobra.Command{
	Use:          "doctor",
	Short:        "Check opm installation health",
	Long:         "Runs a series of checks on the opm installation and reports any issues.",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()

	s := newStore()
	opencodeDir := s.OpencodeDir()

	warnings := 0
	failures := 0

	// ── Symlink ──────────────────────────────────────────────────────────────
	output.DoctorSection(out, "Symlink")

	managed, err := s.IsOpmManaged()
	if err != nil {
		output.DoctorRow(out, output.StatusFail, fmt.Sprintf("~/.config/opencode: %v", err))
		failures++
		_, _ = fmt.Fprintln(out)
		output.DoctorSummary(out, warnings, failures)
		return errSilent
	}
	if !managed {
		output.DoctorRow(out, output.StatusFail, "~/.config/opencode is not an opm-managed symlink — run 'opm init'")
		failures++
		_, _ = fmt.Fprintln(out)
		output.DoctorSummary(out, warnings, failures)
		return errSilent
	}

	st, err := symlink.Inspect(opencodeDir)
	if err != nil {
		output.DoctorRow(out, output.StatusFail, fmt.Sprintf("inspect ~/.config/opencode: %v", err))
		failures++
	} else if st.Dangling {
		output.DoctorRow(out, output.StatusFail,
			fmt.Sprintf("~/.config/opencode → %q (profile directory missing)", st.Target))
		failures++
	} else {
		activeName, _ := s.ActiveProfile()
		output.DoctorRow(out, output.StatusOK,
			fmt.Sprintf("~/.config/opencode → %s", output.ProfileName(activeName)))
	}
	_, _ = fmt.Fprintln(out)

	// ── Profiles ─────────────────────────────────────────────────────────────
	output.DoctorSection(out, "Profiles")

	profiles, err := s.ListProfiles()
	if err != nil {
		output.DoctorRow(out, output.StatusFail, fmt.Sprintf("list profiles: %v", err))
		failures++
	} else {
		for _, p := range profiles {
			if p.Dangling {
				output.DoctorRow(out, output.StatusFail,
					fmt.Sprintf("%s — directory missing", output.ProfileName(p.Name)))
				failures++
				continue
			}
			fi, statErr := os.Lstat(p.Path)
			if statErr != nil || !fi.IsDir() {
				output.DoctorRow(out, output.StatusFail,
					fmt.Sprintf("%s — not a valid directory (%s)", output.ProfileName(p.Name), p.Path))
				failures++
			} else {
				output.DoctorRow(out, output.StatusOK, output.ProfileName(p.Name))
			}
		}
	}

	// ── Consistency (only shown when mismatch exists) ─────────────────────────
	current, curErr := s.GetCurrent()
	active, actErr := s.ActiveProfile()
	mismatch := curErr == nil && actErr == nil && current != "" && active != "" && current != active
	if mismatch {
		_, _ = fmt.Fprintln(out)
		output.DoctorSection(out, "Consistency")
		output.DoctorRow(out, output.StatusWarn,
			fmt.Sprintf("current file says %q but active symlink points to %q", current, active))
		warnings++
	}

	_, _ = fmt.Fprintln(out)
	output.DoctorSummary(out, warnings, failures)

	if failures > 0 {
		return errSilent
	}
	return nil
}
