# Reset Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `opm reset` which restores `~/.config/opencode` to a plain directory (copying the active profile's contents back), removes the managing symlink, and leaves `~/.config/opm/` and all profiles intact with a message telling the user how to clean up manually.

**Architecture:** Two tasks — first add a `Reset` method to `internal/store` (with tests), then add `cmd/reset.go` wired to `rootCmd`. Reset copies the active profile directory to a temp location, removes the symlink, moves the copy into place as `~/.config/opencode`, and removes `~/.config/opm/current`. All profile data in `~/.config/opm/profiles/` is left untouched.

**Tech Stack:** Go stdlib (`os`, `path/filepath`, `io/fs`), cobra, existing `internal/store`, `internal/paths`, `internal/output` packages.

---

### Task 1: Add `Reset` to `internal/store` with tests

**Files:**
- Modify: `internal/store/store.go` (add `Reset` method)
- Modify: `internal/store/store_test.go` (add tests for `Reset`)

The `Reset` method must:
1. Verify `opencodeDir` is an opm-managed symlink (call `symlink.Inspect`). Return an error if it is not.
2. Resolve the active profile directory from the symlink target.
3. Copy the active profile directory to `opencodeDir + ".opm-reset-tmp"` (a temp directory alongside the symlink).
4. Remove the symlink at `opencodeDir`.
5. Rename the temp directory to `opencodeDir`.
6. Remove the `current` file at `s.currentFile()`.

Step 3 uses a recursive directory copy. Implement a private `copyDir(src, dst string) error` function in `store.go`.

`copyDir` must:
- Create `dst` with the same permissions as `src`
- Walk `src` with `os.ReadDir` recursively
- For each regular file: open src file, create dst file, copy bytes with `io.Copy`, close both
- For each directory entry: recurse
- Symlinks inside the profile directory: copy as symlinks (preserve `os.Readlink` target via `os.Symlink`)

- [ ] **Step 1: Write failing tests in `internal/store/store_test.go`**

Add these test functions. The existing test file uses `t.TempDir()` for isolation — follow that pattern exactly.

```go
func TestReset_NotManaged(t *testing.T) {
	root := t.TempDir()
	opencode := t.TempDir() // plain directory, not a symlink
	s := New(root, opencode)

	err := s.Reset()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not managed by opm")
}

func TestReset_RestoresDirectory(t *testing.T) {
	root := t.TempDir()
	opencodeDir := filepath.Join(t.TempDir(), "opencode")

	// Set up a profile with a file in it.
	s := New(root, opencodeDir)
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

func TestReset_PreservesProfiles(t *testing.T) {
	root := t.TempDir()
	opencodeDir := filepath.Join(t.TempDir(), "opencode")

	s := New(root, opencodeDir)
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
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./internal/store/... -run "TestReset" -v
```

Expected: FAIL — `s.Reset undefined`

- [ ] **Step 3: Add `copyDir` and `Reset` to `internal/store/store.go`**

Add at the end of `store.go`:

```go
// Reset removes opm's management of opencodeDir by:
//  1. Verifying opencodeDir is an opm-managed symlink.
//  2. Copying the active profile directory to opencodeDir as a real directory.
//  3. Removing the current file.
//
// All profile data under the store root is left intact.
func (s *Store) Reset() error {
	st, err := symlink.Inspect(s.opencodeDir)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", s.opencodeDir, err)
	}
	if !st.IsSymlink || !strings.HasPrefix(st.Target, s.profilesDir()) {
		return fmt.Errorf("%s is not managed by opm", s.opencodeDir)
	}

	profileDir := st.Target
	tmpDir := s.opencodeDir + ".opm-reset-tmp"

	// Clean up any leftover tmp from a prior crash.
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
		return fmt.Errorf("install directory: %w", err)
	}

	// Best-effort: remove the current file. Non-fatal if absent.
	_ = os.Remove(s.currentFile())
	return nil
}

// copyDir recursively copies src into dst.
// Regular files are copied byte-for-byte. Symlinks are re-created.
// dst must not exist before calling.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.Type()&os.ModeSymlink != 0 {
			target, err := os.Readlink(srcPath)
			if err != nil {
				return err
			}
			if err := os.Symlink(target, dstPath); err != nil {
				return err
			}
			continue
		}

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

// copyFile copies a single regular file from src to dst, preserving permissions.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
```

Also add `"io"` to the import block in `store.go`.

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./internal/store/... -run "TestReset" -v
```

Expected: all three `TestReset_*` tests PASS.

- [ ] **Step 5: Run full store test suite to confirm no regressions**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./internal/store/... -v
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/tylercrawford/dev/playground/opm
git add internal/store/store.go internal/store/store_test.go
git commit -m "feat(store): add Reset method and copyDir helper"
```

---

### Task 2: Add `cmd/reset.go` command

**Files:**
- Create: `cmd/reset.go`

The command must:
- `Use: "reset"`
- `Short: "Restore ~/.config/opencode to a plain directory"`
- `Args: cobra.NoArgs`
- `PersistentPreRunE: managedGuard`
- `SilenceUsage: true`
- On success: call `output.Success` with a two-line message:
  - Primary: `"Reset complete — ~/.config/opencode is now a plain directory"`
  - Detail: `"Profiles left intact at " + output.ShortenHome(paths.ProfilesDir()) + "  •  remove with: rm -rf " + output.ShortenHome(paths.OpmDir())`

- [ ] **Step 1: Create `cmd/reset.go`**

```go
package cmd

import (
	"github.com/tbcrawford/opm/internal/output"
	"github.com/tbcrawford/opm/internal/paths"
	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:               "reset",
	Short:             "Restore ~/.config/opencode to a plain directory",
	Long:              "Removes opm's symlink and copies the active profile back to ~/.config/opencode as a real directory.\nAll profiles in ~/.config/opm/profiles/ are left intact.",
	Args:              cobra.NoArgs,
	PersistentPreRunE: managedGuard,
	SilenceUsage:      true,
	RunE:              runReset,
}

func init() {
	rootCmd.AddCommand(resetCmd)
}

func runReset(cmd *cobra.Command, args []string) error {
	s := newStore()
	if err := s.Reset(); err != nil {
		return err
	}
	output.Success(cmd.OutOrStdout(),
		"Reset complete — ~/.config/opencode is now a plain directory",
		"Profiles left intact at "+output.ShortenHome(paths.ProfilesDir())+"  •  remove with: rm -rf "+output.ShortenHome(paths.OpmDir()),
	)
	return nil
}
```

- [ ] **Step 2: Build**

```bash
cd /Users/tylercrawford/dev/playground/opm && go build ./...
```

Expected: clean build.

- [ ] **Step 3: Verify command appears in help**

```bash
cd /Users/tylercrawford/dev/playground/opm && go run . --help
```

Expected: `reset` appears in the Available Commands list.

- [ ] **Step 4: Verify help text**

```bash
cd /Users/tylercrawford/dev/playground/opm && go run . reset --help
```

Expected output includes:
```
Removes opm's symlink and copies the active profile back to ~/.config/opencode as a real directory.
All profiles in ~/.config/opm/profiles/ are left intact.
```

- [ ] **Step 5: Run full test suite**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/tylercrawford/dev/playground/opm
git add cmd/reset.go
git commit -m "feat(cmd): add reset command"
```
