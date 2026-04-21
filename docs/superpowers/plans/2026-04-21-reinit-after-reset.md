# Reinit After Reset Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `opm init` work correctly when run after `opm reset`, reinstating symlink management against existing profiles instead of erroring with "partial initialization detected".

**Architecture:** Modify `checkForPartialInit` in `internal/store/init.go`. When the target profile directory already exists and opm is not managed (no symlink at `~/.config/opencode`), and `opencodeDir` itself exists as a plain directory, treat this as a post-reset reinit: remove the plain `opencodeDir` and reinstall the symlink. When `opencodeDir` does not exist (truly orphaned profile dir with no config dir at all), fall through to the existing partial-init error. The `--as <name>` flag still controls which profile to activate; if the named profile does not exist post-reset, normal migration runs as today.

**Tech Stack:** Go, `github.com/stretchr/testify`, `just` for test runner.

---

### Task 1: Write failing tests for the post-reset reinit scenario

**Files:**
- Modify: `internal/store/init_test.go`

These tests cover the new behavior. They will fail until Task 2 implements the fix.

- [ ] **Step 1: Add three tests to `internal/store/init_test.go`**

Open `internal/store/init_test.go` and append these three test functions before the final closing brace of the file:

```go
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
```

- [ ] **Step 2: Run the new tests to verify they fail**

```
go test ./internal/store/... -run "TestStore_Initialize_ReinitAfterReset" -v
```

Expected: all three tests FAIL. `TestStore_Initialize_ReinitAfterReset` and `TestStore_Initialize_ReinitAfterReset_MultipleProfiles` should fail with `"partial initialization detected"`. `TestStore_Initialize_ReinitAfterReset_NewProfileName` may pass already (it uses a profile name that doesn't exist, which falls through to migration). Confirm the first two fail before continuing.

---

### Task 2: Implement the post-reset reinit path

**Files:**
- Modify: `internal/store/init.go`

Change `checkForPartialInit` so that when the profile directory exists, opm is not managed, and `opencodeDir` also exists as a plain directory, it removes the plain directory and reinstalls the symlink.

- [ ] **Step 1: Replace the "partial init detected" branch in `checkForPartialInit`**

In `internal/store/init.go`, find this block (lines 90–103):

```go
	if !tmpExists {
		if _, statErr := os.Lstat(profileDir); statErr == nil {
			managed, mErr := s.IsOpmManaged()
			if mErr == nil && managed {
				return InitResult{}, false, s.alreadyInitializedError()
			}
			return InitResult{}, false, fmt.Errorf(
				"partial initialization detected: %s exists but %s is not managed by opm\n\n"+
					"  To recover:\n"+
					"    rm -rf %s\n"+
					"    opm init",
				s.displayPath(profileDir), s.displayPath(s.OpencodeDir()), s.displayPath(profileDir),
			)
		}
		return InitResult{}, false, nil
	}
```

Replace it with:

```go
	if !tmpExists {
		if _, statErr := os.Lstat(profileDir); statErr == nil {
			managed, mErr := s.IsOpmManaged()
			if mErr == nil && managed {
				return InitResult{}, false, s.alreadyInitializedError()
			}
			// opencodeDir exists as a plain directory: this is the post-reset state
			// (opm reset copied the active profile back to opencodeDir and left profiles
			// intact). Remove the plain directory and reinstate the symlink.
			if opencodeExists {
				if err := os.RemoveAll(s.OpencodeDir()); err != nil {
					return InitResult{}, false, fmt.Errorf("remove plain opencode dir: %w", err)
				}
				if err := symlink.SetAtomic(profileDir, s.OpencodeDir()); err != nil {
					return InitResult{}, false, fmt.Errorf("reinstate symlink: %w", err)
				}
				return InitResult{
					ProfileDir:      profileDir,
					Migrated:        false,
					CurrentCacheErr: s.SetCurrent(profileName),
				}, true, nil
			}
			// opencodeDir does not exist but the profile dir does — genuinely unexpected
			// partial state that requires manual recovery.
			return InitResult{}, false, fmt.Errorf(
				"partial initialization detected: %s exists but %s is not managed by opm\n\n"+
					"  To recover:\n"+
					"    rm -rf %s\n"+
					"    opm init",
				s.displayPath(profileDir), s.displayPath(s.OpencodeDir()), s.displayPath(profileDir),
			)
		}
		return InitResult{}, false, nil
	}
```

- [ ] **Step 2: Run the three new tests to verify they pass**

```
go test ./internal/store/... -run "TestStore_Initialize_ReinitAfterReset" -v
```

Expected: all three tests PASS.

- [ ] **Step 3: Run the full test suite to verify no regressions**

```
just test
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/store/init.go internal/store/init_test.go
git commit -m "feat(init): reinitialize against existing profiles after opm reset"
```

---

### Task 3: Update the cmd-level init output for the reinit case

**Files:**
- Modify: `cmd/init.go`

The `runInit` function currently only checks `result.Migrated` to choose its success message. The new reinit path sets `Migrated: false` (the config wasn't migrated — the symlink was reinstated). Add a distinct message so users know which scenario ran.

- [ ] **Step 1: Add `Reinstated bool` to `InitResult`**

In `internal/store/init.go`, update the struct:

```go
// InitResult captures the outcome of a successful store initialization.
type InitResult struct {
	ProfileDir      string
	Migrated        bool
	Reinstated      bool // true when an existing profile was reconnected after opm reset
	CurrentCacheErr error
}
```

And update the reinstate return in `checkForPartialInit` (the block added in Task 2) to set `Reinstated: true`:

```go
			return InitResult{
				ProfileDir:      profileDir,
				Migrated:        false,
				Reinstated:      true,
				CurrentCacheErr: s.SetCurrent(profileName),
			}, true, nil
```

- [ ] **Step 2: Add the reinstated message branch in `cmd/init.go`**

In `cmd/init.go`, update `runInit` to handle the new case. Replace the existing result-handling block:

```go
	if result.Migrated {
		output.Success(cmd.OutOrStdout(), "Initialized opm", "Migrated ~/.config/opencode → profiles/"+profileName)
		return nil
	}

	output.Success(cmd.OutOrStdout(), "Initialized opm",
		"Created "+profileName+" profile at "+output.ShortenHome(result.ProfileDir)+"/",
	)
	return nil
```

With:

```go
	switch {
	case result.Migrated:
		output.Success(cmd.OutOrStdout(), "Initialized opm", "Migrated ~/.config/opencode → profiles/"+profileName)
	case result.Reinstated:
		output.Success(cmd.OutOrStdout(), "Reinitialized opm", "Reconnected to existing profile "+profileName+" at "+output.ShortenHome(result.ProfileDir)+"/")
	default:
		output.Success(cmd.OutOrStdout(), "Initialized opm",
			"Created "+profileName+" profile at "+output.ShortenHome(result.ProfileDir)+"/",
		)
	}
	return nil
```

- [ ] **Step 3: Run the full test suite**

```
just test
```

Expected: all tests pass.

- [ ] **Step 4: Run vet and lint**

```
just check
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/store/init.go cmd/init.go
git commit -m "feat(init): show distinct message when reconnecting after opm reset"
```

---

### Task 4: Add a cmd-level integration test for the reinit output

**Files:**
- Modify: `cmd/cmd_test.go`

Add a test confirming the CLI output message is correct for the reinit path.

- [ ] **Step 1: Confirm the test helper pattern**

The harness is `newHarness(t *testing.T) *cmdHarness`. Run commands via `h.run(args...)` which returns `(stdout, stderr string, err error)`. Store is `h.store`, opencodeDir is `h.opencodeDir`.

- [ ] **Step 2: Add the cmd integration test**

Append this test to `cmd/cmd_test.go`:

```go
func TestCmd_Init_ReinitAfterReset(t *testing.T) {
	h := newHarness(t)

	// First init.
	stdout, _, err := h.run("init")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Initialized opm")

	// Simulate reset: remove symlink, restore plain dir, remove current file.
	profileDir := h.store.ProfileDir("default")
	require.NoError(t, os.Remove(h.opencodeDir))
	require.NoError(t, os.MkdirAll(h.opencodeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "opencode.json"), []byte(`{}`), 0o644))
	_ = os.Remove(filepath.Join(h.store.OpmDir(), "current"))

	// Re-init.
	stdout, _, err = h.run("init")
	require.NoError(t, err)
	assert.Contains(t, stdout, "Reinitialized opm")
	assert.Contains(t, stdout, "default")
}
```

- [ ] **Step 3: Run just the new test**

```
go test ./cmd/... -run "TestCmd_Init_ReinitAfterReset" -v
```

Expected: PASS. If the helper names differ from the actual helpers in `cmd_test.go` (discovered in Step 1), adjust accordingly.

- [ ] **Step 4: Run full test suite + check**

```
just verify
```

Expected: all tests pass, lint clean.

- [ ] **Step 5: Commit**

```bash
git add cmd/cmd_test.go
git commit -m "test(cmd): verify reinit-after-reset CLI output message"
```
