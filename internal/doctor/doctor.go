package doctor

import (
	"fmt"
	"os"

	"github.com/tbcrawford/opm/internal/store"
	"github.com/tbcrawford/opm/internal/symlink"
)

type Status int

const (
	StatusOK Status = iota
	StatusWarn
	StatusFail
)

type Row struct {
	Status      Status
	Message     string
	ProfileName string
}

type Section struct {
	Label string
	Rows  []Row
}

type Report struct {
	Sections     []Section
	WarningCount int
	FailureCount int
	HasFailures  bool
}

func Run(s *store.Store) Report {
	report := Report{}
	symlinkSection := Section{Label: "Symlink"}

	managed, err := s.IsOpmManaged()
	if err != nil {
		symlinkSection.Rows = append(symlinkSection.Rows, Row{
			Status:  StatusFail,
			Message: fmt.Sprintf("~/.config/opencode: %v", err),
		})
		report.Sections = append(report.Sections, symlinkSection)
		report.FailureCount = 1
		report.HasFailures = true
		return report
	}
	if !managed {
		symlinkSection.Rows = append(symlinkSection.Rows, Row{
			Status:  StatusFail,
			Message: "~/.config/opencode is not an opm-managed symlink — run 'opm init'",
		})
		report.Sections = append(report.Sections, symlinkSection)
		report.FailureCount = 1
		report.HasFailures = true
		return report
	}

	st, err := symlink.Inspect(s.OpencodeDir())
	if err != nil {
		symlinkSection.Rows = append(symlinkSection.Rows, Row{
			Status:  StatusFail,
			Message: fmt.Sprintf("inspect ~/.config/opencode: %v", err),
		})
		report.FailureCount++
	} else if st.Dangling {
		symlinkSection.Rows = append(symlinkSection.Rows, Row{
			Status:  StatusFail,
			Message: fmt.Sprintf("~/.config/opencode → %q (profile directory missing)", st.Target),
		})
		report.FailureCount++
	} else {
		activeName, _ := s.ActiveProfile()
		symlinkSection.Rows = append(symlinkSection.Rows, Row{
			Status:      StatusOK,
			Message:     "~/.config/opencode → %s",
			ProfileName: activeName,
		})
	}
	report.Sections = append(report.Sections, symlinkSection)

	profilesSection := Section{Label: "Profiles"}
	profiles, err := s.ListProfiles()
	if err != nil {
		profilesSection.Rows = append(profilesSection.Rows, Row{
			Status:  StatusFail,
			Message: fmt.Sprintf("list profiles: %v", err),
		})
		report.FailureCount++
	} else {
		for _, p := range profiles {
			if p.Dangling {
				profilesSection.Rows = append(profilesSection.Rows, Row{
					Status:      StatusFail,
					Message:     "%s — directory missing",
					ProfileName: p.Name,
				})
				report.FailureCount++
				continue
			}
			fi, statErr := os.Lstat(p.Path)
			if statErr != nil || !fi.IsDir() {
				profilesSection.Rows = append(profilesSection.Rows, Row{
					Status:      StatusFail,
					Message:     "%s — not a valid directory (" + p.Path + ")",
					ProfileName: p.Name,
				})
				report.FailureCount++
			} else {
				profilesSection.Rows = append(profilesSection.Rows, Row{
					Status:      StatusOK,
					Message:     "%s",
					ProfileName: p.Name,
				})
			}
		}
	}
	report.Sections = append(report.Sections, profilesSection)

	current, curErr := s.GetCurrent()
	active, actErr := s.ActiveProfile()
	if curErr == nil && actErr == nil && current != "" && active != "" && current != active {
		report.Sections = append(report.Sections, Section{
			Label: "Consistency",
			Rows: []Row{{
				Status:  StatusWarn,
				Message: fmt.Sprintf("current file says %q but active symlink points to %q", current, active),
			}},
		})
		report.WarningCount++
	}

	report.HasFailures = report.FailureCount > 0
	return report
}
