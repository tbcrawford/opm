package symlink_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/opm-cli/opm/internal/symlink"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspect_NotExist(t *testing.T) {
	dir := t.TempDir()
	s, err := symlink.Inspect(filepath.Join(dir, "nonexistent"))
	require.NoError(t, err)
	assert.False(t, s.Exists)
	assert.False(t, s.IsSymlink)
	assert.False(t, s.IsDir)
	assert.Empty(t, s.Target)
	assert.False(t, s.Dangling)
}

func TestInspect_RealDir(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "realdir")
	require.NoError(t, os.Mkdir(subdir, 0o755))

	s, err := symlink.Inspect(subdir)
	require.NoError(t, err)
	assert.True(t, s.Exists)
	assert.False(t, s.IsSymlink)
	assert.True(t, s.IsDir)
	assert.Empty(t, s.Target)
	assert.False(t, s.Dangling)
}

func TestInspect_RealFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(f, []byte("hello"), 0o644))

	s, err := symlink.Inspect(f)
	require.NoError(t, err)
	assert.True(t, s.Exists)
	assert.False(t, s.IsSymlink)
	assert.False(t, s.IsDir)
}

func TestInspect_ValidSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	require.NoError(t, os.Mkdir(target, 0o755))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))

	s, err := symlink.Inspect(link)
	require.NoError(t, err)
	assert.True(t, s.Exists)
	assert.True(t, s.IsSymlink)
	assert.Equal(t, target, s.Target)
	assert.False(t, s.Dangling)
}

func TestInspect_DanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "gone")
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))
	// target was never created — link is dangling

	s, err := symlink.Inspect(link)
	require.NoError(t, err)
	assert.True(t, s.Exists)
	assert.True(t, s.IsSymlink)
	assert.Equal(t, target, s.Target)
	assert.True(t, s.Dangling)
}

func TestSetAtomic_Fresh(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "profile")
	require.NoError(t, os.Mkdir(target, 0o755))
	link := filepath.Join(dir, "opencode")

	err := symlink.SetAtomic(target, link)
	require.NoError(t, err)

	// Verify symlink was created correctly.
	got, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, target, got)

	// Verify temp file is gone.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, e := range entries {
		assert.NotContains(t, e.Name(), ".opm-tmp-", "temp file should be cleaned up after success")
	}
}

func TestSetAtomic_Replace(t *testing.T) {
	dir := t.TempDir()
	target1 := filepath.Join(dir, "profile1")
	target2 := filepath.Join(dir, "profile2")
	require.NoError(t, os.Mkdir(target1, 0o755))
	require.NoError(t, os.Mkdir(target2, 0o755))
	link := filepath.Join(dir, "opencode")

	// Create initial symlink.
	require.NoError(t, os.Symlink(target1, link))

	// Replace atomically — must not fail with EEXIST.
	err := symlink.SetAtomic(target2, link)
	require.NoError(t, err)

	got, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, target2, got)
}

func TestSetAtomic_LeftoverTmpCleaned(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "profile")
	require.NoError(t, os.Mkdir(target, 0o755))
	link := filepath.Join(dir, "opencode")

	// Pre-create a leftover tmp file (simulates crash on previous run).
	tmpLink := filepath.Join(dir, ".opm-tmp-opencode")
	require.NoError(t, os.Symlink("/some/stale/target", tmpLink))

	// SetAtomic should handle it without error.
	err := symlink.SetAtomic(target, link)
	require.NoError(t, err)

	got, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, target, got)
}
