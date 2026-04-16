# New Commands and Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `opm copy`, `opm path`, update `opm use` to show the from→to transition, add `--long` flag to `opm list`, and add `--from` flag to `opm create`.

**Architecture:** Task 1 adds `store.CopyProfile` (shared by both `copy` and `create --from`). Tasks 2–6 are independent command-layer changes that build on it. Each task is one file or one targeted edit. The `copyDir` helper already exists in `internal/store/store.go` from the `Reset` implementation — `CopyProfile` reuses it directly.

**Tech Stack:** Go, cobra, existing `internal/store`, `internal/output`, `internal/paths` packages.

---

### Task 1: Add `CopyProfile` to `internal/store` with tests

**Files:**
- Modify: `internal/store/store.go` (add `CopyProfile` method)
- Modify: `internal/store/store_test.go` (add tests)

`CopyProfile(src, dst string) error` must:
1. Validate `dst` name with `ValidateName`
2. Verify `src` directory exists (return `fmt.Errorf("context %q does not exist", src)` if not)
3. Verify `dst` directory does not exist (return `fmt.Errorf("context %q already exists", dst)` if it does)
4. Call `copyDir(srcDir, dstDir)` — already defined in `store.go`

- [ ] **Step 1: Write failing tests in `internal/store/store_test.go`**

```go
func TestCopyProfile_Basic(t *testing.T) {
	root := t.TempDir()
	s := New(root, t.TempDir())
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
	s := New(root, t.TempDir())
	require.NoError(t, s.Init())

	err := s.CopyProfile("nonexistent", "dst")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"nonexistent" does not exist`)
}

func TestCopyProfile_DstExists(t *testing.T) {
	root := t.TempDir()
	s := New(root, t.TempDir())
	require.NoError(t, s.Init())
	require.NoError(t, s.CreateProfile("src"))
	require.NoError(t, s.CreateProfile("dst"))

	err := s.CopyProfile("src", "dst")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `"dst" already exists`)
}

func TestCopyProfile_InvalidDstName(t *testing.T) {
	root := t.TempDir()
	s := New(root, t.TempDir())
	require.NoError(t, s.Init())
	require.NoError(t, s.CreateProfile("src"))

	err := s.CopyProfile("src", "bad name!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid profile name")
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./internal/store/... -run "TestCopyProfile" -v
```

Expected: FAIL — `s.CopyProfile undefined`

- [ ] **Step 3: Add `CopyProfile` to `internal/store/store.go`**

Add after `CreateProfile`:

```go
// CopyProfile creates a new profile by copying an existing one.
// Validates dst name, checks src exists and dst does not, then copies the directory tree.
func (s *Store) CopyProfile(src, dst string) error {
	if err := ValidateName(dst); err != nil {
		return err
	}
	srcDir := s.ProfileDir(src)
	dstDir := s.ProfileDir(dst)

	if _, err := os.Lstat(srcDir); os.IsNotExist(err) {
		return fmt.Errorf("context %q does not exist", src)
	}
	if _, err := os.Lstat(dstDir); err == nil {
		return fmt.Errorf("context %q already exists", dst)
	}

	return copyDir(srcDir, dstDir)
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./internal/store/... -run "TestCopyProfile" -v
```

Expected: all four `TestCopyProfile_*` tests PASS.

- [ ] **Step 5: Run full store test suite**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./internal/store/... -v
```

Expected: all tests pass.

- [ ] **Step 6: Commit**

```bash
cd /Users/tylercrawford/dev/playground/opm && git add internal/store/store.go internal/store/store_test.go && git commit -m "feat(store): add CopyProfile method"
```

---

### Task 2: Add `opm copy <src> <dst>` command

**Files:**
- Create: `cmd/copy.go`

- [ ] **Step 1: Create `cmd/copy.go`**

```go
package cmd

import (
	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/spf13/cobra"
)

var copyCmd = &cobra.Command{
	Use:               "copy <src> <dst>",
	Short:             "Copy a profile to a new name",
	Args:              cobra.ExactArgs(2),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runCopy,
}

func init() {
	rootCmd.AddCommand(copyCmd)
}

func runCopy(cmd *cobra.Command, args []string) error {
	src, dst := args[0], args[1]
	s := newStore()
	if err := s.CopyProfile(src, dst); err != nil {
		return err
	}
	dstDir := paths.ProfileDir(dst)
	output.Success(cmd.OutOrStdout(),
		"Copied "+output.ProfileName(src)+" → "+output.ProfileName(dst),
		output.ShortenHome(dstDir)+"/",
	)
	return nil
}
```

- [ ] **Step 2: Build and verify**

```bash
cd /Users/tylercrawford/dev/playground/opm && go build ./... && go run . copy --help
```

Expected: help shows `opm copy <src> <dst>`.

- [ ] **Step 3: Smoke test**

```bash
cd /Users/tylercrawford/dev/playground/opm && go build -o opm . && ./opm copy default default-backup && ./opm list; rm opm
```

Expected: success message + `default-backup` appears in list.

- [ ] **Step 4: Run full test suite**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./...
```

Expected: all packages pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/tylercrawford/dev/playground/opm && git add cmd/copy.go && git commit -m "feat(cmd): add copy command"
```

---

### Task 3: Add `opm path <name>` command

**Files:**
- Create: `cmd/path.go`

The command prints the absolute filesystem path to the profile directory — no `~` shortening, since it is intended for scripting (`cd $(opm path work)`).

- [ ] **Step 1: Create `cmd/path.go`**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var pathCmd = &cobra.Command{
	Use:               "path <name>",
	Short:             "Print the filesystem path to a profile directory",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runPath,
}

func init() {
	rootCmd.AddCommand(pathCmd)
}

func runPath(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := newStore()
	profilePath, err := s.GetProfile(name)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), profilePath)
	return nil
}
```

- [ ] **Step 2: Build and verify**

```bash
cd /Users/tylercrawford/dev/playground/opm && go build -o opm . && ./opm path default; rm opm
```

Expected: prints absolute path like `/Users/<you>/.config/opm/profiles/default` with no `~`.

- [ ] **Step 3: Run full test suite**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./...
```

Expected: all packages pass.

- [ ] **Step 4: Commit**

```bash
cd /Users/tylercrawford/dev/playground/opm && git add cmd/path.go && git commit -m "feat(cmd): add path command"
```

---

### Task 4: Update `opm use` to show from→to transition

**Files:**
- Modify: `cmd/use.go`

Change the success message from `"Switched to <name>"` to `"<from> → <to>"` when switching between different profiles. If the active profile can't be determined (e.g. first switch after init), fall back to `"Switched to <name>"`.

- [ ] **Step 1: Update `cmd/use.go`**

Replace the entire file with:

```go
package cmd

import (
	"fmt"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/opm-cli/opm/internal/symlink"
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:               "use <name>",
	Short:             "Switch to a profile",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runUse,
}

func init() {
	rootCmd.AddCommand(useCmd)
}

func runUse(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := newStore()

	profileDir, err := s.GetProfile(name)
	if err != nil {
		return err
	}

	// Capture the current active profile before switching.
	fromName, _ := s.ActiveProfile()

	opencodeDir := paths.OpencodeConfigDir()
	if err := symlink.SetAtomic(profileDir, opencodeDir); err != nil {
		return fmt.Errorf("switch profile: %w", err)
	}

	if err := s.SetCurrent(name); err != nil {
		return fmt.Errorf("update current: %w", err)
	}

	var msg string
	if fromName != "" && fromName != name {
		msg = output.ProfileName(fromName) + " → " + output.ProfileName(name)
	} else {
		msg = "Switched to " + output.ProfileName(name)
	}
	output.Success(cmd.OutOrStdout(), msg,
		output.ShortenHome(opencodeDir)+" → profiles/"+name)
	return nil
}
```

- [ ] **Step 2: Build and smoke test**

```bash
cd /Users/tylercrawford/dev/playground/opm && go build -o opm . && ./opm use default && ./opm use superpowers; rm opm
```

Expected:
```
✓ Switched to default
  ~/.config/opencode → profiles/default
✓ default → superpowers
  ~/.config/opencode → profiles/superpowers
```

- [ ] **Step 3: Run full test suite**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./...
```

Expected: all packages pass.

- [ ] **Step 4: Commit**

```bash
cd /Users/tylercrawford/dev/playground/opm && git add cmd/use.go && git commit -m "feat(use): show from→to transition in success message"
```

---

### Task 5: Add `--long` / `-l` flag to `opm list`

**Files:**
- Modify: `cmd/list.go`
- Modify: `internal/output/output.go` (add `ProfileTableLong`)
- Modify: `internal/output/output_test.go` (add `TestProfileTableLong`)

`--long` shows the profile path aligned in a second column:

```
● superpowers    ~/.config/opm/profiles/superpowers
○ default        ~/.config/opm/profiles/default
```

Uses `text/tabwriter` for alignment. Path is `~`-shortened.

- [ ] **Step 1: Write failing test in `internal/output/output_test.go`**

```go
func TestProfileTableLong(t *testing.T) {
	profiles := []store.Profile{
		{Name: "default", Path: "/home/user/.config/opm/profiles/default", Active: false},
		{Name: "work", Path: "/home/user/.config/opm/profiles/work", Active: true},
	}
	var buf bytes.Buffer
	ProfileTableLong(&buf, profiles)
	out := buf.String()
	assert.Contains(t, out, "default")
	assert.Contains(t, out, "/home/user/.config/opm/profiles/default")
	assert.Contains(t, out, "work")
	assert.Contains(t, out, "/home/user/.config/opm/profiles/work")
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./internal/output/... -run TestProfileTableLong -v
```

Expected: FAIL — `ProfileTableLong undefined`

- [ ] **Step 3: Add `ProfileTableLong` to `internal/output/output.go`**

Add the import `"text/tabwriter"` if not already present (it is — `tabwriter` is already imported). Add after `ProfileTable`:

```go
// ProfileTableLong writes the list with an extra path column, tab-aligned.
func ProfileTableLong(w io.Writer, profiles []store.Profile) {
	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', 0)
	for _, p := range profiles {
		switch {
		case p.Dangling:
			fmt.Fprintf(tw, "%s %s\t%s\n", red.Sprint("✗"), red.Sprint(p.Name), dim.Sprint("(missing) "+p.Path))
		case p.Active:
			fmt.Fprintf(tw, "%s %s\t%s\n", green.Sprint("●"), blue.Sprint(p.Name), dim.Sprint(ShortenHome(p.Path)))
		default:
			fmt.Fprintf(tw, "%s\t%s\n", dim.Sprintf("○ %s", p.Name), dim.Sprint(ShortenHome(p.Path)))
		}
	}
	_ = tw.Flush()
}
```

- [ ] **Step 4: Run test to confirm it passes**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./internal/output/... -run TestProfileTableLong -v
```

Expected: PASS.

- [ ] **Step 5: Update `cmd/list.go`**

Replace the entire file with:

```go
package cmd

import (
	"fmt"

	"github.com/opm-cli/opm/internal/output"
	"github.com/spf13/cobra"
)

var listLong bool

var listCmd = &cobra.Command{
	Use:               "list",
	Aliases:           []string{"ls"},
	Short:             "List all profiles",
	Args:              cobra.NoArgs,
	PersistentPreRunE: managedGuard,
	SilenceUsage:      true,
	RunE:              runList,
}

func init() {
	listCmd.Flags().BoolVarP(&listLong, "long", "l", false, "Show profile paths")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	s := newStore()
	profiles, err := s.ListProfiles()
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No profiles found. Run 'opm create <name>' to create one.")
		return nil
	}

	if listLong {
		output.ProfileTableLong(cmd.OutOrStdout(), profiles)
	} else {
		output.ProfileTable(cmd.OutOrStdout(), profiles)
	}
	return nil
}
```

- [ ] **Step 6: Build and smoke test**

```bash
cd /Users/tylercrawford/dev/playground/opm && go build -o opm . && ./opm list && echo "---" && ./opm list --long && echo "---" && ./opm ls -l; rm opm
```

Expected: short list unchanged, long list adds path column, alias works with `-l`.

- [ ] **Step 7: Run full test suite**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./...
```

Expected: all packages pass.

- [ ] **Step 8: Commit**

```bash
cd /Users/tylercrawford/dev/playground/opm && git add cmd/list.go internal/output/output.go internal/output/output_test.go && git commit -m "feat(list): add --long flag showing profile paths"
```

---

### Task 6: Add `--from <src>` flag to `opm create`

**Files:**
- Modify: `cmd/create.go`

When `--from <src>` is provided, call `s.CopyProfile(from, name)` instead of `s.CreateProfile(name)`. Success message distinguishes between empty create and copy-from.

- [ ] **Step 1: Update `cmd/create.go`**

Replace the entire file with:

```go
package cmd

import (
	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/spf13/cobra"
)

var createFrom string

var createCmd = &cobra.Command{
	Use:               "create <name>",
	Short:             "Create a new profile",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	SilenceUsage:      true,
	RunE:              runCreate,
}

func init() {
	createCmd.Flags().StringVar(&createFrom, "from", "", "Copy an existing profile as the starting point")
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := newStore()

	if createFrom != "" {
		if err := s.CopyProfile(createFrom, name); err != nil {
			return err
		}
		dstDir := paths.ProfileDir(name)
		output.Success(cmd.OutOrStdout(),
			"Created profile "+output.ProfileName(name)+" from "+output.ProfileName(createFrom),
			output.ShortenHome(dstDir)+"/",
		)
		return nil
	}

	if err := s.CreateProfile(name); err != nil {
		return err
	}
	profileDir := paths.ProfileDir(name)
	output.Success(cmd.OutOrStdout(), "Created profile "+output.ProfileName(name),
		output.ShortenHome(profileDir)+"/")
	return nil
}
```

- [ ] **Step 2: Build and smoke test**

```bash
cd /Users/tylercrawford/dev/playground/opm && go build -o opm . && ./opm create my-new-profile --from default && ./opm list; rm opm
```

Expected:
```
✓ Created profile my-new-profile from default
  ~/.config/opm/profiles/my-new-profile/
```
And `my-new-profile` appears in list.

- [ ] **Step 3: Run full test suite**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./...
```

Expected: all packages pass.

- [ ] **Step 4: Commit**

```bash
cd /Users/tylercrawford/dev/playground/opm && git add cmd/create.go && git commit -m "feat(create): add --from flag to copy an existing profile"
```
