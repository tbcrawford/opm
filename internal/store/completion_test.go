package store_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_CompletableProfiles_UnmanagedReturnsEmpty(t *testing.T) {
	st, _ := newTestStore(t)

	names, managed, err := st.CompletableProfiles(nil)
	require.NoError(t, err)
	assert.False(t, managed)
	assert.Nil(t, names)
}

func TestStore_CompletableProfiles_ExcludesSelectedNames(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, st.CreateProfile("personal"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))

	names, managed, err := st.CompletableProfiles([]string{"work"})
	require.NoError(t, err)
	assert.True(t, managed)
	assert.Equal(t, []string{"default", "personal"}, names)
}

func TestStore_CompletableProfiles_ExcludesDanglingProfiles(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))
	require.NoError(t, os.RemoveAll(st.ProfileDir("work")))

	names, managed, err := st.CompletableProfiles(nil)
	require.NoError(t, err)
	assert.True(t, managed)
	assert.Equal(t, []string{"default"}, names)
}

func TestStore_CompletableProfiles_ManagedWithNoCandidatesReturnsEmptySlice(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, os.Symlink(st.ProfileDir("default"), opencodeDir))

	names, managed, err := st.CompletableProfiles([]string{"default"})
	require.NoError(t, err)
	assert.True(t, managed)
	assert.Empty(t, names)
}
