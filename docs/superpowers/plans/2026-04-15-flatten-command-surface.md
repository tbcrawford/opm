# Flatten Command Surface Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Flatten `opm context <verb>` into `opm <verb>` at the root level, rename `ls` → `list` (with `ls` alias) and `rm` → `remove` (with `rm` alias), and delete the now-unused `context` grouping command.

**Architecture:** Each context subcommand file is re-wired to register on `rootCmd` instead of `contextCmd`. The `context.go` grouping file is deleted. Error messages that reference `opm context <verb>` are updated to `opm <verb>`. Cobra aliases (`Aliases` field) handle `ls`/`rm` as shortcuts.

**Tech Stack:** Go, cobra v1.10.2, existing `internal/output` and `internal/store` packages.

---

### Task 1: Rename `context_ls.go` → `list.go` and wire to root

**Files:**
- Delete: `cmd/context_ls.go`
- Create: `cmd/list.go`

- [ ] **Step 1: Create `cmd/list.go`**

```go
package cmd

import (
	"fmt"

	"github.com/opm-cli/opm/internal/output"
	"github.com/spf13/cobra"
)

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

	output.ProfileTable(cmd.OutOrStdout(), profiles)
	return nil
}
```

- [ ] **Step 2: Delete `cmd/context_ls.go`**

```bash
rm cmd/context_ls.go
```

- [ ] **Step 3: Build and verify**

```bash
go build ./...
go run . list
go run . ls
```

Expected: both `list` and `ls` show the profile table.

- [ ] **Step 4: Commit**

```bash
git add cmd/list.go cmd/context_ls.go
git commit -m "feat: promote list command to root, add ls alias"
```

---

### Task 2: Rename `context_rm.go` → `remove.go` and wire to root

**Files:**
- Delete: `cmd/context_rm.go`
- Create: `cmd/remove.go`

- [ ] **Step 1: Create `cmd/remove.go`**

```go
package cmd

import (
	"fmt"
	"sort"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/opm-cli/opm/internal/store"
	"github.com/opm-cli/opm/internal/symlink"
	"github.com/spf13/cobra"
)

var removeForce bool

var removeCmd = &cobra.Command{
	Use:               "remove <name>",
	Aliases:           []string{"rm"},
	Short:             "Remove a profile",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runRemove,
}

func init() {
	removeCmd.Flags().BoolVarP(&removeForce, "force", "f", false, "Force removal of the active profile (auto-switches first)")
	rootCmd.AddCommand(removeCmd)
}

func runRemove(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := newStore()

	if _, err := s.GetProfile(name); err != nil {
		return err
	}

	active, err := s.ActiveProfile()
	if err != nil {
		return fmt.Errorf("determine active profile: %w", err)
	}

	isActive := active == name
	w := cmd.OutOrStdout()

	if isActive && !removeForce {
		return fmt.Errorf("cannot remove the active profile\n\n  Switch first:     opm use <name>\n  Or force remove:  opm remove --force %s", name)
	}

	if isActive && removeForce {
		switchTarget, err := selectAutoSwitchTarget(s, name)
		if err != nil {
			return err
		}

		targetDir := s.ProfileDir(switchTarget)
		if err := symlink.SetAtomic(targetDir, paths.OpencodeConfigDir()); err != nil {
			return fmt.Errorf("switch to %q: %w", switchTarget, err)
		}
		if err := s.SetCurrent(switchTarget); err != nil {
			return fmt.Errorf("update current: %w", err)
		}
		output.Success(w, "Switched to "+output.ProfileName(switchTarget), "Auto-switched before removal")
	}

	if err := s.DeleteProfile(name, true); err != nil {
		return err
	}
	output.Success(w, "Removed profile "+output.ProfileName(name))
	return nil
}

func selectAutoSwitchTarget(s *store.Store, deletingName string) (string, error) {
	profiles, err := s.ListProfiles()
	if err != nil {
		return "", err
	}

	var candidates []string
	for _, p := range profiles {
		if p.Name != deletingName {
			candidates = append(candidates, p.Name)
		}
	}

	if len(candidates) == 0 {
		return "", fmt.Errorf("cannot remove the only profile\n\n  Create another profile first:\n    opm create <name>")
	}

	for _, c := range candidates {
		if c == "default" {
			return "default", nil
		}
	}

	sort.Strings(candidates)
	return candidates[0], nil
}
```

- [ ] **Step 2: Delete `cmd/context_rm.go`**

```bash
rm cmd/context_rm.go
```

- [ ] **Step 3: Build and verify**

```bash
go build ./...
go run . remove --help
go run . rm --help
```

Expected: both show the remove command help with `--force` flag.

- [ ] **Step 4: Commit**

```bash
git add cmd/remove.go cmd/context_rm.go
git commit -m "feat: promote remove command to root, add rm alias, update error messages"
```

---

### Task 3: Promote `context_create.go`, `context_use.go`, `context_show.go` to root

**Files:**
- Delete: `cmd/context_create.go`, `cmd/context_use.go`, `cmd/context_show.go`
- Create: `cmd/create.go`, `cmd/use.go`, `cmd/show.go`

- [ ] **Step 1: Create `cmd/create.go`**

```go
package cmd

import (
	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:               "create <name>",
	Short:             "Create a new profile",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	SilenceUsage:      true,
	RunE:              runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := newStore()
	if err := s.CreateProfile(name); err != nil {
		return err
	}
	profileDir := paths.ProfileDir(name)
	output.Success(cmd.OutOrStdout(), "Created profile "+output.ProfileName(name),
		output.ShortenHome(profileDir)+"/")
	return nil
}
```

- [ ] **Step 2: Create `cmd/use.go`**

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

	opencodeDir := paths.OpencodeConfigDir()
	if err := symlink.SetAtomic(profileDir, opencodeDir); err != nil {
		return fmt.Errorf("switch profile: %w", err)
	}

	if err := s.SetCurrent(name); err != nil {
		return fmt.Errorf("update current: %w", err)
	}

	output.Success(cmd.OutOrStdout(), "Switched to "+output.ProfileName(name),
		output.ShortenHome(opencodeDir)+" → profiles/"+name)
	return nil
}
```

- [ ] **Step 3: Create `cmd/show.go`**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:               "show",
	Short:             "Print the name of the currently active profile",
	Args:              cobra.NoArgs,
	PersistentPreRunE: managedGuard,
	SilenceUsage:      true,
	RunE:              runShow,
}

func init() {
	rootCmd.AddCommand(showCmd)
}

func runShow(cmd *cobra.Command, args []string) error {
	s := newStore()
	current, err := s.GetCurrent()
	if err != nil {
		return err
	}
	if current == "" {
		current, err = s.ActiveProfile()
		if err != nil || current == "" {
			return fmt.Errorf("no active profile — run 'opm init' first")
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), current)
	return nil
}
```

- [ ] **Step 4: Delete the old files**

```bash
rm cmd/context_create.go cmd/context_use.go cmd/context_show.go
```

- [ ] **Step 5: Build and verify**

```bash
go build ./...
go run . create --help
go run . use --help
go run . show
```

- [ ] **Step 6: Commit**

```bash
git add cmd/create.go cmd/use.go cmd/show.go cmd/context_create.go cmd/context_use.go cmd/context_show.go
git commit -m "feat: promote create, use, show commands to root"
```

---

### Task 4: Promote `context_inspect.go` and `context_rename.go` to root

**Files:**
- Delete: `cmd/context_inspect.go`, `cmd/context_rename.go`
- Create: `cmd/inspect.go`, `cmd/rename.go`

- [ ] **Step 1: Create `cmd/inspect.go`**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/opm-cli/opm/internal/output"
	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:               "inspect <name>",
	Short:             "Show detailed information about a profile",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) error {
	name := args[0]
	s := newStore()

	profilePath, err := s.GetProfile(name)
	if err != nil {
		return err
	}

	active, _ := s.ActiveProfile()
	isActive := active == name

	entries, err := os.ReadDir(profilePath)
	if err != nil {
		return fmt.Errorf("read profile contents: %w", err)
	}

	output.InspectProfile(cmd.OutOrStdout(), name, profilePath, isActive, entries)
	return nil
}
```

- [ ] **Step 2: Create `cmd/rename.go`**

```go
package cmd

import (
	"fmt"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/opm-cli/opm/internal/symlink"
	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:               "rename <old> <new>",
	Short:             "Rename a profile",
	Args:              cobra.ExactArgs(2),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runRename,
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
	oldName, newName := args[0], args[1]
	s := newStore()

	active, err := s.ActiveProfile()
	if err != nil {
		return fmt.Errorf("determine active profile: %w", err)
	}
	wasActive := active == oldName

	if err := s.RenameProfile(oldName, newName); err != nil {
		return err
	}

	w := cmd.OutOrStdout()
	msg := "Renamed " + output.ProfileName(oldName) + " → " + output.ProfileName(newName)

	if wasActive {
		newDir := s.ProfileDir(newName)
		if err := symlink.SetAtomic(newDir, paths.OpencodeConfigDir()); err != nil {
			return fmt.Errorf("update active symlink: %w", err)
		}
		if err := s.SetCurrent(newName); err != nil {
			return fmt.Errorf("update current: %w", err)
		}
		output.Success(w, msg, "Active profile updated")
	} else {
		output.Success(w, msg)
	}
	return nil
}
```

- [ ] **Step 3: Delete old files**

```bash
rm cmd/context_inspect.go cmd/context_rename.go
```

- [ ] **Step 4: Build and verify**

```bash
go build ./...
go run . inspect --help
go run . rename --help
```

- [ ] **Step 5: Commit**

```bash
git add cmd/inspect.go cmd/rename.go cmd/context_inspect.go cmd/context_rename.go
git commit -m "feat: promote inspect and rename commands to root"
```

---

### Task 5: Delete `context.go` grouping file and verify full command surface

**Files:**
- Delete: `cmd/context.go`

- [ ] **Step 1: Delete `cmd/context.go`**

```bash
rm cmd/context.go
```

- [ ] **Step 2: Build**

```bash
go build ./...
```

Expected: clean build, no errors.

- [ ] **Step 3: Verify full command surface**

```bash
go run . --help
```

Expected output (order may vary):
```
Available Commands:
  completion  Generate the autocompletion script for the specified shell
  create      Create a new profile
  doctor      Check opm installation health
  help        Help about any command
  init        Initialize opm and migrate existing OpenCode config
  inspect     Show detailed information about a profile
  list        List all profiles
  remove      Remove a profile
  rename      Rename a profile
  show        Print the name of the currently active profile
  use         Switch to a profile
```

- [ ] **Step 4: Verify aliases work**

```bash
go run . ls
go run . rm --help
```

Expected: `ls` lists profiles, `rm` shows remove help.

- [ ] **Step 5: Run full test suite**

```bash
go test ./...
```

Expected: all packages pass.

- [ ] **Step 6: Commit**

```bash
git add cmd/context.go
git commit -m "feat: remove context grouping command, command surface fully flattened"
```
