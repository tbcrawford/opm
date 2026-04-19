package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tbcrawford/opm/internal/store"
	"github.com/tbcrawford/opm/internal/symlink"
)

// cmdHarness wires a temp-dir store into the cmd package and returns helpers
// for running commands and capturing their output.
type cmdHarness struct {
	t           *testing.T
	store       *store.Store
	opencodeDir string
}

func (h *cmdHarness) currentPath() string {
	return filepath.Join(filepath.Dir(h.store.ProfilesDir()), "current")
}

func (h *cmdHarness) breakCurrentPath(t *testing.T) {
	t.Helper()
	currentPath := h.currentPath()
	err := os.Remove(currentPath)
	if err != nil && !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	require.NoError(t, os.MkdirAll(currentPath, 0o755))
}

func newHarness(t *testing.T) *cmdHarness {
	t.Helper()
	root := t.TempDir()
	opencodeDir := filepath.Join(t.TempDir(), "opencode")
	s := store.New(root, opencodeDir)
	return &cmdHarness{t: t, store: s, opencodeDir: opencodeDir}
}

// resetCmdFlags resets every flag on the command tree to its default value.
// Cobra does NOT reset flag values between Execute() calls on a shared command
// tree — state would leak from test to test without this.
func resetCmdFlags(root *cobra.Command) {
	root.Flags().VisitAll(func(f *pflag.Flag) { _ = f.Value.Set(f.DefValue) })
	for _, sub := range root.Commands() {
		resetCmdFlags(sub)
	}
}

// run executes the root command with the given args and returns stdout, stderr, and any error.
// It injects the harness store so no real filesystem is touched.
func (h *cmdHarness) run(args ...string) (stdout, stderr string, err error) {
	h.t.Helper()

	resetCmdFlags(rootCmd)

	// Inject store factory — restore after test.
	origFactory := storeFactory
	storeFactory = func() *store.Store { return h.store }
	h.t.Cleanup(func() { storeFactory = origFactory })

	var outBuf, errBuf bytes.Buffer
	rootCmd.SetOut(&outBuf)
	rootCmd.SetErr(&errBuf)
	h.t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetErr(nil)
	})

	rootCmd.SetArgs(args)
	h.t.Cleanup(func() { rootCmd.SetArgs(nil) })

	err = rootCmd.Execute()
	return outBuf.String(), errBuf.String(), err
}

func (h *cmdHarness) useStoreFactory(t *testing.T) {
	t.Helper()
	origFactory := storeFactory
	storeFactory = func() *store.Store { return h.store }
	t.Cleanup(func() { storeFactory = origFactory })
}

// mustInit is a helper that runs opm init and requires no error.
func (h *cmdHarness) mustInit(t *testing.T) {
	t.Helper()
	_, _, err := h.run("init")
	require.NoError(t, err)
}

// ── init ──────────────────────────────────────────────────────────────────────

func TestInit_FreshNoOpencode(t *testing.T) {
	h := newHarness(t)
	out, _, err := h.run("init")
	require.NoError(t, err)
	assert.Contains(t, out, "Initialized opm")

	// Symlink should be installed.
	managed, merr := h.store.IsOpmManaged()
	require.NoError(t, merr)
	assert.True(t, managed)
}

func TestInit_InvalidAsFlag(t *testing.T) {
	h := newHarness(t)
	_, _, err := h.run("init", "--as", "../evil")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--as")
}

func TestInit_AlreadyInitialized(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("init")
	require.Error(t, err)
	assert.Equal(t, "already initialized (active: default)", err.Error())
}

func TestInit_AlreadyInitialized_WithRelativeManagedSymlink(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.store.Init())
	require.NoError(t, os.MkdirAll(h.store.ProfileDir("work"), 0o755))

	rel, err := filepath.Rel(filepath.Dir(h.opencodeDir), h.store.ProfileDir("work"))
	require.NoError(t, err)
	require.NoError(t, os.Symlink(rel, h.opencodeDir))

	_, _, err = h.run("init")
	require.Error(t, err)
	assert.Equal(t, "already initialized (active: work)", err.Error())
}

func TestInit_AlreadyInitialized_WithExistingRequestedProfileReportsRealActiveProfile(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	require.NoError(t, os.MkdirAll(h.store.ProfileDir("work"), 0o755))

	_, _, err := h.run("init", "--as", "work")
	require.Error(t, err)
	assert.Equal(t, "already initialized (active: default)", err.Error())
}

func TestInit_WithExistingOpencodeDir(t *testing.T) {
	h := newHarness(t)
	// Create a real ~/.config/opencode directory to simulate existing config.
	require.NoError(t, os.MkdirAll(h.opencodeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(h.opencodeDir, "opencode.json"), []byte(`{}`), 0o644))

	out, _, err := h.run("init")
	require.NoError(t, err)
	assert.Contains(t, out, "Migrated")

	// Original file should now be inside the profile.
	profileDir := h.store.ProfileDir("default")
	assert.FileExists(t, filepath.Join(profileDir, "opencode.json"))

	managed, merr := h.store.IsOpmManaged()
	require.NoError(t, merr)
	assert.True(t, managed)
}

func TestInit_RefusesRegularFileAtOpencodePath(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, os.WriteFile(h.opencodeDir, []byte("not a directory"), 0o644))

	_, _, err := h.run("init")
	require.Error(t, err)
	assert.Contains(t, err.Error(), h.opencodeDir+" is not a directory or symlink")
	assert.Contains(t, err.Error(), "Back it up and remove it")
	assert.NoDirExists(t, h.store.ProfileDir("default"))
}

func TestInit_RejectsStaleResumeSymlink(t *testing.T) {
	h := newHarness(t)
	profileDir := h.store.ProfileDir("default")
	require.NoError(t, os.MkdirAll(profileDir, 0o755))

	foreignDir := filepath.Join(t.TempDir(), "foreign")
	require.NoError(t, os.MkdirAll(foreignDir, 0o755))
	require.NoError(t, os.Symlink(foreignDir, h.opencodeDir+".opm-new"))

	_, _, err := h.run("init")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stale interrupted init state detected")
	assert.Contains(t, err.Error(), "remove "+h.opencodeDir+".opm-new")

	tmpTarget, readErr := os.Readlink(h.opencodeDir + ".opm-new")
	require.NoError(t, readErr)
	assert.Equal(t, foreignDir, tmpTarget)
	_, statErr := os.Lstat(h.opencodeDir)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestInit_RejectsStaleTempSymlinkOnFreshPath(t *testing.T) {
	h := newHarness(t)

	foreignDir := filepath.Join(t.TempDir(), "foreign")
	require.NoError(t, os.MkdirAll(foreignDir, 0o755))
	require.NoError(t, os.Symlink(foreignDir, h.opencodeDir+".opm-new"))

	_, _, err := h.run("init")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stale interrupted init state detected")
	assert.Contains(t, err.Error(), "remove "+h.opencodeDir+".opm-new")

	tmpTarget, readErr := os.Readlink(h.opencodeDir + ".opm-new")
	require.NoError(t, readErr)
	assert.Equal(t, foreignDir, tmpTarget)
	_, statErr := os.Lstat(h.opencodeDir)
	assert.ErrorIs(t, statErr, os.ErrNotExist)
	assert.NoDirExists(t, h.store.ProfileDir("default"))
}

func TestInit_RejectsMalformedTempState(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, os.WriteFile(h.opencodeDir+".opm-new", []byte("bad temp state"), 0o644))

	_, _, err := h.run("init")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stale interrupted init state detected")
	assert.Contains(t, err.Error(), "remove "+h.opencodeDir+".opm-new")

	info, statErr := os.Lstat(h.opencodeDir + ".opm-new")
	require.NoError(t, statErr)
	assert.False(t, info.Mode()&os.ModeSymlink != 0)
	_, opencodeErr := os.Lstat(h.opencodeDir)
	assert.ErrorIs(t, opencodeErr, os.ErrNotExist)
	assert.NoDirExists(t, h.store.ProfileDir("default"))
}

func TestInit_ResumesMatchingTempSymlink(t *testing.T) {
	h := newHarness(t)
	profileDir := h.store.ProfileDir("default")
	require.NoError(t, os.MkdirAll(profileDir, 0o755))
	require.NoError(t, os.Symlink(profileDir, h.opencodeDir+".opm-new"))

	out, _, err := h.run("init")
	require.NoError(t, err)
	assert.Contains(t, out, "Initialized opm")

	target, readErr := os.Readlink(h.opencodeDir)
	require.NoError(t, readErr)
	assert.Equal(t, profileDir, target)
	_, statErr := os.Lstat(h.opencodeDir + ".opm-new")
	assert.ErrorIs(t, statErr, os.ErrNotExist)

	active, activeErr := h.store.ActiveProfile()
	require.NoError(t, activeErr)
	assert.Equal(t, "default", active)
}

func TestInit_RejectsResumeTargetThatIsNotDirectory(t *testing.T) {
	h := newHarness(t)
	profileDir := h.store.ProfileDir("default")
	require.NoError(t, os.MkdirAll(filepath.Dir(profileDir), 0o755))
	require.NoError(t, os.WriteFile(profileDir, []byte("not a profile directory"), 0o644))
	require.NoError(t, os.Symlink(profileDir, h.opencodeDir+".opm-new"))

	_, _, err := h.run("init")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "partial initialization detected")
	assert.Contains(t, err.Error(), "rm -rf "+profileDir)

	info, statErr := os.Lstat(profileDir)
	require.NoError(t, statErr)
	assert.False(t, info.IsDir())
	_, opencodeErr := os.Lstat(h.opencodeDir)
	assert.ErrorIs(t, opencodeErr, os.ErrNotExist)
	_, tmpErr := os.Lstat(h.opencodeDir + ".opm-new")
	require.NoError(t, tmpErr)
}

func TestInit_RejectsResumeWhenOpencodeDirStillExists(t *testing.T) {
	h := newHarness(t)
	profileDir := h.store.ProfileDir("default")
	require.NoError(t, os.MkdirAll(profileDir, 0o755))
	require.NoError(t, os.Symlink(profileDir, h.opencodeDir+".opm-new"))
	require.NoError(t, os.MkdirAll(h.opencodeDir, 0o755))

	_, _, err := h.run("init")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "partial initialization detected")
	assert.Contains(t, err.Error(), h.opencodeDir+" still exists")
	assert.Contains(t, err.Error(), "inspect and back up "+h.opencodeDir)
	assert.Contains(t, err.Error(), "remove "+h.opencodeDir)
	assert.Contains(t, err.Error(), "remove "+h.opencodeDir+".opm-new")
	assert.NotContains(t, err.Error(), "rm -rf "+profileDir)

	info, statErr := os.Lstat(h.opencodeDir)
	require.NoError(t, statErr)
	assert.True(t, info.IsDir())
	_, tmpErr := os.Lstat(h.opencodeDir + ".opm-new")
	require.NoError(t, tmpErr)
	active, activeErr := h.store.ActiveProfile()
	require.NoError(t, activeErr)
	assert.Empty(t, active)
}

func TestInit_CustomAsName(t *testing.T) {
	h := newHarness(t)
	_, _, err := h.run("init", "--as", "work")
	require.NoError(t, err)

	active, err := h.store.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "work", active)
}

func TestInit_CurrentWriteFailureWarnsButSucceeds(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.store.Init())
	h.breakCurrentPath(t)

	out, stderr, err := h.run("init")
	require.NoError(t, err)
	assert.Contains(t, out, "Initialized opm")
	assert.Contains(t, stderr, "⚠ Updated live symlink state")
	assert.Contains(t, stderr, "failed to update current cache")

	target, readErr := os.Readlink(h.opencodeDir)
	require.NoError(t, readErr)
	assert.Equal(t, h.store.ProfileDir("default"), target)
}

// ── use ───────────────────────────────────────────────────────────────────────

func TestUse_SwitchesProfile(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)

	out, _, err := h.run("use", "work")
	require.NoError(t, err)
	assert.Contains(t, out, "work")

	active, err := h.store.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "work", active)
}

func TestUse_AlreadyOnProfile(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	out, _, err := h.run("use", "default")
	require.NoError(t, err)
	assert.Contains(t, out, "Already on")
}

func TestUse_NonexistentProfile(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	_, _, err := h.run("use", "ghost")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestUse_InvalidName(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	_, _, err := h.run("use", "../evil")
	require.Error(t, err)
}

func TestUse_RequiresInit(t *testing.T) {
	h := newHarness(t)
	_, _, err := h.run("use", "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not managed by opm")
}

func TestUse_CurrentWriteFailureWarnsButSucceeds(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)
	h.breakCurrentPath(t)

	out, stderr, err := h.run("use", "work")
	require.NoError(t, err)
	assert.Contains(t, out, "work")
	assert.Contains(t, stderr, "⚠ Updated live symlink state")
	assert.Contains(t, stderr, "failed to update current cache")

	target, readErr := os.Readlink(h.opencodeDir)
	require.NoError(t, readErr)
	assert.Equal(t, h.store.ProfileDir("work"), target)
}

// ── create ────────────────────────────────────────────────────────────────────

func TestCreate_Basic(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	out, _, err := h.run("create", "work")
	require.NoError(t, err)
	assert.Contains(t, out, "work")
	assert.DirExists(t, h.store.ProfileDir("work"))
}

func TestCreate_Duplicate(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)

	_, _, err = h.run("create", "work")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCreate_FromExisting(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	// Write a file into default profile.
	require.NoError(t, os.WriteFile(filepath.Join(h.store.ProfileDir("default"), "settings.json"), []byte(`{}`), 0o644))

	_, _, err := h.run("create", "work", "--from", "default")
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(h.store.ProfileDir("work"), "settings.json"))
}

func TestCreate_InvalidName(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	_, _, err := h.run("create", "../evil")
	require.Error(t, err)
}

// ── list ──────────────────────────────────────────────────────────────────────

func TestList_ShowsProfiles(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)

	out, _, err := h.run("list")
	require.NoError(t, err)
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "work")
}

func TestList_ActiveMarked(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	out, _, err := h.run("list")
	require.NoError(t, err)
	// Active profile line contains ● marker.
	assert.True(t, strings.Contains(out, "default"), "default should appear in list")
}

func TestList_LongFlag(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	out, _, err := h.run("list", "-l")
	require.NoError(t, err)
	// Long format includes the path.
	assert.Contains(t, out, h.store.ProfileDir("default"))
}

// ── show ──────────────────────────────────────────────────────────────────────

func TestShow_PrintsActiveName(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	out, _, err := h.run("show")
	require.NoError(t, err)
	assert.Equal(t, "default\n", out)
}

func TestShow_BrokenManagedSymlinkErrors(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	// Point the symlink at a nonexistent profile directory to create a managed dangling symlink.
	goneDir := h.store.ProfileDir("default")
	require.NoError(t, os.RemoveAll(goneDir))
	require.NoError(t, os.Remove(h.opencodeDir))
	require.NoError(t, os.Symlink(goneDir, h.opencodeDir))
	require.NoError(t, h.store.SetCurrent("default"))

	_, _, err := h.run("show")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active profile")
}

func TestShow_BrokenSymlinkWithoutCurrentErrors(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	goneDir := h.store.ProfileDir("default")
	require.NoError(t, os.RemoveAll(goneDir))
	require.NoError(t, os.Remove(h.opencodeDir))
	require.NoError(t, os.Symlink(goneDir, h.opencodeDir))
	require.NoError(t, h.store.SetCurrent(""))

	_, _, err := h.run("show")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active profile")
}

func TestShow_MissingSymlinkErrorsEvenWithCurrentCache(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	require.NoError(t, os.Remove(h.opencodeDir))
	require.NoError(t, h.store.SetCurrent("default"))

	_, _, err := h.run("show")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active profile")
}

func TestShow_AbsentSymlinkWithoutCurrentErrors(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	require.NoError(t, os.Remove(h.opencodeDir))
	require.NoError(t, h.store.SetCurrent(""))

	_, _, err := h.run("show")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active profile")
}

func TestShow_ForeignSymlinkRejected(t *testing.T) {
	h := newHarness(t)
	foreign := t.TempDir()
	require.NoError(t, os.Symlink(foreign, h.opencodeDir))

	_, _, err := h.run("show")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not managed by opm")
}

// ── remove ────────────────────────────────────────────────────────────────────

func TestRemove_NonActive(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)

	_, _, err = h.run("remove", "work")
	require.NoError(t, err)
	assert.NoDirExists(t, h.store.ProfileDir("work"))
}

func TestRemove_ActiveRefused(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	_, _, err := h.run("remove", "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot remove the active profile")
}

func TestRemove_ActiveForced_AutoSwitches(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)

	_, _, err = h.run("use", "default")
	require.NoError(t, err)

	_, _, err = h.run("remove", "--force", "default")
	require.NoError(t, err)

	active, err := h.store.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "work", active)
	assert.NoDirExists(t, h.store.ProfileDir("default"))
}

func TestRemove_OnlyProfile_Refused(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	_, _, err := h.run("remove", "--force", "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot remove the only profile")
}

func TestRemove_MultipleNames(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "a")
	require.NoError(t, err)
	_, _, err = h.run("create", "b")
	require.NoError(t, err)

	_, _, err = h.run("remove", "a", "b")
	require.NoError(t, err)
	assert.NoDirExists(t, h.store.ProfileDir("a"))
	assert.NoDirExists(t, h.store.ProfileDir("b"))
}

func TestRemove_DuplicateNamesRejectedBeforeDeletion(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)

	_, _, err = h.run("remove", "work", "work")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specified more than once")
	assert.DirExists(t, h.store.ProfileDir("work"))
}

func TestRemove_NonexistentAborts(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "real")
	require.NoError(t, err)

	// "ghost" doesn't exist — should error before removing anything.
	_, _, err = h.run("remove", "real", "ghost")
	require.Error(t, err)
	// "real" should still exist (atomic all-or-nothing validation).
	assert.DirExists(t, h.store.ProfileDir("real"))
}

func TestRemove_ForceCurrentWriteFailureWarnsButSucceeds(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)
	h.breakCurrentPath(t)

	out, stderr, err := h.run("remove", "--force", "default")
	require.NoError(t, err)
	assert.Contains(t, out, "Removed profile")
	assert.Contains(t, stderr, "⚠ Updated live symlink state")
	assert.Contains(t, stderr, "failed to update current cache")
	assert.NoDirExists(t, h.store.ProfileDir("default"))

	target, readErr := os.Readlink(h.opencodeDir)
	require.NoError(t, readErr)
	assert.Equal(t, h.store.ProfileDir("work"), target)
}

func TestRemove_ForceDeleteFailureStillWarnsAfterAutoSwitch(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)
	h.breakCurrentPath(t)

	profilesDir := h.store.ProfilesDir()
	info, statErr := os.Stat(profilesDir)
	require.NoError(t, statErr)
	require.NoError(t, os.Chmod(profilesDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(profilesDir, info.Mode()) })

	_, stderr, err := h.run("remove", "--force", "default")
	require.Error(t, err)
	assert.Contains(t, stderr, "⚠ Updated live symlink state")
	assert.Contains(t, stderr, "failed to update current cache")
	assert.DirExists(t, h.store.ProfileDir("default"))

	target, readErr := os.Readlink(h.opencodeDir)
	require.NoError(t, readErr)
	assert.Equal(t, h.store.ProfileDir("work"), target)
	active, activeErr := h.store.ActiveProfile()
	require.NoError(t, activeErr)
	assert.Equal(t, "work", active)
}

// ── rename ────────────────────────────────────────────────────────────────────

func TestRename_Inactive(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "old")
	require.NoError(t, err)

	_, _, err = h.run("rename", "old", "new")
	require.NoError(t, err)
	assert.NoDirExists(t, h.store.ProfileDir("old"))
	assert.DirExists(t, h.store.ProfileDir("new"))
}

func TestRename_ActiveUpdatesSymlink(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	_, _, err := h.run("rename", "default", "primary")
	require.NoError(t, err)

	active, err := h.store.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "primary", active)
}

func TestRename_InvalidNewName(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	_, _, err := h.run("rename", "default", "../evil")
	require.Error(t, err)
}

func TestRename_ActiveCurrentWriteFailureWarnsButSucceeds(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	h.breakCurrentPath(t)

	out, stderr, err := h.run("rename", "default", "primary")
	require.NoError(t, err)
	assert.Contains(t, out, "Renamed")
	assert.Contains(t, stderr, "⚠ Updated live symlink state")
	assert.Contains(t, stderr, "failed to update current cache")
	assert.NoDirExists(t, h.store.ProfileDir("default"))
	assert.DirExists(t, h.store.ProfileDir("primary"))

	target, readErr := os.Readlink(h.opencodeDir)
	require.NoError(t, readErr)
	assert.Equal(t, h.store.ProfileDir("primary"), target)
}

// ── copy ──────────────────────────────────────────────────────────────────────

func TestCopy_Basic(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	require.NoError(t, os.WriteFile(filepath.Join(h.store.ProfileDir("default"), "cfg.json"), []byte(`{}`), 0o644))

	_, _, err := h.run("copy", "default", "backup")
	require.NoError(t, err)
	assert.DirExists(t, h.store.ProfileDir("backup"))
	assert.FileExists(t, filepath.Join(h.store.ProfileDir("backup"), "cfg.json"))
}

func TestCopy_InvalidSrcName(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	_, _, err := h.run("copy", "../evil", "dst")
	require.Error(t, err)
}

// ── completion ───────────────────────────────────────────────────────────────

func TestCopy_Completion_OnlyCompletesFirstArg(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)
	h.useStoreFactory(t)

	names, directive := copyCmd.ValidArgsFunction(copyCmd, nil, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.ElementsMatch(t, []string{"default", "work"}, names)

	names, directive = copyCmd.ValidArgsFunction(copyCmd, []string{"default"}, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Empty(t, names)
}

func TestRename_Completion_OnlyCompletesFirstArg(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)
	h.useStoreFactory(t)

	names, directive := renameCmd.ValidArgsFunction(renameCmd, []string{"default"}, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.Empty(t, names)
}

func TestRemove_Completion_ExcludesAlreadySelectedProfiles(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)
	_, _, err = h.run("create", "personal")
	require.NoError(t, err)
	h.useStoreFactory(t)

	names, directive := removeCmd.ValidArgsFunction(removeCmd, []string{"work"}, "")
	assert.Equal(t, cobra.ShellCompDirectiveNoFileComp, directive)
	assert.ElementsMatch(t, []string{"default", "personal"}, names)
}

// ── path ──────────────────────────────────────────────────────────────────────

func TestPath_PrintsPath(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	out, _, err := h.run("path", "default")
	require.NoError(t, err)
	assert.Equal(t, h.store.ProfileDir("default")+"\n", out)
}

func TestPath_NonexistentProfile(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	_, _, err := h.run("path", "ghost")
	require.Error(t, err)
}

func TestPath_HelpUsesAbsolutePathWording(t *testing.T) {
	h := newHarness(t)

	out, _, err := h.run("path", "--help")
	require.NoError(t, err)
	assert.Contains(t, out, "Print the absolute path to a profile directory")
}

// ── inspect ───────────────────────────────────────────────────────────────────

func TestInspect_ShowsInfo(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	require.NoError(t, os.WriteFile(filepath.Join(h.store.ProfileDir("default"), "opencode.json"), []byte(`{}`), 0o644))

	out, _, err := h.run("inspect", "default")
	require.NoError(t, err)
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "opencode.json")
}

// ── reset ─────────────────────────────────────────────────────────────────────

func TestReset_RestoresDirectory(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	require.NoError(t, os.WriteFile(filepath.Join(h.store.ProfileDir("default"), "opencode.json"), []byte(`{}`), 0o644))

	_, _, err := h.run("reset")
	require.NoError(t, err)

	// opencodeDir should now be a real directory.
	info, err := os.Lstat(h.opencodeDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.False(t, info.Mode()&os.ModeSymlink != 0)
	assert.FileExists(t, filepath.Join(h.opencodeDir, "opencode.json"))
}

// ── doctor ────────────────────────────────────────────────────────────────────

func TestDoctor_HealthyInstallation(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	out, _, err := h.run("doctor")
	require.NoError(t, err)
	assert.Contains(t, out, "All checks passed")
}

func TestDoctor_NotInitialized(t *testing.T) {
	h := newHarness(t)

	out, _, err := h.run("doctor")
	// doctor exits with errSilent (non-nil) on failure.
	assert.Error(t, err)
	assert.Contains(t, out, "not an opm-managed symlink")
}

func TestDoctor_ConsistencyMismatch(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)

	// Manually install symlink to "work" but leave current file as "default".
	require.NoError(t, symlink.SetAtomic(h.store.ProfileDir("work"), h.opencodeDir))
	// current file still says "default" — mismatch.

	out, _, err := h.run("doctor")
	// Consistency mismatch is a warning, not a failure — doctor should succeed.
	require.NoError(t, err)
	assert.Contains(t, out, "warning") // Consistency section appears
}

func TestRootHelp_OmitsCompletionCommand(t *testing.T) {
	h := newHarness(t)

	out, _, err := h.run("--help")
	require.NoError(t, err)
	assert.NotContains(t, out, "completion")
}

func TestBuildRootHelpSections_UsesCommandMetadata(t *testing.T) {
	root := &cobra.Command{Use: "opm", Short: "OpenCode profile manager"}
	setup := &cobra.Command{Use: "init", Short: "Initialize opm"}
	profiles := &cobra.Command{Use: "list", Aliases: []string{"ls"}, Short: "List all profiles"}
	hidden := &cobra.Command{Use: "completion", Short: "Generate completions"}

	markRootHelpGroup(setup, helpGroupSetup)
	markRootHelpGroup(profiles, helpGroupProfiles)
	root.AddCommand(setup, profiles, hidden)

	sections := buildRootHelpSections(root)
	require.Len(t, sections, 2)
	assert.Equal(t, helpGroupSetup, sections[0].label)
	require.Len(t, sections[0].entries, 1)
	assert.Equal(t, "init", sections[0].entries[0].name)
	assert.Equal(t, "Initialize opm", sections[0].entries[0].short)
	assert.Equal(t, "", sections[0].entries[0].alias)

	assert.Equal(t, helpGroupProfiles, sections[1].label)
	require.Len(t, sections[1].entries, 1)
	assert.Equal(t, "list", sections[1].entries[0].name)
	assert.Equal(t, "List all profiles", sections[1].entries[0].short)
	assert.Equal(t, "ls", sections[1].entries[0].alias)
}

func TestBuildRootHelpSections_RealRootCommandCoversExpectedCommands(t *testing.T) {
	sections := buildRootHelpSections(rootCmd)

	byGroup := make(map[string][]string)
	for _, section := range sections {
		for _, entry := range section.entries {
			byGroup[section.label] = append(byGroup[section.label], entry.name)
		}
	}

	assert.Equal(t, []string{"init", "doctor", "reset"}, byGroup[helpGroupSetup])
	assert.Equal(t, []string{"create", "copy", "use", "list", "show", "inspect", "rename", "remove"}, byGroup[helpGroupProfiles])
	assert.Equal(t, []string{"path"}, byGroup[helpGroupScripting])
}
