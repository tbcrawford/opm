package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbcrawford/opm/internal/store"
)

func TestStore_ShowActiveProfile_ReturnsActiveName(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))

	name, err := st.ShowActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "default", name)
}

func TestStore_ShowActiveProfile_RejectsForeignSymlink(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	foreign := t.TempDir()
	require.NoError(t, os.Symlink(foreign, opencodeDir))

	_, err := st.ShowActiveProfile()
	require.Error(t, err)
	assert.ErrorIs(t, err, store.ErrShowNotManaged)
}

func TestStore_ShowActiveProfile_MissingSymlinkReturnsNoActiveProfile(t *testing.T) {
	st, _ := newTestStore(t)

	_, err := st.ShowActiveProfile()
	require.Error(t, err)
	assert.ErrorIs(t, err, store.ErrShowNoActiveProfile)
}

func TestStore_ShowActiveProfile_DanglingManagedSymlinkReturnsNoActiveProfile(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, os.RemoveAll(st.ProfileDir("default")))

	_, err := st.ShowActiveProfile()
	require.Error(t, err)
	assert.ErrorIs(t, err, store.ErrShowNoActiveProfile)
}

func TestStore_ShowActiveProfile_ManagedMissingSymlinkWithCurrentCacheReturnsNoActiveProfile(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, st.SetCurrent("default"))
	require.NoError(t, os.Remove(opencodeDir))

	_, err := st.ShowActiveProfile()
	require.Error(t, err)
	assert.ErrorIs(t, err, store.ErrShowNoActiveProfile)
}

func TestStore_ShowActiveProfile_ExistingDirectoryReturnsNotManaged(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, os.MkdirAll(opencodeDir, 0o755))

	_, err := st.ShowActiveProfile()
	require.Error(t, err)
	assert.ErrorIs(t, err, store.ErrShowNotManaged)
}

func TestStore_ShowActiveProfile_InspectFailureIsWrapped(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	parent := filepath.Dir(opencodeDir)
	info, err := os.Stat(parent)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(parent, 0o000))
	t.Cleanup(func() { _ = os.Chmod(parent, info.Mode()) })

	_, err = st.ShowActiveProfile()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot determine opm state")
}
