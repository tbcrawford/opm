package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/doctor"
	"github.com/tbcrawford/opm/internal/output"
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
	report := doctor.Run(newStore())
	for i, section := range report.Sections {
		if i > 0 {
			_, _ = fmt.Fprintln(out)
		}
		output.DoctorSection(out, section.Label)
		for _, row := range section.Rows {
			output.DoctorRow(out, doctorStatus(row.Status), doctorMessage(row))
		}
	}
	_, _ = fmt.Fprintln(out)
	output.DoctorSummary(out, report.WarningCount, report.FailureCount)

	if report.HasFailures {
		return errSilent
	}
	return nil
}

func doctorStatus(status doctor.Status) output.DoctorStatus {
	switch status {
	case doctor.StatusWarn:
		return output.StatusWarn
	case doctor.StatusFail:
		return output.StatusFail
	default:
		return output.StatusOK
	}
}

func doctorMessage(row doctor.Row) string {
	if row.ProfileName == "" {
		return row.Message
	}
	return fmt.Sprintf(row.Message, output.ProfileName(row.ProfileName))
}
