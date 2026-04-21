package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbcrawford/opm/internal/store"
)

func breakStoreCurrentPath(t *testing.T, st *store.Store) {
	t.Helper()
	currentPath := filepath.Join(st.OpmDir(), "current")
	err := os.Remove(currentPath)
	if err != nil && !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	require.NoError(t, os.MkdirAll(currentPath, 0o755))
}

func TestStore_Initialize_FreshNoOpencode(t *testing.T) {
	st, opencodeDir := newTestStore(t)

	result, err := st.Initialize("default")
	require.NoError(t, err)
	assert.False(t, result.Migrated)
	assert.NoError(t, result.CurrentCacheErr)
	assert.Equal(t, st.ProfileDir("default"), result.ProfileDir)

	target, err := os.Readlink(opencodeDir)
	require.NoError(t, err)
	assert.Equal(t, st.ProfileDir("default"), target)

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "default", active)

	current, err := st.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "default", current)
	assert.DirExists(t, st.ProfileDir("default"))
}

func TestStore_Initialize_InvalidName(t *testing.T) {
	st, _ := newTestStore(t)

	_, err := st.Initialize("../evil")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

func TestStore_Initialize_MigratesExistingOpencodeDir(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, os.MkdirAll(opencodeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(opencodeDir, "opencode.json"), []byte(`{}`), 0o644))

	result, err := st.Initialize("default")
	require.NoError(t, err)
	assert.True(t, result.Migrated)
	assert.NoError(t, result.CurrentCacheErr)

	target, err := os.Readlink(opencodeDir)
	require.NoError(t, err)
	assert.Equal(t, st.ProfileDir("default"), target)
	assert.FileExists(t, filepath.Join(st.ProfileDir("default"), "opencode.json"))

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "default", active)

	current, err := st.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "default", current)
}

func TestStore_Initialize_AlreadyInitializedReportsActiveProfile(t *testing.T) {
	st, _ := newTestStore(t)
	_, err := st.Initialize("default")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(st.ProfileDir("work"), 0o755))

	_, err = st.Initialize("work")
	require.Error(t, err)
	assert.Equal(t, "already initialized (active: default)", err.Error())
}

func TestStore_Initialize_AlreadyInitializedUsesDanglingManagedTargetName(t *testing.T) {
	st, _ := newTestStore(t)
	_, err := st.Initialize("default")
	require.NoError(t, err)
	require.NoError(t, os.Remove(st.ProfileDir("default")))

	_, err = st.Initialize("work")
	require.Error(t, err)
	assert.Equal(t, "already initialized (active: default)", err.Error())
}

func TestStore_Initialize_ResumesMatchingTempSymlink(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	profileDir := st.ProfileDir("default")
	require.NoError(t, os.MkdirAll(profileDir, 0o755))
	require.NoError(t, os.Symlink(profileDir, opencodeDir+".opm-new"))

	result, err := st.Initialize("default")
	require.NoError(t, err)
	assert.True(t, result.Migrated)
	assert.NoError(t, result.CurrentCacheErr)

	target, err := os.Readlink(opencodeDir)
	require.NoError(t, err)
	assert.Equal(t, profileDir, target)
	_, err = os.Lstat(opencodeDir + ".opm-new")
	assert.ErrorIs(t, err, os.ErrNotExist)

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "default", active)

	current, err := st.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "default", current)
}

func TestStore_Initialize_RejectsStaleTempSymlink(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	foreignDir := filepath.Join(t.TempDir(), "foreign")
	require.NoError(t, os.MkdirAll(foreignDir, 0o755))
	require.NoError(t, os.Symlink(foreignDir, opencodeDir+".opm-new"))

	_, err := st.Initialize("default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stale interrupted init state detected")

	_, statErr := os.Lstat(opencodeDir)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
	assert.NoDirExists(t, st.ProfileDir("default"))
}

func TestStore_Initialize_RejectsStaleTempSymlinkUsingActualTempPath(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	foreignDir := filepath.Join(t.TempDir(), "foreign")
	require.NoError(t, os.MkdirAll(foreignDir, 0o755))
	require.NoError(t, os.Symlink(foreignDir, opencodeDir+".opm-new"))

	_, err := st.Initialize("default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), opencodeDir+".opm-new")
	assert.NotContains(t, err.Error(), "~/.config/opencode.opm-new")
}

func TestStore_Initialize_PartialProfileDirErrorUsesActualPaths(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	profileDir := st.ProfileDir("default")
	require.NoError(t, os.MkdirAll(profileDir, 0o755))

	_, err := st.Initialize("default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), profileDir)
	assert.Contains(t, err.Error(), opencodeDir)
	assert.NotContains(t, err.Error(), "~/.config/opencode")
	assert.NotContains(t, err.Error(), "~/.config/opm/profiles/default")
}

func TestStore_Initialize_CurrentWriteFailureIsNonFatal(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	breakStoreCurrentPath(t, st)

	result, err := st.Initialize("default")
	require.NoError(t, err)
	require.Error(t, result.CurrentCacheErr)
	assert.Contains(t, result.CurrentCacheErr.Error(), "write current")

	active, activeErr := st.ActiveProfile()
	require.NoError(t, activeErr)
	assert.Equal(t, "default", active)
	assert.DirExists(t, st.ProfileDir("default"))
}

// TestStore_Initialize_ReinitAfterReset checks the primary scenario:
// opm reset left a plain opencodeDir + profile dir intact; opm init
// should remove the plain dir and reinstall the symlink.
func TestStore_Initialize_ReinitAfterReset(t *testing.T) {
	st, opencodeDir := newTestStore(t)

	// Simulate a completed init.
	_, err := st.Initialize("default")
	require.NoError(t, err)

	// Simulate opm reset: replace symlink with a plain directory copy.
	profileDir := st.ProfileDir("default")
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "opencode.json"), []byte(`{}`), 0o644))
	require.NoError(t, os.Remove(opencodeDir))                       // remove symlink
	require.NoError(t, os.MkdirAll(opencodeDir, 0o755))              // restore as plain dir
	require.NoError(t, os.WriteFile(filepath.Join(opencodeDir, "opencode.json"), []byte(`{}`), 0o644))
	_ = os.Remove(filepath.Join(st.OpmDir(), "current"))             // reset removes current file

	// Re-init should succeed.
	result, err := st.Initialize("default")
	require.NoError(t, err)
	assert.False(t, result.Migrated)
	assert.NoError(t, result.CurrentCacheErr)
	assert.Equal(t, profileDir, result.ProfileDir)

	// opencodeDir must now be a symlink to the profile.
	target, err := os.Readlink(opencodeDir)
	require.NoError(t, err)
	assert.Equal(t, profileDir, target)

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "default", active)

	current, err := st.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "default", current)

	// Profile directory contents must be preserved — reinit only removes the plain opencodeDir copy.
	assert.FileExists(t, filepath.Join(profileDir, "opencode.json"))
}

// TestStore_Initialize_ReinitAfterReset_MultipleProfiles checks that when
// multiple profiles exist (from before the reset) they are left untouched
// and the named profile is activated.
func TestStore_Initialize_ReinitAfterReset_MultipleProfiles(t *testing.T) {
	st, opencodeDir := newTestStore(t)

	// Simulate a completed init with two profiles.
	_, err := st.Initialize("default")
	require.NoError(t, err)
	require.NoError(t, st.CreateProfile("work"))

	// Simulate opm reset: remove symlink, restore plain dir.
	defaultProfileDir := st.ProfileDir("default")
	require.NoError(t, os.Remove(opencodeDir))
	require.NoError(t, os.MkdirAll(opencodeDir, 0o755))
	_ = os.Remove(filepath.Join(st.OpmDir(), "current"))

	// Re-init with --as work.
	workProfileDir := st.ProfileDir("work")
	result, err := st.Initialize("work")
	require.NoError(t, err)
	assert.Equal(t, workProfileDir, result.ProfileDir)

	target, err := os.Readlink(opencodeDir)
	require.NoError(t, err)
	assert.Equal(t, workProfileDir, target)

	// "default" profile must still be intact.
	assert.DirExists(t, defaultProfileDir)
}

// TestStore_Initialize_ReinitAfterReset_NewProfileName checks that when
// --as names a profile that does NOT exist, init falls through to normal
// migration (moves opencodeDir into the new profile).
func TestStore_Initialize_ReinitAfterReset_NewProfileName(t *testing.T) {
	st, opencodeDir := newTestStore(t)

	// Simulate a completed init.
	_, err := st.Initialize("default")
	require.NoError(t, err)

	// Simulate opm reset.
	require.NoError(t, os.Remove(opencodeDir))
	require.NoError(t, os.MkdirAll(opencodeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(opencodeDir, "marker.txt"), []byte("hello"), 0o644))
	_ = os.Remove(filepath.Join(st.OpmDir(), "current"))

	// Re-init with --as fresh (does not exist).
	freshProfileDir := st.ProfileDir("fresh")
	result, err := st.Initialize("fresh")
	require.NoError(t, err)
	assert.True(t, result.Migrated)
	assert.Equal(t, freshProfileDir, result.ProfileDir)

	// opencodeDir must now be a symlink to the new profile.
	target, err := os.Readlink(opencodeDir)
	require.NoError(t, err)
	assert.Equal(t, freshProfileDir, target)

	// The marker file from opencodeDir must have been migrated.
	assert.FileExists(t, filepath.Join(freshProfileDir, "marker.txt"))
}
