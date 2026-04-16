package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/opm-cli/opm/internal/symlink"
	"github.com/spf13/cobra"
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
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	opencodeDir := paths.OpencodeConfigDir()
	s := newStore()

	warnings := 0
	failures := 0

	fmt.Fprintln(out, "opm doctor")
	fmt.Fprintln(out)

	managed, err := s.IsOpmManaged()
	if err != nil {
		output.DoctorRow(w, output.StatusFail, fmt.Sprintf("~/.config/opencode: %v", err))
		failures++
		_ = w.Flush()
		fmt.Fprintln(out)
		output.DoctorSummary(out, warnings, failures)
		os.Exit(1)
	}
	if !managed {
		output.DoctorRow(w, output.StatusFail, "~/.config/opencode is not an opm-managed symlink — run 'opm init'")
		failures++
		_ = w.Flush()
		fmt.Fprintln(out)
		output.DoctorSummary(out, warnings, failures)
		os.Exit(1)
	}

	st, err := symlink.Inspect(opencodeDir)
	if err != nil {
		output.DoctorRow(w, output.StatusFail, fmt.Sprintf("inspect ~/.config/opencode: %v", err))
		failures++
	} else if st.Dangling {
		output.DoctorRow(w, output.StatusFail,
			fmt.Sprintf("~/.config/opencode is a dangling symlink → %q (profile directory is missing)", st.Target))
		failures++
	} else {
		activeName, _ := s.ActiveProfile()
		output.DoctorRow(w, output.StatusOK,
			fmt.Sprintf("~/.config/opencode → %s", output.ProfileName(activeName)))
	}

	profiles, err := s.ListProfiles()
	if err != nil {
		output.DoctorRow(w, output.StatusFail, fmt.Sprintf("list profiles: %v", err))
		failures++
	} else {
		for _, p := range profiles {
			if p.Dangling {
				output.DoctorRow(w, output.StatusFail,
					fmt.Sprintf("Profile %s — directory missing", output.ProfileName(p.Name)))
				failures++
				continue
			}
			fi, statErr := os.Lstat(p.Path)
			if statErr != nil || !fi.IsDir() {
				output.DoctorRow(w, output.StatusFail,
					fmt.Sprintf("Profile %s — not a valid directory: %s", output.ProfileName(p.Name), p.Path))
				failures++
			} else {
				output.DoctorRow(w, output.StatusOK,
					fmt.Sprintf("Profile %s — ok", output.ProfileName(p.Name)))
			}
		}
	}

	current, curErr := s.GetCurrent()
	active, actErr := s.ActiveProfile()
	if curErr == nil && actErr == nil && current != "" && active != "" && current != active {
		output.DoctorRow(w, output.StatusWarn,
			fmt.Sprintf("current file says %q but active symlink points to %q", current, active))
		warnings++
	}

	_ = w.Flush()
	fmt.Fprintln(out)
	output.DoctorSummary(out, warnings, failures)

	if failures > 0 {
		os.Exit(1)
	}
	return nil
}
