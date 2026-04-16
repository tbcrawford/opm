package output_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/fatih/color"
	"github.com/opm-cli/opm/internal/output"
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
	output.Failure(&buf, "Cannot remove the active profile", "Switch first: opm context use <name>")
	assert.Equal(t, "✗ Cannot remove the active profile\n  Switch first: opm context use <name>\n", buf.String())
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
	output.Error(&buf, "cannot remove the active profile\n\n  Switch first: opm context use <name>")
	assert.Equal(t, "✗ cannot remove the active profile\n\n  Switch first: opm context use <name>\n", buf.String())
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
