# Filesystem Safety and CLI Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden symlink and filesystem handling so `opm` only trusts verified managed targets, then fix the reviewed command bugs around `show`, `init`, cache-sync warnings, and root help discoverability.

**Architecture:** Add one private managed-link resolver inside `internal/store` and route `IsOpmManaged`, `ActiveProfile`, `ListProfiles`, and `Reset` through it. Then apply focused command-layer fixes in `cmd/` and keep each change covered by regression tests written first.

**Tech Stack:** Go, cobra, existing `cmd`, `internal/store`, `internal/symlink`, and `internal/output` packages.

All commands below assume the isolated worktree at `/Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening`.

---

### Task 1: Centralize managed symlink resolution in `internal/store`

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`

This task adds a single private resolver for `~/.config/opencode` and updates `IsOpmManaged`, `ActiveProfile`, and `ListProfiles` to trust only verified direct children of `profiles/`.

- [ ] **Step 1: Write the failing store regression tests**

Add these tests to `internal/store/store_test.go`:

```go
func TestStore_IsOpmManaged_RejectsEscapingTarget(t *testing.T) {
	st, opencodeDir := newTestStore(t)
	require.NoError(t, st.Init())
	require.NoError(t, st.CreateProfile("work"))

	root := filepath.Dir(st.ProfilesDir())
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))

	forged := filepath.Join(st.ProfilesDir(), "work", "..", "..", "outside")
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

	byName := make(map[string]store.Profile)
	for _, p := range profiles {
		byName[p.Name] = p
	}

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
```

- [ ] **Step 2: Run the targeted tests and confirm they fail for the current implementation**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./internal/store/... -run 'TestStore_(IsOpmManaged|ActiveProfile|ListProfiles|DeleteProfile_ForeignSymlinkWithSameBasenameAllowsDeletion)' -v
```

Expected:
- `TestStore_IsOpmManaged_RejectsEscapingTarget` fails because `strings.HasPrefix` treats the forged target as managed.
- `TestStore_ActiveProfile_ForeignSymlinkReturnsEmpty` fails because `ActiveProfile()` currently returns the basename of any symlink target.
- `TestStore_ListProfiles_ForeignSymlinkDoesNotMarkActive` fails because `ListProfiles()` trusts the symlink basename.
- `TestStore_DeleteProfile_ForeignSymlinkWithSameBasenameAllowsDeletion` fails because `DeleteProfile()` currently sees the foreign basename as active.

- [ ] **Step 3: Add the managed-link resolver and replace the three store methods**

In `internal/store/store.go`, add this private helper near `SetCurrent` and replace `ActiveProfile`, `ListProfiles`, and `IsOpmManaged` with these versions:

```go
type managedLinkState struct {
	Exists         bool
	IsSymlink      bool
	Managed        bool
	ResolvedTarget string
	ProfileName    string
	Dangling       bool
}

func (s *Store) managedLinkState() (managedLinkState, error) {
	st, err := symlink.Inspect(s.opencodeDir)
	if err != nil {
		return managedLinkState{}, err
	}

	state := managedLinkState{
		Exists:    st.Exists,
		IsSymlink: st.IsSymlink,
		Dangling:  st.Dangling,
	}
	if !st.IsSymlink {
		return state, nil
	}

	target := st.Target
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(s.opencodeDir), target)
	}
	target = filepath.Clean(target)
	state.ResolvedTarget = target

	rel, err := filepath.Rel(s.profilesDir(), target)
	if err != nil {
		return managedLinkState{}, fmt.Errorf("resolve managed target: %w", err)
	}
	if rel == "." || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return state, nil
	}
	if strings.Contains(rel, string(filepath.Separator)) {
		return state, nil
	}

	state.Managed = true
	state.ProfileName = rel
	return state, nil
}

func (s *Store) ActiveProfile() (string, error) {
	state, err := s.managedLinkState()
	if err != nil {
		return "", err
	}
	if !state.Managed || state.Dangling {
		return "", nil
	}
	return state.ProfileName, nil
}

func (s *Store) ListProfiles() ([]Profile, error) {
	state, err := s.managedLinkState()
	if err != nil {
		return nil, err
	}

	active := ""
	if state.Managed && !state.Dangling {
		active = state.ProfileName
	}

	entries, err := os.ReadDir(s.profilesDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}

	var profiles []Profile
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		profiles = append(profiles, Profile{
			Name:   name,
			Path:   s.ProfileDir(name),
			Active: name == active,
		})
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	if state.Managed && state.Dangling {
		profiles = append(profiles, Profile{
			Name:     state.ProfileName,
			Path:     state.ResolvedTarget,
			Active:   true,
			Dangling: true,
		})
		sort.Slice(profiles, func(i, j int) bool {
			return profiles[i].Name < profiles[j].Name
		})
	}

	return profiles, nil
}

func (s *Store) IsOpmManaged() (bool, error) {
	state, err := s.managedLinkState()
	if err != nil {
		return false, err
	}
	return state.Managed, nil
}
```

- [ ] **Step 4: Run the targeted tests again and confirm they pass**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./internal/store/... -run 'TestStore_(IsOpmManaged|ActiveProfile|ListProfiles|DeleteProfile_ForeignSymlinkWithSameBasenameAllowsDeletion)' -v
```

Expected: all six new tests pass.

- [ ] **Step 5: Run the full store package tests**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./internal/store/... -v
```

Expected: the existing store tests plus the new managed-symlink tests all pass.

- [ ] **Step 6: Commit the resolver change**

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && git add internal/store/store.go internal/store/store_test.go && git commit -m "fix(store): verify managed symlink targets before trusting them"
```

---

### Task 2: Harden store path validation, copy safety, and reset behavior

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`

This task closes the remaining store-level review findings: internal name validation, symlinked copy sources, safe active-profile deletion checks, and `Reset()` using the verified managed-target resolver.

- [ ] **Step 1: Write the failing store tests for validation and reset safety**

Add these tests to `internal/store/store_test.go`:

```go
func TestStore_GetProfile_RejectsTraversalName(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())

	_, err := st.GetProfile("../evil")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

func TestStore_DeleteProfile_RejectsTraversalName(t *testing.T) {
	st, _ := newTestStore(t)
	require.NoError(t, st.Init())

	err := st.DeleteProfile("../evil", true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}

func TestCopyProfile_SymlinkSourceRejected(t *testing.T) {
	root := t.TempDir()
	s := store.New(root, t.TempDir())
	require.NoError(t, s.Init())

	outside := filepath.Join(t.TempDir(), "outside-src")
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.Symlink(outside, s.ProfileDir("src")))

	err := s.CopyProfile("src", "dst")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"src" is not a directory`)
}

func TestReset_RejectsEscapingManagedTarget(t *testing.T) {
	root := t.TempDir()
	opencodeDir := filepath.Join(t.TempDir(), "opencode")
	s := store.New(root, opencodeDir)
	require.NoError(t, s.Init())

	outside := filepath.Join(root, "outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))

	forged := filepath.Join(s.ProfilesDir(), "default", "..", "..", "outside")
	require.NoError(t, os.Symlink(forged, opencodeDir))

	err := s.Reset()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not managed by opm")
}
```

- [ ] **Step 2: Run the targeted store tests and confirm they fail**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./internal/store/... -run 'Test(Store_(GetProfile|DeleteProfile)|CopyProfile_SymlinkSourceRejected|Reset_RejectsEscapingManagedTarget)' -v
```

Expected:
- traversal-name tests fail because `GetProfile` and `DeleteProfile` currently do not validate names
- the symlink-source copy test fails because `CopyProfile` currently follows the source root
- the reset test fails because `Reset()` still uses a raw prefix check

- [ ] **Step 3: Implement the remaining store safety changes**

In `internal/store/store.go`, replace `GetProfile`, `CopyProfile`, `DeleteProfile`, and the start of `Reset()` with this code:

```go
func (s *Store) GetProfile(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}

	dir := s.ProfileDir(name)
	fi, err := os.Lstat(dir)
	if os.IsNotExist(err) || (err == nil && !fi.IsDir()) {
		return "", fmt.Errorf("context %q does not exist", name)
	}
	if err != nil {
		return "", fmt.Errorf("stat profile %q: %w", name, err)
	}
	return dir, nil
}

func (s *Store) CopyProfile(src, dst string) error {
	if err := ValidateName(src); err != nil {
		return err
	}
	if err := ValidateName(dst); err != nil {
		return err
	}

	srcDir := s.ProfileDir(src)
	dstDir := s.ProfileDir(dst)

	srcInfo, err := os.Lstat(srcDir)
	if os.IsNotExist(err) {
		return fmt.Errorf("context %q does not exist", src)
	}
	if err != nil {
		return fmt.Errorf("stat profile %q: %w", src, err)
	}
	if srcInfo.Mode()&os.ModeSymlink != 0 || !srcInfo.IsDir() {
		return fmt.Errorf("context %q is not a directory", src)
	}

	if _, err := os.Lstat(dstDir); err == nil {
		return fmt.Errorf("context %q already exists", dst)
	}

	return copyDir(srcDir, dstDir)
}

func (s *Store) DeleteProfile(name string, force bool) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	if !force {
		active, err := s.ActiveProfile()
		if err == nil && active == name {
			return fmt.Errorf("cannot remove active context %q — switch to another context first, or use --force", name)
		}
	}

	dir := s.ProfileDir(name)
	if _, err := os.Lstat(dir); os.IsNotExist(err) {
		return fmt.Errorf("context %q does not exist", name)
	}
	return os.RemoveAll(dir)
}

func (s *Store) Reset() error {
	state, err := s.managedLinkState()
	if err != nil {
		return fmt.Errorf("inspect %s: %w", s.opencodeDir, err)
	}
	if !state.Managed {
		return fmt.Errorf("%s is not managed by opm", s.opencodeDir)
	}
	if state.Dangling {
		return fmt.Errorf("%s points to a missing managed profile", s.opencodeDir)
	}

	profileDir := state.ResolvedTarget
	tmpDir := s.opencodeDir + ".opm-reset-tmp"

	_ = os.RemoveAll(tmpDir)

	if err := copyDir(profileDir, tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("copy profile: %w", err)
	}

	if err := os.Remove(s.opencodeDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return fmt.Errorf("remove symlink: %w", err)
	}

	if err := os.Rename(tmpDir, s.opencodeDir); err != nil {
		if rerr := symlink.SetAtomic(profileDir, s.opencodeDir); rerr != nil {
			return fmt.Errorf("install directory: %w; rollback also failed: %v — opencodeDir may be absent", err, rerr)
		}
		return fmt.Errorf("install directory: %w", err)
	}

	_ = os.Remove(s.currentFile())
	return nil
}
```

- [ ] **Step 4: Re-run the targeted store tests and confirm they pass**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./internal/store/... -run 'Test(Store_(GetProfile|DeleteProfile)|CopyProfile_SymlinkSourceRejected|Reset_RejectsEscapingManagedTarget)' -v
```

Expected: all five new tests pass.

- [ ] **Step 5: Run the full store package again**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./internal/store/... -v
```

Expected: the full store package stays green after the validation and reset changes.

- [ ] **Step 6: Commit the rest of the store hardening**

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && git add internal/store/store.go internal/store/store_test.go && git commit -m "fix(store): harden profile path and reset safety"
```

---

### Task 3: Make `SetAtomic` safe under concurrent calls

**Files:**
- Modify: `internal/symlink/symlink.go`
- Modify: `internal/symlink/symlink_test.go`

This task keeps the existing atomic swap design but removes the shared temp-name collision.

- [ ] **Step 1: Write the failing concurrency test**

Add this test to `internal/symlink/symlink_test.go` and add `fmt` plus `sync` to the imports:

```go
func TestSetAtomic_ConcurrentCallsShareNoTempName(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "opencode")

	const writers = 16
	targets := make([]string, 0, writers)
	for i := 0; i < writers; i++ {
		target := filepath.Join(dir, fmt.Sprintf("profile-%02d", i))
		require.NoError(t, os.Mkdir(target, 0o755))
		targets = append(targets, target)
	}

	start := make(chan struct{})
	errs := make(chan error, writers)
	var wg sync.WaitGroup

	for _, target := range targets {
		wg.Add(1)
		go func(target string) {
			defer wg.Done()
			<-start
			errs <- symlink.SetAtomic(target, link)
		}(target)
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.NoError(t, err)
	}

	got, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Contains(t, targets, got)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	for _, entry := range entries {
		assert.NotContains(t, entry.Name(), ".opm-tmp-opencode-", "temp symlink should be cleaned up")
	}
}
```

- [ ] **Step 2: Run the targeted symlink test and confirm it fails**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./internal/symlink/... -run TestSetAtomic_ConcurrentCallsShareNoTempName -count=25 -v
```

Expected: at least one iteration fails with a temp-link collision or rename error because `SetAtomic()` currently reuses the same `.opm-tmp-opencode` path.

- [ ] **Step 3: Replace the shared temp path with a unique temp name**

In `internal/symlink/symlink.go`, replace `SetAtomic()` with this implementation:

```go
func SetAtomic(target, linkPath string) error {
	dir := filepath.Dir(linkPath)
	tmpFile, err := os.CreateTemp(dir, ".opm-tmp-"+filepath.Base(linkPath)+"-*")
	if err != nil {
		return fmt.Errorf("create temp symlink path: %w", err)
	}
	tmpLink := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpLink)
		return fmt.Errorf("close temp symlink path: %w", err)
	}
	if err := os.Remove(tmpLink); err != nil {
		return fmt.Errorf("prepare temp symlink path: %w", err)
	}

	if err := os.Symlink(target, tmpLink); err != nil {
		return fmt.Errorf("create temp symlink: %w", err)
	}

	if err := os.Rename(tmpLink, linkPath); err != nil {
		_ = os.Remove(tmpLink)
		return fmt.Errorf("atomic swap: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Re-run the concurrency test and confirm it passes**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./internal/symlink/... -run TestSetAtomic_ConcurrentCallsShareNoTempName -count=25 -v
```

Expected: all goroutines succeed, the final link target is one of the valid profile directories, and no `.opm-tmp-opencode-*` entries remain.

- [ ] **Step 5: Run the full symlink package tests**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./internal/symlink/... -v
```

Expected: all existing `Inspect` and `SetAtomic` tests remain green.

- [ ] **Step 6: Commit the concurrent `SetAtomic` fix**

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && git add internal/symlink/symlink.go internal/symlink/symlink_test.go && git commit -m "fix(symlink): use unique temp names for atomic swaps"
```

---

### Task 4: Fix `opm show` to stop trusting broken symlinks

**Files:**
- Modify: `cmd/show.go`
- Modify: `cmd/cmd_test.go`

This task makes `show` consistent with the hardened store logic: healthy managed symlink wins, broken/missing symlink falls back to `current`, foreign symlink still errors.

- [ ] **Step 1: Replace the stale `show` tests with the new expected behavior**

In `cmd/cmd_test.go`, replace `TestShow_FallbackWarnsOnBrokenSymlink` with this version and add the second new test:

```go
func TestShow_FallbackWarnsOnBrokenSymlink(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	goneDir := h.store.ProfileDir("default") + "_gone"
	require.NoError(t, os.Remove(h.opencodeDir))
	require.NoError(t, os.Symlink(goneDir, h.opencodeDir))
	require.NoError(t, h.store.SetCurrent("default"))

	out, stderr, err := h.run("show")
	require.NoError(t, err)
	assert.Equal(t, "default\n", out)
	assert.Contains(t, stderr, "warning: symlink is broken or absent")
}

func TestShow_BrokenSymlinkWithoutCurrentErrors(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	root := filepath.Dir(filepath.Dir(h.store.ProfileDir("default")))
	require.NoError(t, os.Remove(filepath.Join(root, "current")))

	goneDir := h.store.ProfileDir("default") + "_gone"
	require.NoError(t, os.Remove(h.opencodeDir))
	require.NoError(t, os.Symlink(goneDir, h.opencodeDir))

	_, _, err := h.run("show")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no active profile")
}
```

- [ ] **Step 2: Run the targeted `show` tests and confirm they fail**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./cmd -run 'TestShow_(FallbackWarnsOnBrokenSymlink|BrokenSymlinkWithoutCurrentErrors)' -v
```

Expected:
- the first test fails because `show` still prints the dangling basename
- the second test fails because `show` currently never reaches the fallback error path for a dangling managed symlink

- [ ] **Step 3: Replace `runShow()` with the hardened fallback logic**

Replace `cmd/show.go` with this file:

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/tbcrawford/opm/internal/symlink"
)

var showCmd = &cobra.Command{
	Use:          "show",
	Short:        "Print the name of the currently active profile",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	s := newStore()
	managed, err := s.IsOpmManaged()
	if err != nil {
		return fmt.Errorf("cannot determine opm state: %w", err)
	}
	if !managed {
		st, err := symlink.Inspect(s.OpencodeDir())
		if err != nil {
			return fmt.Errorf("inspect active symlink: %w", err)
		}
		if st.Exists {
			return fmt.Errorf("~/.config/opencode is not managed by opm\n\n  Run 'opm init' to initialize")
		}
	}

	name, err := s.ActiveProfile()
	if err != nil {
		return fmt.Errorf("read active profile: %w", err)
	}
	if name != "" {
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), name)
		return nil
	}

	if cached, cerr := s.GetCurrent(); cerr == nil && cached != "" {
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "warning: symlink is broken or absent; reporting cached profile name")
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), cached)
		return nil
	}

	return fmt.Errorf("no active profile — run 'opm init' first")
}
```

- [ ] **Step 4: Re-run the targeted `show` tests and confirm they pass**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./cmd -run 'TestShow_(FallbackWarnsOnBrokenSymlink|BrokenSymlinkWithoutCurrentErrors)' -v
```

Expected: both `show` regression tests pass.

- [ ] **Step 5: Run the full command package tests**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./cmd -v
```

Expected: the updated `show` behavior does not break the rest of the command suite.

- [ ] **Step 6: Commit the `show` fix**

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && git add cmd/show.go cmd/cmd_test.go && git commit -m "fix(show): fall back cleanly when active symlink is broken"
```

---

### Task 5: Harden `opm init` against unsafe pre-existing paths and stale resume symlinks

**Files:**
- Modify: `cmd/init.go`
- Modify: `cmd/cmd_test.go`

This task fixes the destructive regular-file case and validates `.opm-new` before resuming interrupted initialization.

- [ ] **Step 1: Add the failing `init` regression tests**

Add these tests to `cmd/cmd_test.go`:

```go
func TestInit_RefusesRegularFileAtOpencodePath(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, os.WriteFile(h.opencodeDir, []byte("not a directory"), 0o644))

	_, _, err := h.run("init")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a directory or symlink")

	managed, merr := h.store.IsOpmManaged()
	require.NoError(t, merr)
	assert.False(t, managed)
}

func TestInit_RejectsStaleResumeSymlink(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.store.Init())
	require.NoError(t, os.MkdirAll(h.store.ProfileDir("default"), 0o755))

	tmpSym := h.opencodeDir + ".opm-new"
	require.NoError(t, os.Symlink(t.TempDir(), tmpSym))

	_, _, err := h.run("init")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "partial initialization detected")
}

func TestInit_ResumesMatchingTempSymlink(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.store.Init())
	require.NoError(t, os.MkdirAll(h.store.ProfileDir("default"), 0o755))

	tmpSym := h.opencodeDir + ".opm-new"
	require.NoError(t, os.Symlink(h.store.ProfileDir("default"), tmpSym))

	out, _, err := h.run("init")
	require.NoError(t, err)
	assert.Contains(t, out, "Initialized opm")

	managed, merr := h.store.IsOpmManaged()
	require.NoError(t, merr)
	assert.True(t, managed)

	current, cerr := h.store.GetCurrent()
	require.NoError(t, cerr)
	assert.Equal(t, "default", current)
}
```

- [ ] **Step 2: Run the targeted `init` tests and confirm they fail**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./cmd -run 'TestInit_(RefusesRegularFileAtOpencodePath|RejectsStaleResumeSymlink|ResumesMatchingTempSymlink)' -v
```

Expected:
- the regular-file test fails because `runInit()` still treats the file as a fresh-init case
- the stale resume test fails because `.opm-new` is currently trusted without validation

- [ ] **Step 3: Add the regular-file guard and validate `.opm-new` before resuming**

In `cmd/init.go`, add the regular-file guard immediately after `symlink.Inspect(opencodeDir)` and replace the interrupted-init branch with this version:

```go
	if st.Exists && !st.IsDir && !st.IsSymlink {
		return fmt.Errorf("~/.config/opencode exists but is not a directory or symlink\n\n  Back it up and remove it, then run 'opm init' again")
	}

	if _, statErr := os.Lstat(profileDir); statErr == nil {
		tmpSym := opencodeDir + ".opm-new"
		tmpState, err := symlink.Inspect(tmpSym)
		if err != nil {
			return fmt.Errorf("inspect %s: %w", tmpSym, err)
		}
		if tmpState.Exists {
			if !tmpState.IsSymlink || tmpState.Target != profileDir {
				return fmt.Errorf(
					"partial initialization detected: found unexpected %s\n\n"+
						"  To recover:\n"+
						"    rm -f %s\n"+
						"    rm -rf ~/.config/opm/profiles/%s\n"+
						"    opm init",
					tmpSym, tmpSym, profileName,
				)
			}
			if err := os.Rename(tmpSym, opencodeDir); err != nil {
				return fmt.Errorf("resume: atomic rename symlink: %w", err)
			}
			if err := s.SetCurrent(profileName); err != nil {
				return fmt.Errorf("set current: %w", err)
			}
			output.Success(cmd.OutOrStdout(), "Initialized opm", "Migrated ~/.config/opencode → profiles/"+profileName)
			return nil
		}

		managed, mErr := s.IsOpmManaged()
		if mErr == nil && managed {
			return fmt.Errorf("already initialized (active: %s)", profileName)
		}
		return fmt.Errorf(
			"partial initialization detected: profiles/%s exists but ~/.config/opencode is not managed by opm\n\n"+
				"  To recover:\n"+
				"    rm -rf ~/.config/opm/profiles/%s\n"+
				"    opm init",
			profileName, profileName,
		)
	}
```

- [ ] **Step 4: Re-run the targeted `init` tests and confirm they pass**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./cmd -run 'TestInit_(RefusesRegularFileAtOpencodePath|RejectsStaleResumeSymlink|ResumesMatchingTempSymlink)' -v
```

Expected: all three `init` regression tests pass.

- [ ] **Step 5: Run the full command package tests**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./cmd -v
```

Expected: the rest of the command package still passes with the new `init` guards.

- [ ] **Step 6: Commit the `init` safety fixes**

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && git add cmd/init.go cmd/cmd_test.go && git commit -m "fix(init): reject unsafe paths and validate resume symlinks"
```

---

### Task 6: Warn instead of failing when only the `current` cache update fails

**Files:**
- Modify: `cmd/root.go`
- Modify: `cmd/use.go`
- Modify: `cmd/remove.go`
- Modify: `cmd/rename.go`
- Modify: `cmd/init.go`
- Modify: `cmd/cmd_test.go`

This task keeps command exit status aligned with the real source of truth: if the symlink update succeeded, the command should succeed and emit a warning instead of failing because the cache file could not be written.

- [ ] **Step 1: Add a small test helper and the four failing warning tests**

In `cmd/cmd_test.go`, add these helper methods next to `useStoreFactory()` and add the four tests below them:

```go
func (h *cmdHarness) currentPath() string {
	return filepath.Join(filepath.Dir(filepath.Dir(h.store.ProfileDir("default"))), "current")
}

func (h *cmdHarness) forceCurrentWriteFailure(t *testing.T) {
	t.Helper()
	_ = os.Remove(h.currentPath())
	require.NoError(t, os.Mkdir(h.currentPath(), 0o755))
}

func TestUse_CurrentWriteFailureWarnsButSucceeds(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)

	h.forceCurrentWriteFailure(t)
	out, stderr, err := h.run("use", "work")
	require.NoError(t, err)
	assert.Contains(t, out, "work")
	assert.Contains(t, stderr, "warning: active profile is")

	active, aerr := h.store.ActiveProfile()
	require.NoError(t, aerr)
	assert.Equal(t, "work", active)
}

func TestRename_ActiveCurrentWriteFailureWarnsButSucceeds(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)

	h.forceCurrentWriteFailure(t)
	out, stderr, err := h.run("rename", "default", "primary")
	require.NoError(t, err)
	assert.Contains(t, out, "primary")
	assert.Contains(t, stderr, "warning: active profile is")

	active, aerr := h.store.ActiveProfile()
	require.NoError(t, aerr)
	assert.Equal(t, "primary", active)
}

func TestRemove_ForceCurrentWriteFailureWarnsButSucceeds(t *testing.T) {
	h := newHarness(t)
	h.mustInit(t)
	_, _, err := h.run("create", "work")
	require.NoError(t, err)

	h.forceCurrentWriteFailure(t)
	out, stderr, err := h.run("remove", "--force", "default")
	require.NoError(t, err)
	assert.Contains(t, out, "Removed profile")
	assert.Contains(t, stderr, "warning: active profile is")

	active, aerr := h.store.ActiveProfile()
	require.NoError(t, aerr)
	assert.Equal(t, "work", active)
}

func TestInit_CurrentWriteFailureWarnsButSucceeds(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, os.Mkdir(h.currentPath(), 0o755))

	out, stderr, err := h.run("init")
	require.NoError(t, err)
	assert.Contains(t, out, "Initialized opm")
	assert.Contains(t, stderr, "warning: active profile is")

	managed, merr := h.store.IsOpmManaged()
	require.NoError(t, merr)
	assert.True(t, managed)
}
```

- [ ] **Step 2: Run the targeted warning tests and confirm they fail**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./cmd -run 'Test(Use_CurrentWriteFailureWarnsButSucceeds|Rename_ActiveCurrentWriteFailureWarnsButSucceeds|Remove_ForceCurrentWriteFailureWarnsButSucceeds|Init_CurrentWriteFailureWarnsButSucceeds)' -v
```

Expected: all four tests fail because the commands currently return `update current` or `set current` errors.

- [ ] **Step 3: Add the shared warning helper and switch each command to use it**

In `cmd/root.go`, add this helper near `managedGuard`:

```go
func warnCurrentCacheSync(cmd *cobra.Command, name string, err error) {
	_, _ = fmt.Fprintf(
		cmd.ErrOrStderr(),
		"warning: active profile is %q, but failed to update the current cache: %v\n",
		name,
		err,
	)
}
```

Then update the `SetCurrent` call sites in `cmd/use.go`, `cmd/remove.go`, `cmd/rename.go`, and `cmd/init.go` like this:

```go
if err := s.SetCurrent(name); err != nil {
	warnCurrentCacheSync(cmd, name, err)
}
```

```go
if err := s.SetCurrent(switchTarget); err != nil {
	warnCurrentCacheSync(cmd, switchTarget, err)
}
```

```go
if err := s.SetCurrent(newName); err != nil {
	warnCurrentCacheSync(cmd, newName, err)
}
```

```go
if err := s.SetCurrent(profileName); err != nil {
	warnCurrentCacheSync(cmd, profileName, err)
}
```

Do not change the symlink update behavior. Only downgrade the cache write failure to a warning after the real state change has already succeeded.

- [ ] **Step 4: Re-run the targeted warning tests and confirm they pass**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./cmd -run 'Test(Use_CurrentWriteFailureWarnsButSucceeds|Rename_ActiveCurrentWriteFailureWarnsButSucceeds|Remove_ForceCurrentWriteFailureWarnsButSucceeds|Init_CurrentWriteFailureWarnsButSucceeds)' -v
```

Expected: all four tests pass, stderr contains the warning, and the active symlink still points to the new profile.

- [ ] **Step 5: Run the full command package tests**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./cmd -v
```

Expected: the whole command suite remains green after the warning downgrade.

- [ ] **Step 6: Commit the cache-warning behavior**

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && git add cmd/root.go cmd/use.go cmd/remove.go cmd/rename.go cmd/init.go cmd/cmd_test.go && git commit -m "fix(cmd): warn when current cache sync fails after success"
```

---

### Task 7: Expose the `completion` command in custom root help

**Files:**
- Modify: `cmd/help.go`
- Modify: `cmd/cmd_test.go`

This task closes the discoverability gap between the README and `opm --help`.

- [ ] **Step 1: Add the failing root help test**

Add this test to `cmd/cmd_test.go`:

```go
func TestRootHelp_IncludesCompletionCommand(t *testing.T) {
	h := newHarness(t)
	out, _, err := h.run("--help")
	require.NoError(t, err)
	assert.Contains(t, out, "completion")
}
```

- [ ] **Step 2: Run the targeted help test and confirm it fails**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./cmd -run TestRootHelp_IncludesCompletionCommand -v
```

Expected: the test fails because the current hard-coded help sections omit `completion`.

- [ ] **Step 3: Add `completion` to the custom root help**

In `cmd/help.go`, update the `Scripting` section entries to this:

```go
		{
			label: "Scripting",
			entries: []entry{
				{"path", "Print the absolute path to a profile directory", ""},
				{"completion", "Generate shell completion scripts", ""},
			},
		},
```

- [ ] **Step 4: Re-run the targeted help test and confirm it passes**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && go test ./cmd -run TestRootHelp_IncludesCompletionCommand -v
```

Expected: the root help output now includes `completion`.

- [ ] **Step 5: Smoke test the real CLI help output**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && just run -- --help
```

Expected: the root help page renders normally and the `Scripting` section lists both `path` and `completion`.

- [ ] **Step 6: Commit the help update**

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && git add cmd/help.go cmd/cmd_test.go && git commit -m "fix(help): include completion in root help"
```

---

### Task 8: Run full verification before calling the branch ready

**Files:**
- No code changes

- [ ] **Step 1: Run the full project verification command**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && just verify
```

Expected:
- `go vet ./...` passes
- `golangci-lint run` reports `0 issues`
- `go test ./...` passes for `cmd`, `internal/store`, `internal/symlink`, and the rest of the repo

- [ ] **Step 2: Inspect the final diff before handing off**

Run:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && git status --short && git diff --stat
```

Expected: only the planned files are modified, and the diff matches the tasks above.

- [ ] **Step 3: Commit the verification checkpoint if needed**

If any task above was intentionally left uncommitted, commit the remaining changes now with:

```bash
cd /Users/tylercrawford/dev/playground/opm/.worktrees/filesystem-safety-cli-hardening && git add cmd/help.go cmd/init.go cmd/remove.go cmd/rename.go cmd/root.go cmd/show.go cmd/use.go cmd/cmd_test.go internal/store/store.go internal/store/store_test.go internal/symlink/symlink.go internal/symlink/symlink_test.go && git commit -m "fix: harden filesystem safety and CLI edge cases"
```

Expected: `git status --short` is clean after the final commit.
