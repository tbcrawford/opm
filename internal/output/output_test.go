package output_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbcrawford/opm/internal/output"
	"github.com/tbcrawford/opm/internal/store"
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
	require.NoError(t, tw.Flush())
	assert.Contains(t, buf.String(), "✓")
	assert.Contains(t, buf.String(), "everything fine")
}

func TestDoctorRow_Fail(t *testing.T) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	output.DoctorRow(tw, output.StatusFail, "profile missing")
	require.NoError(t, tw.Flush())
	assert.Contains(t, buf.String(), "✗")
	assert.Contains(t, buf.String(), "profile missing")
}

func TestDoctorRow_Warn(t *testing.T) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	output.DoctorRow(tw, output.StatusWarn, "symlink is unusual")
	require.NoError(t, tw.Flush())
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

func TestProfileTableLong(t *testing.T) {
	profiles := []store.Profile{
		{Name: "default", Path: "/home/user/.config/opm/profiles/default", Active: false},
		{Name: "work", Path: "/home/user/.config/opm/profiles/work", Active: true},
	}
	var buf bytes.Buffer
	output.ProfileTableLong(&buf, profiles)
	out := buf.String()
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "/home/user/.config/opm/profiles/default")
	assert.Contains(t, out, "work")
	assert.Contains(t, out, "/home/user/.config/opm/profiles/work")
}

func TestDoctorSection(t *testing.T) {
	var buf bytes.Buffer
	output.DoctorSection(&buf, "Profiles")
	assert.Equal(t, "Profiles\n", buf.String())
}

func TestHelpSection(t *testing.T) {
	var buf strings.Builder
	output.HelpSection(&buf, "Setup")
	assert.Equal(t, "Setup\n", buf.String())
}

func TestHelpCommand(t *testing.T) {
	var buf strings.Builder
	output.HelpCommand(&buf, "init", "Initialize opm and migrate your existing OpenCode config", "")
	assert.Equal(t, "  init\tInitialize opm and migrate your existing OpenCode config\n", buf.String())
}

func TestHelpCommandWithAlias(t *testing.T) {
	var buf strings.Builder
	output.HelpCommand(&buf, "list", "List all profiles", "ls")
	assert.Equal(t, "  list\tList all profiles  (ls)\n", buf.String())
}

func TestHelpHeader(t *testing.T) {
	var buf strings.Builder
	output.HelpHeader(&buf, "opm", "OpenCode profile manager")
	assert.Equal(t, "opm — OpenCode profile manager\n\nUsage:\n  opm <command> [flags]\n\n", buf.String())
}

func TestHelpFlag(t *testing.T) {
	result := output.HelpFlag("--version", "Print version and exit")
	assert.Equal(t, "  --version    Print version and exit", result)
}

func TestSubcmdHelp_NoFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "use <name>",
		Short: "Switch to a profile",
	}
	var buf bytes.Buffer
	output.SubcmdHelp(&buf, cmd)
	out := buf.String()
	assert.Contains(t, out, "opm use — Switch to a profile\n")
	assert.Contains(t, out, "Usage:\n")
	assert.Contains(t, out, "  opm use <name>\n")
	assert.NotContains(t, out, "[flags]")
	assert.NotContains(t, out, "Flags:\n")
}

func TestSubcmdHelp_WithLong(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize opm",
		Long:  "Migrates ~/.config/opencode to a default profile.",
	}
	var buf bytes.Buffer
	output.SubcmdHelp(&buf, cmd)
	out := buf.String()
	assert.Contains(t, out, "Migrates ~/.config/opencode to a default profile.\n")
}

func TestSubcmdHelp_WithFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new profile",
	}
	cmd.Flags().String("from", "", "Copy an existing profile as the starting point")
	var buf bytes.Buffer
	output.SubcmdHelp(&buf, cmd)
	out := buf.String()
	assert.Contains(t, out, "--from")
	assert.Contains(t, out, "Copy an existing profile as the starting point")
}

func TestSubcmdHelp_WithShorthand(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all profiles",
	}
	cmd.Flags().BoolP("long", "l", false, "Show profile paths")
	var buf bytes.Buffer
	output.SubcmdHelp(&buf, cmd)
	out := buf.String()
	assert.Contains(t, out, "-l, --long")
}
