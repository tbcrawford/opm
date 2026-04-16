package output_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/store"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	color.NoColor = true // strip ANSI codes in tests
	os.Exit(m.Run())
}

func TestSuccess_WithDetail(t *testing.T) {
	var buf bytes.Buffer
	output.Success(&buf, "Switched to work", "~/.config/opencode → profiles/work")
	assert.Equal(t, "✓ Switched to work\n  ~/.config/opencode → profiles/work\n", buf.String())
}

func TestSuccess_NoDetail(t *testing.T) {
	var buf bytes.Buffer
	output.Success(&buf, "Removed profile work")
	assert.Equal(t, "✓ Removed profile work\n", buf.String())
}

func TestFailure_WithDetail(t *testing.T) {
	var buf bytes.Buffer
	output.Failure(&buf, "Cannot remove the active profile", "Switch first: opm use <name>")
	assert.Equal(t, "✗ Cannot remove the active profile\n  Switch first: opm use <name>\n", buf.String())
}

func TestFailure_NoDetail(t *testing.T) {
	var buf bytes.Buffer
	output.Failure(&buf, "Cannot remove the active profile")
	assert.Equal(t, "✗ Cannot remove the active profile\n", buf.String())
}

func TestError_SingleLine(t *testing.T) {
	var buf bytes.Buffer
	output.Error(&buf, "profile \"foo\" does not exist")
	assert.Equal(t, "✗ profile \"foo\" does not exist\n", buf.String())
}

func TestError_MultiLine(t *testing.T) {
	var buf bytes.Buffer
	output.Error(&buf, "cannot remove the active profile\n\n  Switch first: opm use <name>")
	assert.Equal(t, "✗ cannot remove the active profile\n\n  Switch first: opm use <name>\n", buf.String())
}

func TestProfileName(t *testing.T) {
	assert.Equal(t, "work", output.ProfileName("work"))
}

func TestShortenHome_UnderHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	assert.Equal(t, "~/foo/bar", output.ShortenHome(home+"/foo/bar"))
}

func TestShortenHome_NotUnderHome(t *testing.T) {
	assert.Equal(t, "/etc/foo", output.ShortenHome("/etc/foo"))
}

func TestProfileTable_Mixed(t *testing.T) {
	var buf bytes.Buffer
	profiles := []store.Profile{
		{Name: "work", Active: true},
		{Name: "personal"},
		{Name: "staging", Dangling: true},
	}
	output.ProfileTable(&buf, profiles)
	got := buf.String()
	assert.Contains(t, got, "● work")
	assert.Contains(t, got, "○ personal")
	assert.Contains(t, got, "✗ staging")
	assert.Contains(t, got, "(missing)")
}

func TestProfileTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	output.ProfileTable(&buf, nil)
	assert.Equal(t, "", buf.String())
}

func TestInspectProfile_Active(t *testing.T) {
	var buf bytes.Buffer
	entries := []os.DirEntry{}
	output.InspectProfile(&buf, "work", "/home/user/.config/opm/profiles/work", true, entries)
	got := buf.String()
	assert.Contains(t, got, "work")
	assert.Contains(t, got, "● active")
	assert.Contains(t, got, "Contents")
	assert.Contains(t, got, "(empty)")
}

func TestInspectProfile_Inactive(t *testing.T) {
	var buf bytes.Buffer
	output.InspectProfile(&buf, "personal", "/home/user/.config/opm/profiles/personal", false, nil)
	got := buf.String()
	assert.Contains(t, got, "personal")
	assert.NotContains(t, got, "● active")
}

func TestInspectProfile_WithEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "opencode.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	output.InspectProfile(&buf, "work", dir, false, entries)
	got := buf.String()
	assert.Contains(t, got, "agents/")
	assert.Contains(t, got, "opencode.json")
}

func TestDoctorRow_OK(t *testing.T) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	output.DoctorRow(tw, output.StatusOK, "everything fine")
	tw.Flush()
	assert.Contains(t, buf.String(), "✓")
	assert.Contains(t, buf.String(), "everything fine")
}

func TestDoctorRow_Fail(t *testing.T) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	output.DoctorRow(tw, output.StatusFail, "profile missing")
	tw.Flush()
	assert.Contains(t, buf.String(), "✗")
	assert.Contains(t, buf.String(), "profile missing")
}

func TestDoctorRow_Warn(t *testing.T) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	output.DoctorRow(tw, output.StatusWarn, "symlink is unusual")
	tw.Flush()
	assert.Contains(t, buf.String(), "⚠")
	assert.Contains(t, buf.String(), "symlink is unusual")
}

func TestDoctorSummary_Healthy(t *testing.T) {
	var buf bytes.Buffer
	output.DoctorSummary(&buf, 0, 0)
	assert.Contains(t, buf.String(), "All checks passed")
}

func TestDoctorSummary_Failures(t *testing.T) {
	var buf bytes.Buffer
	output.DoctorSummary(&buf, 0, 2)
	assert.Contains(t, buf.String(), "2 problem")
}

func TestDoctorSummary_Warnings(t *testing.T) {
	var buf bytes.Buffer
	output.DoctorSummary(&buf, 1, 0)
	assert.Contains(t, buf.String(), "1 warning")
}
