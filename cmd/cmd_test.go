package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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

func newHarness(t *testing.T) *cmdHarness {
	t.Helper()
	root := t.TempDir()
	opencodeDir := filepath.Join(t.TempDir(), "opencode")
	s := store.New(root, opencodeDir)
	return &cmdHarness{t: t, store: s, opencodeDir: opencodeDir}
}

// run executes the root command with the given args and returns stdout, stderr, and any error.
// It injects the harness store so no real filesystem is touched.
func (h *cmdHarness) run(args ...string) (stdout, stderr string, err error) {
	h.t.Helper()

	// Reset all flag-bound package vars to their defaults before each Execute().
	// Cobra does NOT reset flag values between Execute() calls on a shared command
	// tree — persistent state would leak from test to test without this.
	initProfileName = "default"
	createFrom = ""
	listLong = false
	removeForce = false

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
	assert.Contains(t, err.Error(), "already initialized")
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

func TestInit_CustomAsName(t *testing.T) {
	h := newHarness(t)
	_, _, err := h.run("init", "--as", "work")
	require.NoError(t, err)

	active, err := h.store.ActiveProfile()
	require.NoError(t, err)
	assert.Equal(t, "work", active)
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

func TestShow_FallbackWarnsOnBrokenSymlink(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	// Point the symlink at a nonexistent profile directory to create a dangling symlink.
	// ActiveProfile() will still return the base name from Readlink (no error), so show
	// still prints the name — but it's the dangling target's basename.
	goneDir := h.store.ProfileDir("default") + "_gone"
	require.NoError(t, os.Remove(h.opencodeDir))
	require.NoError(t, os.Symlink(goneDir, h.opencodeDir))

	out, _, err := h.run("show")
	require.NoError(t, err)
	// ActiveProfile reads the symlink target and returns its base name even when dangling.
	assert.Equal(t, "default_gone\n", out)
}

func TestShow_FallbackWhenSymlinkMissingUsesCurrent(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	require.NoError(t, os.Remove(h.opencodeDir))
	require.NoError(t, h.store.SetCurrent("default"))

	out, stderr, err := h.run("show")
	require.NoError(t, err)
	assert.Equal(t, "default\n", out)
	assert.Contains(t, stderr, "warning: symlink is broken or absent")
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
