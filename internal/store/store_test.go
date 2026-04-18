package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbcrawford/opm/internal/store"
	"github.com/tbcrawford/opm/internal/symlink"
)

// newTestStore creates a Store fully isolated in t.TempDir().
// opencodeDir is a separate temp subdir simulating ~/.config/opencode.
func newTestStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	root := t.TempDir()
	opencodeDir := filepath.Join(t.TempDir(), "opencode")
	return store.New(root, opencodeDir), opencodeDir
}

// ---- Init ----

func TestStore_Init_Idempotent(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.Init()) // second call must not error
}

// ---- GetCurrent / SetCurrent ----

func TestStore_GetCurrent_Empty(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	cur, err := st.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "", cur)
}

func TestStore_GetSetCurrent(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.SetCurrent("work"))
	cur, err := st.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "work", cur)
}

func TestStore_GetCurrent_TrimsWhitespace(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.SetCurrent("  work  "))
	cur, err := st.GetCurrent()
	require.NoError(t, err)
	assert.Equal(t, "work", cur)
}

// ---- ValidateName ----

func TestValidateName_ValidNames(t *testing.T) {
	valid := []string{
		"default",
		"work",
		"my-profile",
		"my.profile",
		"my_profile",
		"a",
		"A1",
		"profile123",
	}
	for _, name := range valid {
		t.Run(name, func(t *testing.T) {
			assert.NoError(t, store.ValidateName(name), "expected %q to be valid", name)
		})
	}
}

func TestValidateName_InvalidNames(t *testing.T) {
	invalid := []string{
		"",
		".hidden",                // leading dot
		"-start",                 // leading dash
		"has space",              // space
		"../evil",                // path traversal
		"/absolute",              // absolute path
		"a/b",                    // path separator
		"has\nnewline",           // newline
		string(make([]byte, 64)), // too long (64 chars)
	}
	for _, name := range invalid {
		t.Run(name, func(t *testing.T) {
			assert.Error(t, store.ValidateName(name), "expected %q to be invalid", name)
		})
	}
}

// ---- CreateProfile ----

func TestStore_CreateProfile_Valid(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))
	// Profile dir should exist.
	_, err := os.Stat(st.ProfileDir("work"))
	assert.NoError(t, err)
}

func TestStore_CreateProfile_InvalidName(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	err := st.CreateProfile("../evil")
	assert.Error(t, err)
}

func TestStore_CreateProfile_Duplicate(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))
	err := st.CreateProfile("work")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

// ---- ListProfiles ----

func TestStore_ListProfiles_Empty(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	profiles, err := st.ListProfiles()
	require.NoError(t, err)
	assert.Empty(t, profiles)
}

func TestStore_ListProfiles_Sorted(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("zebra"))
	require.NoError(t, st.CreateProfile("apple"))
	require.NoError(t, st.CreateProfile("mango"))

	profiles, err := st.ListProfiles()
	require.NoError(t, err)
	require.Len(t, profiles, 3)
	assert.Equal(t, "apple", profiles[0].Name)
	assert.Equal(t, "mango", profiles[1].Name)
	assert.Equal(t, "zebra", profiles[2].Name)
}

func TestStore_ListProfiles_ActiveMarked(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, st.CreateProfile("personal"))

	// Simulate symlink pointing to "work" profile.
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))

	profiles, err := st.ListProfiles()
	require.NoError(t, err)
	require.Len(t, profiles, 2)

	byName := make(map[string]store.Profile)
	for _, p := range profiles {
		byName[p.Name] = p
	}
	assert.True(t, byName["work"].Active, "work should be active")
	assert.False(t, byName["personal"].Active, "personal should not be active")
}

// ---- GetProfile ----

func TestStore_GetProfile_Exists(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))
	path, err := st.GetProfile("work")
	require.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestStore_GetProfile_NotExist(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	_, err := st.GetProfile("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestStore_GetProfile_RejectsTraversalName(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())

	_, err := st.GetProfile("../evil")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

// ---- DeleteProfile ----

func TestStore_DeleteProfile_NonActive(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, st.CreateProfile("personal"))

	// Make "personal" active.
	require.NoError(t, os.Symlink(st.ProfileDir("personal"), opencodeDir))

	// Deleting "work" (non-active) without force should succeed.
	err := st.DeleteProfile("work", false)
	assert.NoError(t, err)
}

func TestStore_DeleteProfile_ActiveRefused(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	// Make "work" active.
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))

	// Deleting active profile without force must fail.
	err := st.DeleteProfile("work", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot remove active")
}

func TestStore_DeleteProfile_ActiveForced(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	// Make "work" active.
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))

	// With force=true the store just removes the dir; caller handles symlink.
	err := st.DeleteProfile("work", true)
	assert.NoError(t, err)
}

func TestStore_DeleteProfile_NotExist(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	err := st.DeleteProfile("ghost", false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestStore_DeleteProfile_RejectsTraversalName(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())

	err := st.DeleteProfile("../evil", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

// ---- IsOpmManaged ----

func TestStore_IsOpmManaged_True(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	// Symlink opencodeDir → profile dir.
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.True(t, managed)
}

func TestStore_IsOpmManaged_RealDir(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())

	// opencodeDir is a real directory, not a symlink.
	require.NoError(t, os.Mkdir(opencodeDir, 0o755))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.False(t, managed)
}

func TestStore_IsOpmManaged_NotExist(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	// opencodeDir doesn't exist at all.
	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.False(t, managed)
}

func TestStore_IsOpmManaged_PreInitMissingProfilesReturnsFalse(t *testing.T) {
	st, opencodeDir := newTestStore(t)

	aliasParent := t.TempDir()
	aliasRoot := filepath.Join(aliasParent, "opm-root")
	require.NoError(t, os.Symlink(t.TempDir(), aliasRoot))
	require.NoError(t, os.Symlink(filepath.Join(aliasRoot, "profiles", "work"), opencodeDir))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.False(t, managed)
}

func TestStore_IsOpmManaged_PreInitDirectDanglingTargetReturnsFalse(t *testing.T) {
	st, opencodeDir := newTestStore(t)

	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.False(t, managed)
}

func TestStore_IsOpmManaged_ForeignSymlink(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())

	// Symlink points somewhere outside opm's profiles dir.
	foreign := t.TempDir()
	require.NoError(t, os.Symlink(foreign, opencodeDir))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.False(t, managed)
}

func TestStore_IsOpmManaged_RejectsEscapingTarget(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	root := filepath.Dir(st.ProfilesDir())
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))

	forged := st.ProfilesDir() + string(filepath.Separator) + "work" + string(filepath.Separator) + ".." + string(filepath.Separator) + ".." + string(filepath.Separator) + "outside"
	require.NoError(t, os.Symlink(forged, opencodeDir))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.False(t, managed)
}

func TestStore_IsOpmManaged_RelativeTarget(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	rel, err := filepath.Rel(filepath.Dir(opencodeDir), st.ProfileDir("work"))
	require.NoError(t, err)
	require.NoError(t, os.Symlink(rel, opencodeDir))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.True(t, managed)

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "work", active)
}

func TestStore_ActiveProfile_ForeignSymlinkReturnsEmpty(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())

	foreign := filepath.Join(t.TempDir(), "work")
	require.NoError(t, os.MkdirAll(foreign, 0o755))
	require.NoError(t, os.Symlink(foreign, opencodeDir))

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "", active)
}

func TestStore_ActiveProfile_DanglingManagedSymlinkReturnsEmpty(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))
	require.NoError(t, os.RemoveAll(st.ProfileDir("work")))

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "", active)
}

func TestStore_ProfileDirSymlinkEscapingStoreIsNotManaged(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	outside := filepath.Join(t.TempDir(), "outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.RemoveAll(st.ProfileDir("work")))
	require.NoError(t, os.Symlink(outside, st.ProfileDir("work")))
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.False(t, managed)

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "", active)
}

func TestStore_ProfileDirSymlinkWithinStoreIsNotManaged(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, st.CreateProfile("personal"))
	require.NoError(t, os.RemoveAll(st.ProfileDir("work")))
	require.NoError(t, os.Symlink(st.ProfileDir("personal"), st.ProfileDir("work")))
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.False(t, managed)

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "", active)

	profiles, err := st.ListProfiles()
	require.NoError(t, err)
	require.Len(t, profiles, 1)

	byName := make(map[string]store.Profile)
	for _, p := range profiles {
		byName[p.Name] = p
	}

	require.Contains(t, byName, "personal")
	assert.False(t, byName["personal"].Active)

	err = st.DeleteProfile("personal", false)
	assert.NoError(t, err)
}

func TestStore_DanglingProfileEntrySymlinkIsNotManaged(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	missing := filepath.Join(t.TempDir(), "missing-target")
	require.NoError(t, os.RemoveAll(st.ProfileDir("work")))
	require.NoError(t, os.Symlink(missing, st.ProfileDir("work")))
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.False(t, managed)

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "", active)

	profiles, err := st.ListProfiles()
	require.NoError(t, err)
	assert.Empty(t, profiles)
}

func TestStore_ForeignDanglingSymlinkIsNotManaged(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	foreignMissing := filepath.Join(t.TempDir(), "work")
	require.NoError(t, os.Symlink(foreignMissing, opencodeDir))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.False(t, managed)

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "", active)

	profiles, err := st.ListProfiles()
	require.NoError(t, err)
	require.Len(t, profiles, 1)

	byName := make(map[string]store.Profile)
	for _, p := range profiles {
		byName[p.Name] = p
	}

	require.Contains(t, byName, "work")
	assert.False(t, byName["work"].Active)
	assert.False(t, byName["work"].Dangling)
}

func TestStore_ManagedAliasAncestorPathIsTrusted(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	realRoot := filepath.Dir(st.ProfilesDir())
	aliasParent := t.TempDir()
	aliasRoot := filepath.Join(aliasParent, "opm-root")
	require.NoError(t, os.Symlink(realRoot, aliasRoot))
	require.NoError(t, os.Symlink(filepath.Join(aliasRoot, "profiles", "work"), opencodeDir))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.True(t, managed)

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "work", active)

	profiles, err := st.ListProfiles()
	require.NoError(t, err)
	require.Len(t, profiles, 1)

	byName := make(map[string]store.Profile)
	for _, p := range profiles {
		byName[p.Name] = p
	}

	require.Contains(t, byName, "work")
	assert.True(t, byName["work"].Active)
}

func TestStore_ProfileEntryRegularFileIsNotManaged(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, os.RemoveAll(st.ProfileDir("work")))
	require.NoError(t, os.WriteFile(st.ProfileDir("work"), []byte("not a directory"), 0o644))
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))

	managed, err := st.IsOpmManaged()
	require.NoError(t, err)
	assert.False(t, managed)

	active, err := st.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "", active)

	profiles, err := st.ListProfiles()
	require.NoError(t, err)
	assert.Empty(t, profiles)
}

func TestStore_ListProfiles_ForeignSymlinkDoesNotMarkActive(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, st.CreateProfile("personal"))

	foreign := filepath.Join(t.TempDir(), "work")
	require.NoError(t, os.MkdirAll(foreign, 0o755))
	require.NoError(t, os.Symlink(foreign, opencodeDir))

	profiles, err := st.ListProfiles()
	require.NoError(t, err)
	require.Len(t, profiles, 2)

	byName := make(map[string]store.Profile)
	for _, p := range profiles {
		byName[p.Name] = p
	}

	require.Contains(t, byName, "work")
	require.Contains(t, byName, "personal")
	assert.False(t, byName["work"].Active)
	assert.False(t, byName["personal"].Active)
}

func TestStore_DeleteProfile_ForeignSymlinkWithSameBasenameAllowsDeletion(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	foreign := filepath.Join(t.TempDir(), "work")
	require.NoError(t, os.MkdirAll(foreign, 0o755))
	require.NoError(t, os.Symlink(foreign, opencodeDir))

	err := st.DeleteProfile("work", false)
	assert.NoError(t, err)

	_, statErr := os.Lstat(st.ProfileDir("work"))
	assert.True(t, os.IsNotExist(statErr))
}

// ---- Dangling detection ----

func TestListProfiles_DanglingActive(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	// Symlink opencodeDir → work profile.
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))

	// Manually delete the profile dir (simulates user running rm -rf directly).
	require.NoError(t, os.RemoveAll(st.ProfileDir("work")))

	profiles, err := st.ListProfiles()
	require.NoError(t, err)

	// Should have exactly one profile: the dangling "work".
	require.Len(t, profiles, 1)
	p := profiles[0]
	assert.Equal(t, "work", p.Name)
	assert.True(t, p.Active)
	assert.True(t, p.Dangling, "profile dir is missing — should be Dangling=true")
}

func TestListProfiles_HealthyActive_NotDangling(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))

	profiles, err := st.ListProfiles()
	require.NoError(t, err)
	require.Len(t, profiles, 1)
	assert.False(t, profiles[0].Dangling, "healthy active profile should not be Dangling")
	assert.True(t, profiles[0].Active)
}

func TestListProfiles_DanglingDoesNotAffectOtherProfiles(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("default"))
	require.NoError(t, st.CreateProfile("work"))

	// Make "work" active, then delete it.
	require.NoError(t, os.Symlink(st.ProfileDir("work"), opencodeDir))
	require.NoError(t, os.RemoveAll(st.ProfileDir("work")))

	profiles, err := st.ListProfiles()
	require.NoError(t, err)

	// Should have 2 profiles: "default" (healthy) and "work" (dangling).
	require.Len(t, profiles, 2)

	byName := make(map[string]store.Profile)
	for _, p := range profiles {
		byName[p.Name] = p
	}
	assert.False(t, byName["default"].Dangling)
	assert.False(t, byName["default"].Active)
	assert.True(t, byName["work"].Dangling)
	assert.True(t, byName["work"].Active)
}

// ---- RenameProfile ----

func TestRenameProfile_Inactive(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	require.NoError(t, st.RenameProfile("work", "work-old"))

	// Old dir gone, new dir exists.
	_, err := os.Lstat(st.ProfileDir("work"))
	assert.True(t, os.IsNotExist(err), "old profile dir should be gone")
	_, err = os.Lstat(st.ProfileDir("work-old"))
	assert.NoError(t, err, "new profile dir should exist")
}

func TestRenameProfile_InvalidNewName(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	err := st.RenameProfile("work", "../evil")
	assert.Error(t, err, "path traversal in new name should be rejected")
}

func TestRenameProfile_OldNotExist(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())

	err := st.RenameProfile("ghost", "newname")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestRenameProfile_NewAlreadyExists(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))
	require.NoError(t, st.CreateProfile("personal"))

	err := st.RenameProfile("work", "personal")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestRenameProfile_SameNameIsNoop(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	// Renaming to same name: os.Rename is a no-op on most filesystems.
	err := st.RenameProfile("work", "work")
	// Either succeeds (no-op) or errors on "already exists" — both are acceptable.
	// The key thing is the profile still exists afterward.
	_ = err
	_, statErr := os.Lstat(st.ProfileDir("work"))
	assert.NoError(t, statErr, "profile should still exist after rename to same name")
}

// ---- Reset ----

func TestReset_NotManaged(t *testing.T) {
	root := t.TempDir()
	opencode := t.TempDir() // plain directory, not a symlink
	s := store.New(root, opencode)

	err := s.Reset()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not managed by opm")
}

func TestReset_RestoresDirectory(t *testing.T) {
	root := t.TempDir()
	opencodeDir := filepath.Join(t.TempDir(), "opencode")

	// Set up a profile with a file in it.
	s := store.New(root, opencodeDir)
	require.NoError(t, s.Init())
	require.NoError(t, s.CreateProfile("default"))
	profileDir := s.ProfileDir("default")

	// Write a file into the profile.
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "opencode.json"), []byte(`{}`), 0o644))

	// Install the symlink.
	require.NoError(t, symlink.SetAtomic(profileDir, opencodeDir))
	require.NoError(t, s.SetCurrent("default"))

	// Reset.
	require.NoError(t, s.Reset())

	// opencodeDir should now be a real directory, not a symlink.
	info, err := os.Lstat(opencodeDir)
	require.NoError(t, err)
	assert.False(t, info.Mode()&os.ModeSymlink != 0, "opencodeDir should not be a symlink after reset")
	assert.True(t, info.IsDir(), "opencodeDir should be a directory after reset")

	// The file from the profile should be present.
	assert.FileExists(t, filepath.Join(opencodeDir, "opencode.json"))

	// The current file should be gone.
	_, err = os.Stat(filepath.Join(root, "current"))
	assert.True(t, os.IsNotExist(err), "current file should be removed after reset")

	// Profile directory should still exist (copy, not move).
	assert.DirExists(t, profileDir)
	assert.FileExists(t, filepath.Join(profileDir, "opencode.json"))
}

// ---- CopyProfile ----

func TestCopyProfile_Basic(t *testing.T) {
	root := t.TempDir()
	s := store.New(root, t.TempDir())
	require.NoError(t, s.Init())
	require.NoError(t, s.CreateProfile("src"))
	// Write a file into src.
	require.NoError(t, os.WriteFile(filepath.Join(s.ProfileDir("src"), "opencode.json"), []byte(`{}`), 0o644))

	require.NoError(t, s.CopyProfile("src", "dst"))

	assert.DirExists(t, s.ProfileDir("dst"))
	assert.FileExists(t, filepath.Join(s.ProfileDir("dst"), "opencode.json"))
	// src still exists.
	assert.DirExists(t, s.ProfileDir("src"))
}

func TestCopyProfile_SrcMissing(t *testing.T) {
	root := t.TempDir()
	s := store.New(root, t.TempDir())
	require.NoError(t, s.Init())

	err := s.CopyProfile("nonexistent", "dst")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nonexistent" does not exist`)
}

func TestCopyProfile_DstExists(t *testing.T) {
	root := t.TempDir()
	s := store.New(root, t.TempDir())
	require.NoError(t, s.Init())
	require.NoError(t, s.CreateProfile("src"))
	require.NoError(t, s.CreateProfile("dst"))

	err := s.CopyProfile("src", "dst")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"dst" already exists`)
}

func TestCopyProfile_InvalidDstName(t *testing.T) {
	root := t.TempDir()
	s := store.New(root, t.TempDir())
	require.NoError(t, s.Init())
	require.NoError(t, s.CreateProfile("src"))

	err := s.CopyProfile("src", "bad name!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

func TestCopyProfile_SymlinkSourceRejected(t *testing.T) {
	root := t.TempDir()
	s := store.New(root, t.TempDir())
	require.NoError(t, s.Init())
	require.NoError(t, s.CreateProfile("src"))
	require.NoError(t, os.RemoveAll(s.ProfileDir("src")))

	target := filepath.Join(t.TempDir(), "foreign")
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.Symlink(target, s.ProfileDir("src")))

	err := s.CopyProfile("src", "dst")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context \"src\" is not a directory")
}

func TestReset_PreservesProfiles(t *testing.T) {
	root := t.TempDir()
	opencodeDir := filepath.Join(t.TempDir(), "opencode")

	s := store.New(root, opencodeDir)
	require.NoError(t, s.Init())
	require.NoError(t, s.CreateProfile("default"))
	require.NoError(t, s.CreateProfile("work"))

	profileDir := s.ProfileDir("default")
	require.NoError(t, symlink.SetAtomic(profileDir, opencodeDir))
	require.NoError(t, s.SetCurrent("default"))

	require.NoError(t, s.Reset())

	// Both profiles still exist.
	assert.DirExists(t, s.ProfileDir("default"))
	assert.DirExists(t, s.ProfileDir("work"))
}

func TestReset_RelativeManagedTargetSucceeds(t *testing.T) {
	root := t.TempDir()
	opencodeDir := filepath.Join(t.TempDir(), "opencode")

	s := store.New(root, opencodeDir)
	require.NoError(t, s.Init())
	require.NoError(t, s.CreateProfile("default"))
	profileDir := s.ProfileDir("default")
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "opencode.json"), []byte(`{}`), 0o644))

	rel, err := filepath.Rel(filepath.Dir(opencodeDir), profileDir)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(rel, opencodeDir))
	require.NoError(t, s.SetCurrent("default"))

	require.NoError(t, s.Reset())

	info, err := os.Lstat(opencodeDir)
	require.NoError(t, err)
	assert.False(t, info.Mode()&os.ModeSymlink != 0)
	assert.True(t, info.IsDir())
	assert.FileExists(t, filepath.Join(opencodeDir, "opencode.json"))
	_, err = os.Stat(filepath.Join(root, "current"))
	assert.True(t, os.IsNotExist(err))
}

func TestReset_RejectsEscapingManagedTarget(t *testing.T) {
	root := t.TempDir()
	opencodeDir := filepath.Join(t.TempDir(), "opencode")

	s := store.New(root, opencodeDir)
	require.NoError(t, s.Init())
	require.NoError(t, s.CreateProfile("work"))

	outside := filepath.Join(t.TempDir(), "outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.RemoveAll(s.ProfileDir("work")))
	require.NoError(t, os.Symlink(outside, s.ProfileDir("work")))
	require.NoError(t, os.Symlink(s.ProfileDir("work"), opencodeDir))

	err := s.Reset()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not managed by opm")
}
