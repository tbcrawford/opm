# CLI Output Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Polish opm's CLI output with ANSI color, symbols, and a structured-blocks style via a centralized `internal/output` package.

**Architecture:** A new `internal/output` package wraps `fatih/color` and exposes typed helpers (`Success`, `Failure`, `Error`, `ProfileTable`, `InspectProfile`, `DoctorRow`, `DoctorSummary`). All command files are updated to call these helpers instead of formatting output inline. TTY detection and `NO_COLOR` handling live exclusively in the output package.

**Tech Stack:** Go stdlib, `github.com/fatih/color` (new), `github.com/spf13/cobra` (existing), `github.com/stretchr/testify` (existing)

---

### Task 1: Add fatih/color dependency

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add the dependency**

```bash
go get github.com/fatih/color@latest
```

- [ ] **Step 2: Verify it appears in go.mod**

```bash
grep "fatih/color" go.mod
```

Expected output contains: `github.com/fatih/color v1.x.x`

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add fatih/color dependency"
```

---

### Task 2: Create internal/output — core helpers

**Files:**
- Create: `internal/output/output.go`
- Create: `internal/output/output_test.go`

- [ ] **Step 1: Write failing tests**

Create `internal/output/output_test.go`:

```go
package output_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/fatih/color"
	"github.com/opm-cli/opm/internal/output"
	"github.com/stretchr/testify/assert"
)

func TestMain(m *testing.M) {
	color.NoColor = true // strip ANSI codes in tests
	os.Exit(m.Run())
}

func TestSuccess_WithDetail(t *testing.T) {
	var buf bytes.Buffer
	output.Success(&buf, "Switched to work", "~/.config/opencode → profiles/work")
	assert.Equal(t, "✓ Switched to work\n  ~/.config/opencode → profiles/work\n", buf.String())
}

func TestSuccess_NoDetail(t *testing.T) {
	var buf bytes.Buffer
	output.Success(&buf, "Removed profile work")
	assert.Equal(t, "✓ Removed profile work\n", buf.String())
}

func TestFailure_WithDetail(t *testing.T) {
	var buf bytes.Buffer
	output.Failure(&buf, "Cannot remove the active profile", "Switch first: opm context use <name>")
	assert.Equal(t, "✗ Cannot remove the active profile\n  Switch first: opm context use <name>\n", buf.String())
}

func TestError_SingleLine(t *testing.T) {
	var buf bytes.Buffer
	output.Error(&buf, "profile \"foo\" does not exist")
	assert.Equal(t, "✗ profile \"foo\" does not exist\n", buf.String())
}

func TestError_MultiLine(t *testing.T) {
	var buf bytes.Buffer
	output.Error(&buf, "cannot remove the active profile\n\n  Switch first: opm context use <name>")
	assert.Equal(t, "✗ cannot remove the active profile\n\n  Switch first: opm context use <name>\n", buf.String())
}

func TestProfileName(t *testing.T) {
	assert.Equal(t, "work", output.ProfileName("work"))
}

func TestShortenHome_UnderHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	assert.Equal(t, "~/foo/bar", output.ShortenHome(home+"/foo/bar"))
}

func TestShortenHome_NotUnderHome(t *testing.T) {
	assert.Equal(t, "/etc/foo", output.ShortenHome("/etc/foo"))
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/output/...
```

Expected: compile error — package does not exist yet.

- [ ] **Step 3: Create the output package**

Create `internal/output/output.go`:

```go
package output

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
)

var (
	green  = color.New(color.FgGreen)
	blue   = color.New(color.FgBlue, color.Bold)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	dim    = color.New(color.Faint)
)

func init() {
	// Disable color when stdout is not a terminal (e.g. piped to grep/file).
	// fatih/color additionally respects the NO_COLOR env var automatically.
	if fi, err := os.Stdout.Stat(); err == nil {
		if fi.Mode()&os.ModeCharDevice == 0 {
			color.NoColor = true
		}
	}
}

// Success prints a green ✓ line followed by optional dim detail lines.
// Used by all state-changing commands on success.
func Success(w io.Writer, msg string, detail ...string) {
	fmt.Fprintf(w, "%s %s\n", green.Sprint("✓"), msg)
	for _, d := range detail {
		fmt.Fprintf(w, "%s\n", dim.Sprintf("  %s", d))
	}
}

// Failure prints a red ✗ line followed by optional dim detail lines.
// Used for non-fatal in-command failure messages printed before returning an error.
func Failure(w io.Writer, msg string, detail ...string) {
	fmt.Fprintf(w, "%s %s\n", red.Sprint("✗"), msg)
	for _, d := range detail {
		fmt.Fprintf(w, "%s\n", dim.Sprintf("  %s", d))
	}
}

// Error prints a red ✗ first line, then remaining lines with dim indent preserved.
// Used by Execute() to format all command errors uniformly.
func Error(w io.Writer, msg string) {
	parts := strings.SplitN(msg, "\n", 2)
	fmt.Fprintf(w, "%s %s\n", red.Sprint("✗"), parts[0])
	if len(parts) > 1 && parts[1] != "" {
		fmt.Fprintln(w, dim.Sprint(parts[1]))
	}
}

// ProfileName returns the profile name formatted as bold blue for inline use in strings.
func ProfileName(name string) string {
	return blue.Sprint(name)
}

// ShortenHome replaces the user's home directory prefix with ~.
func ShortenHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/output/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/output/output.go internal/output/output_test.go
git commit -m "feat(output): add core helpers — Success, Failure, Error, ProfileName, ShortenHome"
```

---

### Task 3: Add ProfileTable to output package

**Files:**
- Modify: `internal/output/output.go`
- Modify: `internal/output/output_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/output/output_test.go`:

```go
func TestProfileTable_Mixed(t *testing.T) {
	var buf bytes.Buffer
	profiles := []store.Profile{
		{Name: "work", Active: true},
		{Name: "personal"},
		{Name: "staging", Dangling: true},
	}
	output.ProfileTable(&buf, profiles)
	got := buf.String()
	assert.Contains(t, got, "● work")
	assert.Contains(t, got, "○ personal")
	assert.Contains(t, got, "✗ staging")
	assert.Contains(t, got, "(missing)")
}

func TestProfileTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	output.ProfileTable(&buf, nil)
	assert.Equal(t, "", buf.String())
}
```

Add the import for `store` to the import block at the top of the test file:

```go
import (
	"bytes"
	"os"
	"testing"

	"github.com/fatih/color"
	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/store"
	"github.com/stretchr/testify/assert"
)
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/output/...
```

Expected: compile error — `output.ProfileTable` undefined.

- [ ] **Step 3: Implement ProfileTable**

Append to `internal/output/output.go`:

```go
// ProfileTable writes the ls listing: one profile per line with ●/○/✗ markers.
func ProfileTable(w io.Writer, profiles []store.Profile) {
	for _, p := range profiles {
		switch {
		case p.Dangling:
			fmt.Fprintf(w, "%s %s %s\n", red.Sprint("✗"), red.Sprint(p.Name), dim.Sprint("(missing)"))
		case p.Active:
			fmt.Fprintf(w, "%s %s\n", green.Sprint("●"), blue.Sprint(p.Name))
		default:
			fmt.Fprintf(w, "%s\n", dim.Sprintf("○ %s", p.Name))
		}
	}
}
```

Add the store import to `output.go`'s import block:

```go
import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/opm-cli/opm/internal/store"
)
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/output/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/output/output.go internal/output/output_test.go
git commit -m "feat(output): add ProfileTable"
```

---

### Task 4: Add InspectProfile to output package

**Files:**
- Modify: `internal/output/output.go`
- Modify: `internal/output/output_test.go`

- [ ] **Step 1: Write failing test**

Append to `internal/output/output_test.go`:

```go
func TestInspectProfile_Active(t *testing.T) {
	var buf bytes.Buffer
	entries := []os.DirEntry{} // use empty for simplicity; real entries tested via integration
	output.InspectProfile(&buf, "work", "/home/user/.config/opm/profiles/work", true, entries)
	got := buf.String()
	assert.Contains(t, got, "work")
	assert.Contains(t, got, "● active")
	assert.Contains(t, got, "Contents")
	assert.Contains(t, got, "(empty)")
}

func TestInspectProfile_Inactive(t *testing.T) {
	var buf bytes.Buffer
	output.InspectProfile(&buf, "personal", "/home/user/.config/opm/profiles/personal", false, nil)
	got := buf.String()
	assert.Contains(t, got, "personal")
	assert.NotContains(t, got, "● active")
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/output/...
```

Expected: compile error — `output.InspectProfile` undefined.

- [ ] **Step 3: Implement InspectProfile**

Append to `internal/output/output.go`:

```go
// InspectProfile writes the inspect block: name header, path row, contents list.
func InspectProfile(w io.Writer, name, path string, active bool, entries []os.DirEntry) {
	// Header: name + optional active badge
	if active {
		fmt.Fprintf(w, "%s %s\n", blue.Sprint(name), green.Sprint("● active"))
	} else {
		fmt.Fprintln(w, blue.Sprint(name))
	}
	fmt.Fprintln(w)

	// Path row
	fmt.Fprintf(w, "%s%s\n", dim.Sprintf("%-9s", "Path"), ShortenHome(path))

	// Contents rows
	if len(entries) == 0 {
		fmt.Fprintf(w, "%s%s\n", dim.Sprintf("%-9s", "Contents"), dim.Sprint("(empty)"))
		return
	}
	for i, e := range entries {
		entryName := e.Name()
		if e.IsDir() {
			entryName += "/"
		}
		label := ""
		if i == 0 {
			label = "Contents"
		}
		fmt.Fprintf(w, "%s%s\n", dim.Sprintf("%-9s", label), entryName)
	}
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/output/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/output/output.go internal/output/output_test.go
git commit -m "feat(output): add InspectProfile"
```

---

### Task 5: Add DoctorRow and DoctorSummary to output package

**Files:**
- Modify: `internal/output/output.go`
- Modify: `internal/output/output_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/output/output_test.go`:

```go
func TestDoctorRow_OK(t *testing.T) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	output.DoctorRow(tw, output.StatusOK, "everything fine")
	tw.Flush()
	assert.Contains(t, buf.String(), "✓")
	assert.Contains(t, buf.String(), "everything fine")
}

func TestDoctorRow_Fail(t *testing.T) {
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	output.DoctorRow(tw, output.StatusFail, "profile missing")
	tw.Flush()
	assert.Contains(t, buf.String(), "✗")
	assert.Contains(t, buf.String(), "profile missing")
}

func TestDoctorSummary_Healthy(t *testing.T) {
	var buf bytes.Buffer
	output.DoctorSummary(&buf, 0, 0)
	assert.Contains(t, buf.String(), "All checks passed")
}

func TestDoctorSummary_Failures(t *testing.T) {
	var buf bytes.Buffer
	output.DoctorSummary(&buf, 0, 2)
	assert.Contains(t, buf.String(), "2 problem")
}

func TestDoctorSummary_Warnings(t *testing.T) {
	var buf bytes.Buffer
	output.DoctorSummary(&buf, 1, 0)
	assert.Contains(t, buf.String(), "1 warning")
}
```

Add `"text/tabwriter"` to the test file's import block:

```go
import (
	"bytes"
	"os"
	"testing"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/store"
	"github.com/stretchr/testify/assert"
)
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
go test ./internal/output/...
```

Expected: compile error — `output.DoctorRow` and `output.DoctorSummary` undefined.

- [ ] **Step 3: Implement DoctorRow and DoctorSummary**

Append to `internal/output/output.go`:

```go
// DoctorStatus represents the result of a single doctor check.
type DoctorStatus int

const (
	StatusOK   DoctorStatus = iota
	StatusWarn              // yellow ⚠
	StatusFail              // red ✗
)

// DoctorRow writes a single tabwriter-aligned doctor check line.
func DoctorRow(tw *tabwriter.Writer, status DoctorStatus, msg string) {
	switch status {
	case StatusOK:
		fmt.Fprintf(tw, "  %s\t%s\n", green.Sprint("✓"), msg)
	case StatusWarn:
		fmt.Fprintf(tw, "  %s\t%s\n", yellow.Sprint("⚠"), msg)
	case StatusFail:
		fmt.Fprintf(tw, "  %s\t%s\n", red.Sprint("✗"), msg)
	}
}

// DoctorSummary writes the final summary line for `opm doctor`.
func DoctorSummary(w io.Writer, warnings, failures int) {
	switch {
	case failures == 0 && warnings == 0:
		fmt.Fprintln(w, green.Sprint("✓ All checks passed"))
	case failures == 0:
		fmt.Fprintln(w, yellow.Sprintf("⚠ %d warning(s)", warnings))
	default:
		fmt.Fprintln(w, red.Sprintf("✗ %d problem(s) found", failures))
	}
}
```

Add `"text/tabwriter"` to `output.go`'s import block:

```go
import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/opm-cli/opm/internal/store"
)
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
go test ./internal/output/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/output/output.go internal/output/output_test.go
git commit -m "feat(output): add DoctorRow and DoctorSummary"
```

---

### Task 6: Update root.go — SilenceErrors + styled error printer

**Files:**
- Modify: `cmd/root.go`

- [ ] **Step 1: Update root.go**

Replace the file content of `cmd/root.go` with:

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/opm-cli/opm/internal/store"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "opm",
	Short:         "OpenCode profile manager",
	Long:          "opm manages multiple OpenCode configurations by switching symlinked profile directories.",
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute is the binary entry point called by main.go.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		output.Error(os.Stderr, err.Error())
		os.Exit(1)
	}
}

// SetVersionInfo sets the version string displayed by `opm --version`.
// Called from main() with values injected by GoReleaser ldflags.
func SetVersionInfo(version, commit string) {
	rootCmd.Version = version + " (" + commit + ")"
}

// newStore returns a production Store wired to real config paths.
func newStore() *store.Store {
	return store.New(paths.OpmDir(), paths.OpencodeConfigDir())
}

// managedGuard is a PersistentPreRunE that blocks context subcommands
// if ~/.config/opencode is not managed by opm (per D-08/D-09).
// Do NOT attach to opm init — init is the bootstrap command.
func managedGuard(cmd *cobra.Command, args []string) error {
	s := newStore()
	managed, err := s.IsOpmManaged()
	if err != nil {
		return fmt.Errorf("cannot determine opm state: %w", err)
	}
	if !managed {
		return fmt.Errorf("~/.config/opencode is not managed by opm\n\n  Run 'opm init' to initialize.")
	}
	return nil
}
```

- [ ] **Step 2: Build to confirm no compile errors**

```bash
go build ./...
```

Expected: no output (clean build).

- [ ] **Step 3: Smoke-test error formatting manually**

```bash
./opm context use nonexistent 2>&1 || true
```

Expected output starts with `✗` (red in a terminal) instead of `Error:`.

- [ ] **Step 4: Commit**

```bash
git add cmd/root.go
git commit -m "feat(cmd): silence cobra errors, route through styled output.Error"
```

---

### Task 7: Update init.go

**Files:**
- Modify: `cmd/init.go`

- [ ] **Step 1: Update init.go**

Replace `cmd/init.go` with:

```go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/opm-cli/opm/internal/symlink"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:          "init",
	Short:        "Initialize opm and migrate existing OpenCode config",
	Long:         "Migrates ~/.config/opencode to a 'default' opm profile and installs the managing symlink.",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	opencodeDir := paths.OpencodeConfigDir()
	defaultProfileDir := paths.ProfileDir("default")
	profilesDir := paths.ProfilesDir()

	// Step 0: Idempotency check.
	st, err := symlink.Inspect(opencodeDir)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", opencodeDir, err)
	}

	if st.IsSymlink && strings.HasPrefix(st.Target, profilesDir) {
		activeName := filepath.Base(st.Target)
		return fmt.Errorf("Already initialized (active: %s)", activeName)
	}

	// Foreign symlink — not safe to touch.
	if st.IsSymlink && !strings.HasPrefix(st.Target, profilesDir) {
		return fmt.Errorf("~/.config/opencode is an unrecognized symlink\n\n  Back it up and remove it, then run 'opm init' again.")
	}

	// Detect partial crash recovery.
	if _, statErr := os.Lstat(defaultProfileDir); statErr == nil {
		tmpSym := opencodeDir + ".opm-new"
		if _, tmpErr := os.Lstat(tmpSym); tmpErr == nil {
			if err := os.Rename(tmpSym, opencodeDir); err != nil {
				return fmt.Errorf("resume: atomic rename symlink: %w", err)
			}
			s := newStore()
			if err := s.SetCurrent("default"); err != nil {
				return fmt.Errorf("set current: %w", err)
			}
			w := cmd.OutOrStdout()
			output.Success(w, "Initialized opm", "Migrated ~/.config/opencode → profiles/default")
			return nil
		}
		return fmt.Errorf("Already initialized (active: default)")
	}

	s := newStore()
	if err := s.Init(); err != nil {
		return fmt.Errorf("create opm dirs: %w", err)
	}

	w := cmd.OutOrStdout()

	if st.IsDir {
		// Existing real directory — 3-step crash-safe migration (per D-16).
		tmpSym := opencodeDir + ".opm-new"
		_ = os.Remove(tmpSym)
		if err := symlink.SetAtomic(defaultProfileDir, tmpSym); err != nil {
			return fmt.Errorf("step 1 — create temp symlink: %w", err)
		}
		if err := os.Rename(opencodeDir, defaultProfileDir); err != nil {
			_ = os.Remove(tmpSym)
			return fmt.Errorf("step 2 — move opencode dir to profile: %w", err)
		}
		if err := os.Rename(tmpSym, opencodeDir); err != nil {
			return fmt.Errorf("step 3 — install symlink: %w", err)
		}
		if err := s.SetCurrent("default"); err != nil {
			return fmt.Errorf("set current: %w", err)
		}
		output.Success(w, "Initialized opm", "Migrated ~/.config/opencode → profiles/default")
	} else {
		// Fresh machine: no ~/.config/opencode exists yet.
		if err := os.MkdirAll(defaultProfileDir, 0o755); err != nil {
			return fmt.Errorf("create default profile: %w", err)
		}
		if err := symlink.SetAtomic(defaultProfileDir, opencodeDir); err != nil {
			return fmt.Errorf("install symlink: %w", err)
		}
		if err := s.SetCurrent("default"); err != nil {
			return fmt.Errorf("set current: %w", err)
		}
		output.Success(w, "Initialized opm",
			"Created default profile at "+output.ShortenHome(defaultProfileDir)+"/")
	}

	return nil
}
```

- [ ] **Step 2: Build to confirm no compile errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add cmd/init.go
git commit -m "feat(cmd/init): styled output with migration vs fresh distinction"
```

---

### Task 8: Update context_create.go

**Files:**
- Modify: `cmd/context_create.go`

- [ ] **Step 1: Update context_create.go**

Replace `cmd/context_create.go` with:

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
	RunE:              runContextCreate,
}

func init() {
	contextCmd.AddCommand(createCmd)
}

func runContextCreate(cmd *cobra.Command, args []string) error {
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

- [ ] **Step 2: Build to confirm no compile errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add cmd/context_create.go
git commit -m "feat(cmd/context create): styled output"
```

---

### Task 9: Update context_use.go

**Files:**
- Modify: `cmd/context_use.go`

- [ ] **Step 1: Update context_use.go**

Replace `cmd/context_use.go` with:

```go
package cmd

import (
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
	RunE:              runContextUse,
}

func init() {
	contextCmd.AddCommand(useCmd)
}

func runContextUse(cmd *cobra.Command, args []string) error {
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

Add `"fmt"` to the import block since it's used in the error wrapping:

```go
import (
	"fmt"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/opm-cli/opm/internal/symlink"
	"github.com/spf13/cobra"
)
```

- [ ] **Step 2: Build to confirm no compile errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add cmd/context_use.go
git commit -m "feat(cmd/context use): styled output"
```

---

### Task 10: Update context_ls.go

**Files:**
- Modify: `cmd/context_ls.go`

- [ ] **Step 1: Update context_ls.go**

Replace `cmd/context_ls.go` with:

```go
package cmd

import (
	"fmt"

	"github.com/opm-cli/opm/internal/output"
	"github.com/spf13/cobra"
)

var lsCmd = &cobra.Command{
	Use:               "ls",
	Short:             "List all profiles",
	Args:              cobra.NoArgs,
	PersistentPreRunE: managedGuard,
	SilenceUsage:      true,
	RunE:              runContextLs,
}

func init() {
	contextCmd.AddCommand(lsCmd)
}

func runContextLs(cmd *cobra.Command, args []string) error {
	s := newStore()
	profiles, err := s.ListProfiles()
	if err != nil {
		return err
	}

	if len(profiles) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No profiles found. Run 'opm context create <name>' to create one.")
		return nil
	}

	output.ProfileTable(cmd.OutOrStdout(), profiles)
	return nil
}
```

- [ ] **Step 2: Build to confirm no compile errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add cmd/context_ls.go
git commit -m "feat(cmd/context ls): styled output via ProfileTable"
```

---

### Task 11: Update context_inspect.go

**Files:**
- Modify: `cmd/context_inspect.go`

- [ ] **Step 1: Update context_inspect.go**

Replace `cmd/context_inspect.go` with:

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
	RunE:              runContextInspect,
}

func init() {
	contextCmd.AddCommand(inspectCmd)
}

func runContextInspect(cmd *cobra.Command, args []string) error {
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

- [ ] **Step 2: Build to confirm no compile errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add cmd/context_inspect.go
git commit -m "feat(cmd/context inspect): styled output via InspectProfile"
```

---

### Task 12: Update context_rename.go

**Files:**
- Modify: `cmd/context_rename.go`

- [ ] **Step 1: Update context_rename.go**

Replace `cmd/context_rename.go` with:

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
	RunE:              runContextRename,
}

func init() {
	contextCmd.AddCommand(renameCmd)
}

func runContextRename(cmd *cobra.Command, args []string) error {
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

- [ ] **Step 2: Build to confirm no compile errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add cmd/context_rename.go
git commit -m "feat(cmd/context rename): styled output"
```

---

### Task 13: Update context_rm.go

**Files:**
- Modify: `cmd/context_rm.go`

- [ ] **Step 1: Update context_rm.go**

Replace `cmd/context_rm.go` with:

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

var rmForce bool

var rmCmd = &cobra.Command{
	Use:               "rm <name>",
	Short:             "Remove a profile",
	Args:              cobra.ExactArgs(1),
	PersistentPreRunE: managedGuard,
	ValidArgsFunction: profileNameCompletion,
	SilenceUsage:      true,
	RunE:              runContextRm,
}

func init() {
	rmCmd.Flags().BoolVarP(&rmForce, "force", "f", false, "Force removal of the active profile (auto-switches first)")
	contextCmd.AddCommand(rmCmd)
}

func runContextRm(cmd *cobra.Command, args []string) error {
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

	if isActive && !rmForce {
		return fmt.Errorf("cannot remove the active profile\n\n  Switch first:     opm context use <name>\n  Or force remove:  opm context rm --force %s", name)
	}

	if isActive && rmForce {
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
		return "", fmt.Errorf("cannot remove the only profile\n\n  Create another profile first:\n    opm context create <name>")
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

- [ ] **Step 2: Build to confirm no compile errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 3: Commit**

```bash
git add cmd/context_rm.go
git commit -m "feat(cmd/context rm): styled output and cleaner error messages"
```

---

### Task 14: Update doctor.go

**Files:**
- Modify: `cmd/doctor.go`

- [ ] **Step 1: Update doctor.go**

Replace `cmd/doctor.go` with:

```go
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/opm-cli/opm/internal/output"
	"github.com/opm-cli/opm/internal/paths"
	"github.com/opm-cli/opm/internal/symlink"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:          "doctor",
	Short:        "Check opm installation health",
	Long:         "Runs a series of checks on the opm installation and reports any issues.",
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE:         runDoctor,
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)

	opencodeDir := paths.OpencodeConfigDir()
	s := newStore()

	warnings := 0
	failures := 0

	fmt.Fprintln(out, "opm doctor")
	fmt.Fprintln(out)

	// Check 1: Is opm managing ~/.config/opencode?
	managed, err := s.IsOpmManaged()
	if err != nil {
		output.DoctorRow(w, output.StatusFail, fmt.Sprintf("~/.config/opencode: %v", err))
		failures++
		_ = w.Flush()
		fmt.Fprintln(out)
		output.DoctorSummary(out, warnings, failures)
		os.Exit(1)
	}
	if !managed {
		output.DoctorRow(w, output.StatusFail, "~/.config/opencode is not an opm-managed symlink — run 'opm init'")
		failures++
		_ = w.Flush()
		fmt.Fprintln(out)
		output.DoctorSummary(out, warnings, failures)
		os.Exit(1)
	}

	// Check 2: Is the active symlink dangling?
	st, err := symlink.Inspect(opencodeDir)
	if err != nil {
		output.DoctorRow(w, output.StatusFail, fmt.Sprintf("inspect ~/.config/opencode: %v", err))
		failures++
	} else if st.Dangling {
		output.DoctorRow(w, output.StatusFail,
			fmt.Sprintf("~/.config/opencode is a dangling symlink → %q (profile directory is missing)", st.Target))
		failures++
	} else {
		activeName, _ := s.ActiveProfile()
		output.DoctorRow(w, output.StatusOK,
			fmt.Sprintf("~/.config/opencode → %s", output.ProfileName(activeName)))
	}

	// Check 3: Each profile directory.
	profiles, err := s.ListProfiles()
	if err != nil {
		output.DoctorRow(w, output.StatusFail, fmt.Sprintf("list profiles: %v", err))
		failures++
	} else {
		for _, p := range profiles {
			if p.Dangling {
				output.DoctorRow(w, output.StatusFail,
					fmt.Sprintf("Profile %s — directory missing", output.ProfileName(p.Name)))
				failures++
				continue
			}
			fi, statErr := os.Lstat(p.Path)
			if statErr != nil || !fi.IsDir() {
				output.DoctorRow(w, output.StatusFail,
					fmt.Sprintf("Profile %s — not a valid directory: %s", output.ProfileName(p.Name), p.Path))
				failures++
			} else {
				output.DoctorRow(w, output.StatusOK,
					fmt.Sprintf("Profile %s — ok", output.ProfileName(p.Name)))
			}
		}
	}

	// Check 4: Cross-check current file vs Readlink-derived active profile.
	current, curErr := s.GetCurrent()
	active, actErr := s.ActiveProfile()
	if curErr == nil && actErr == nil && current != "" && active != "" && current != active {
		output.DoctorRow(w, output.StatusWarn,
			fmt.Sprintf("current file says %q but active symlink points to %q", current, active))
		warnings++
	}

	_ = w.Flush()
	fmt.Fprintln(out)
	output.DoctorSummary(out, warnings, failures)

	if failures > 0 {
		os.Exit(1)
	}
	return nil
}
```

- [ ] **Step 2: Build to confirm no compile errors**

```bash
go build ./...
```

Expected: no output.

- [ ] **Step 3: Run full test suite**

```bash
go test ./...
```

Expected: all tests pass.

- [ ] **Step 4: Commit**

```bash
git add cmd/doctor.go
git commit -m "feat(cmd/doctor): styled output via DoctorRow and DoctorSummary"
```

---

### Task 15: Final smoke test

**Files:** none (verification only)

- [ ] **Step 1: Build the binary**

```bash
go build -o opm .
```

- [ ] **Step 2: Run doctor on a live opm install**

```bash
./opm doctor
```

Expected: colored ✓/✗ rows + summary line.

- [ ] **Step 3: Verify piped output strips color**

```bash
./opm context ls | cat
```

Expected: plain text with ●/○/✗ symbols but no ANSI escape codes.

- [ ] **Step 4: Verify NO_COLOR is respected**

```bash
NO_COLOR=1 ./opm context ls
```

Expected: plain text output, no color codes.

- [ ] **Step 5: Run full test suite one final time**

```bash
go test ./...
```

Expected: all tests pass.
