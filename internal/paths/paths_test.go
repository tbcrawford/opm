package paths_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tbcrawford/opm/internal/paths"
	"github.com/stretchr/testify/assert"
)

func TestOpmDir_IsAbsolute(t *testing.T) {
	dir := paths.OpmDir()
	assert.True(t, filepath.IsAbs(dir), "OpmDir() must be absolute, got %q", dir)
}

func TestOpmDir_ContainsConfigOpm(t *testing.T) {
	dir := paths.OpmDir()
	assert.True(t, strings.Contains(dir, ".config/opm"), "OpmDir() should contain .config/opm, got %q", dir)
}

func TestOpencodeConfigDir_IsAbsolute(t *testing.T) {
	dir := paths.OpencodeConfigDir()
	assert.True(t, filepath.IsAbs(dir), "OpencodeConfigDir() must be absolute, got %q", dir)
}

func TestOpencodeConfigDir_ContainsConfigOpencode(t *testing.T) {
	dir := paths.OpencodeConfigDir()
	assert.True(t, strings.Contains(dir, ".config/opencode"), "OpencodeConfigDir() should contain .config/opencode, got %q", dir)
}

func TestProfilesDir_IsAbsolute(t *testing.T) {
	dir := paths.ProfilesDir()
	assert.True(t, filepath.IsAbs(dir), "ProfilesDir() must be absolute, got %q", dir)
}

func TestProfilesDir_IsUnderOpmDir(t *testing.T) {
	opm := paths.OpmDir()
	profiles := paths.ProfilesDir()
	assert.True(t, strings.HasPrefix(profiles, opm), "ProfilesDir() should be under OpmDir(), got %q vs %q", profiles, opm)
}

func TestProfileDir_IsAbsolute(t *testing.T) {
	dir := paths.ProfileDir("work")
	assert.True(t, filepath.IsAbs(dir), "ProfileDir() must be absolute, got %q", dir)
}

func TestProfileDir_ContainsName(t *testing.T) {
	dir := paths.ProfileDir("work")
	assert.True(t, strings.HasSuffix(dir, "/work"), "ProfileDir(\"work\") should end with /work, got %q", dir)
}

func TestProfileDir_IsUnderProfilesDir(t *testing.T) {
	profiles := paths.ProfilesDir()
	dir := paths.ProfileDir("myprofile")
	assert.True(t, strings.HasPrefix(dir, profiles), "ProfileDir() should be under ProfilesDir(), got %q", dir)
}

func TestCurrentFile_IsAbsolute(t *testing.T) {
	f := paths.CurrentFile()
	assert.True(t, filepath.IsAbs(f), "CurrentFile() must be absolute, got %q", f)
}

func TestCurrentFile_IsUnderOpmDir(t *testing.T) {
	opm := paths.OpmDir()
	f := paths.CurrentFile()
	assert.True(t, strings.HasPrefix(f, opm), "CurrentFile() should be under OpmDir(), got %q", f)
}

func TestCurrentFile_EndsWithCurrent(t *testing.T) {
	f := paths.CurrentFile()
	assert.True(t, strings.HasSuffix(f, "/current"), "CurrentFile() should end with /current, got %q", f)
}

// Ensure none of the paths use UserConfigDir (which returns ~/Library/Application Support on macOS).
// We enforce this at the test level by verifying no path contains "Library/Application Support".
func TestPaths_NoLibraryApplicationSupport(t *testing.T) {
	forbidden := "Library/Application Support"
	fns := map[string]string{
		"OpmDir":            paths.OpmDir(),
		"OpencodeConfigDir": paths.OpencodeConfigDir(),
		"ProfilesDir":       paths.ProfilesDir(),
		"ProfileDir":        paths.ProfileDir("test"),
		"CurrentFile":       paths.CurrentFile(),
	}
	for name, p := range fns {
		assert.False(t, strings.Contains(p, forbidden), "%s() must not use UserConfigDir path (got %q)", name, p)
	}
}
