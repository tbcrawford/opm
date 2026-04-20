package store_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbcrawford/opm/internal/store"
)

func TestStore_RenameProfileAndRetarget_Inactive(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, st.CreateProfile("old"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, st.SetCurrent("default"))

	result, err := st.RenameProfileAndRetarget("old", "new")
	require.NoError(t, err)
	assert.False(t, result.WasActive)
	assert.NoError(t, result.CurrentCacheErr)
	assert.NoDirExists(t, st.ProfileDir("old"))
	assert.DirExists(t, st.ProfileDir("new"))

	target, err := os.Readlink(opencodeDir)
	require.NoError(t, err)
	assert.Equal(t, st.ProfileDir("default"), target)

	current, err := st.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "default", current)
}

func TestStore_RenameProfileAndRetarget_Active(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, st.SetCurrent("default"))

	result, err := st.RenameProfileAndRetarget("default", "primary")
	require.NoError(t, err)
	assert.True(t, result.WasActive)
	assert.NoError(t, result.CurrentCacheErr)
	assert.NoDirExists(t, st.ProfileDir("default"))
	assert.DirExists(t, st.ProfileDir("primary"))

	target, err := os.Readlink(opencodeDir)
	require.NoError(t, err)
	assert.Equal(t, st.ProfileDir("primary"), target)

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "primary", active)

	current, err := st.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "primary", current)
}

func TestStore_RenameProfileAndRetarget_ActiveCurrentWriteFailureIsNonFatal(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, st.SetCurrent("default"))
	breakStoreCurrentPath(t, st)

	result, err := st.RenameProfileAndRetarget("default", "primary")
	require.NoError(t, err)
	assert.True(t, result.WasActive)
	require.Error(t, result.CurrentCacheErr)
	assert.Contains(t, result.CurrentCacheErr.Error(), "write current")
	assert.NoDirExists(t, st.ProfileDir("default"))
	assert.DirExists(t, st.ProfileDir("primary"))

	target, readErr := os.Readlink(opencodeDir)
	require.NoError(t, readErr)
	assert.Equal(t, st.ProfileDir("primary"), target)

	active, activeErr := st.ActiveProfile()
	require.NoError(t, activeErr)
	assert.Equal(t, "primary", active)
}

func TestStore_RenameProfileAndRetarget_RollsBackWhenSymlinkUpdateFails(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, st.SetCurrent("default"))
	restore := store.TestHookRenameProfileAndRetarget(t, func(s *store.Store, oldName, newName string) (store.RenameProfileResult, error) {
		if err := s.RenameProfile(oldName, newName); err != nil {
			return store.RenameProfileResult{}, err
		}
		if rerr := os.Rename(s.ProfileDir(newName), s.ProfileDir(oldName)); rerr != nil {
			return store.RenameProfileResult{}, fmt.Errorf("update active symlink: injected failure; rollback also failed: %v — profile directory is at %q", rerr, newName)
		}
		return store.RenameProfileResult{}, fmt.Errorf("update active symlink: injected failure (rolled back directory rename)")
	})
	t.Cleanup(restore)

	result, err := st.RenameProfileAndRetarget("default", "primary")
	require.Error(t, err)
	assert.False(t, result.WasActive)
	assert.NoError(t, result.CurrentCacheErr)
	assert.DirExists(t, st.ProfileDir("default"))
	assert.NoDirExists(t, st.ProfileDir("primary"))

	target, readErr := os.Readlink(opencodeDir)
	require.NoError(t, readErr)
	assert.Equal(t, st.ProfileDir("default"), target)

	current, currentErr := st.GetCurrent()
	require.NoError(t, currentErr)
	assert.Equal(t, "default", current)
}
