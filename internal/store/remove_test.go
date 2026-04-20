package store_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbcrawford/opm/internal/store"
)

func TestStore_RemoveProfiles_NonActive(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, st.SetCurrent("default"))

	result, err := st.RemoveProfiles([]string{"work"}, false)
	require.NoError(t, err)
	assert.Empty(t, result.SwitchedTo)
	assert.Equal(t, []string{"work"}, result.Removed)
	assert.NoError(t, result.CurrentCacheErr)
	assert.NoDirExists(t, st.ProfileDir("work"))

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "default", active)
}

func TestStore_RemoveProfiles_RejectsDuplicateNames(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	_, err := st.RemoveProfiles([]string{"work", "work"}, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specified more than once")
	assert.DirExists(t, st.ProfileDir("work"))
}

func TestStore_RemoveProfiles_RejectsOnlyProfileEvenWithForce(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, st.SetCurrent("default"))

	_, err := st.RemoveProfiles([]string{"default"}, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot remove the only profile")
	assert.DirExists(t, st.ProfileDir("default"))
}

func TestStore_RemoveProfiles_ForcedActiveAutoSwitches(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, st.SetCurrent("default"))

	result, err := st.RemoveProfiles([]string{"default"}, true)
	require.NoError(t, err)
	assert.Equal(t, "work", result.SwitchedTo)
	assert.Equal(t, []string{"default"}, result.Removed)
	assert.NoError(t, result.CurrentCacheErr)
	assert.NoDirExists(t, st.ProfileDir("default"))

	target, err := os.Readlink(opencodeDir)
	require.NoError(t, err)
	assert.Equal(t, st.ProfileDir("work"), target)

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "work", active)

	current, err := st.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "work", current)
}

func TestStore_RemoveProfiles_ForcedActiveCurrentWriteFailureIsNonFatal(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, st.SetCurrent("default"))
	breakStoreCurrentPath(t, st)

	result, err := st.RemoveProfiles([]string{"default"}, true)
	require.NoError(t, err)
	assert.Equal(t, "work", result.SwitchedTo)
	require.Error(t, result.CurrentCacheErr)
	assert.Contains(t, result.CurrentCacheErr.Error(), "write current")
	assert.NoDirExists(t, st.ProfileDir("default"))

	active, activeErr := st.ActiveProfile()
	require.NoError(t, activeErr)
	assert.Equal(t, "work", active)
}

func TestStore_RemoveProfiles_ForcedMultiRemoveExcludesAllRemovedProfiles(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, st.CreateProfile("personal"))
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))
	require.NoError(t, st.SetCurrent("work"))

	result, err := st.RemoveProfiles([]string{"work", "personal"}, true)
	require.NoError(t, err)
	assert.Equal(t, "default", result.SwitchedTo)
	assert.Equal(t, []string{"work", "personal"}, result.Removed)
	assert.NoError(t, result.CurrentCacheErr)
	assert.NoDirExists(t, st.ProfileDir("work"))
	assert.NoDirExists(t, st.ProfileDir("personal"))
	assert.DirExists(t, st.ProfileDir("default"))

	target, err := os.Readlink(opencodeDir)
	require.NoError(t, err)
	assert.Equal(t, st.ProfileDir("default"), target)

	current, err := st.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "default", current)
}

func TestStore_RemoveProfiles_ReturnsPartialResultWhenDeleteFailsAfterSwitch(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, st.SetCurrent("default"))
	breakStoreCurrentPath(t, st)
	restore := store.TestHookDeleteProfile(t, func(s *store.Store, name string, force bool) error {
		if name == "default" {
			return fmt.Errorf("delete profile %q: injected failure", name)
		}
		return s.DeleteProfile(name, force)
	})
	t.Cleanup(restore)

	result, err := st.RemoveProfiles([]string{"default"}, true)
	require.Error(t, err)
	assert.Equal(t, "work", result.SwitchedTo)
	assert.Equal(t, []string{"default"}, result.Removed)
	require.Error(t, result.CurrentCacheErr)
	assert.Contains(t, result.CurrentCacheErr.Error(), "write current")

	target, readErr := os.Readlink(opencodeDir)
	require.NoError(t, readErr)
	assert.Equal(t, st.ProfileDir("work"), target)
	assert.DirExists(t, st.ProfileDir("default"))
}
