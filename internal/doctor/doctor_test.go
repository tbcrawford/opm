package doctor_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbcrawford/opm/internal/doctor"
	"github.com/tbcrawford/opm/internal/store"
	"github.com/tbcrawford/opm/internal/symlink"
)

func newDoctorStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	root := t.TempDir()
	opencodeDir := filepath.Join(t.TempDir(), "opencode")
	return store.New(root, opencodeDir), opencodeDir
}

func TestRun_HealthyInstallation(t *testing.T) {
	st, opencodeDir := newDoctorStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, symlink.SetAtomic(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, st.SetCurrent("default"))

	report := doctor.Run(st)
	assert.False(t, report.HasFailures)
	assert.Equal(t, 0, report.WarningCount)
	assert.Equal(t, 0, report.FailureCount)
	require.Len(t, report.Sections, 2)
	assert.Equal(t, "Symlink", report.Sections[0].Label)
	assert.Equal(t, doctor.StatusOK, report.Sections[0].Rows[0].Status)
	assert.Equal(t, "~/.config/opencode → %s", report.Sections[0].Rows[0].Message)
	assert.Equal(t, "default", report.Sections[0].Rows[0].ProfileName)
	assert.Equal(t, "Profiles", report.Sections[1].Label)
	assert.Equal(t, doctor.StatusOK, report.Sections[1].Rows[0].Status)
	assert.Equal(t, "%s", report.Sections[1].Rows[0].Message)
	assert.Equal(t, "default", report.Sections[1].Rows[0].ProfileName)
}

func TestRun_UnmanagedInstallationFailsFast(t *testing.T) {
	st, opencodeDir := newDoctorStore(t)
	require.NoError(t, os.MkdirAll(opencodeDir, 0o755))

	report := doctor.Run(st)
	assert.True(t, report.HasFailures)
	assert.Equal(t, 0, report.WarningCount)
	assert.Equal(t, 1, report.FailureCount)
	require.Len(t, report.Sections, 1)
	assert.Equal(t, "Symlink", report.Sections[0].Label)
	assert.Equal(t, doctor.StatusFail, report.Sections[0].Rows[0].Status)
	assert.Contains(t, report.Sections[0].Rows[0].Message, "not an opm-managed symlink")
}

func TestRun_CurrentSymlinkMismatchAddsWarningSection(t *testing.T) {
	st, opencodeDir := newDoctorStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, symlink.SetAtomic(st.ProfileDir("work"), opencodeDir))
	require.NoError(t, st.SetCurrent("default"))

	report := doctor.Run(st)
	assert.False(t, report.HasFailures)
	assert.Equal(t, 1, report.WarningCount)
	assert.Equal(t, 0, report.FailureCount)
	require.Len(t, report.Sections, 3)
	assert.Equal(t, "Consistency", report.Sections[2].Label)
	assert.Equal(t, doctor.StatusWarn, report.Sections[2].Rows[0].Status)
	assert.Contains(t, report.Sections[2].Rows[0].Message, "current file says \"default\"")
	assert.Contains(t, report.Sections[2].Rows[0].Message, "active symlink points to \"work\"")
}
