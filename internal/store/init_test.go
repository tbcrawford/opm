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
