# Custom Grouped Help Output Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace cobra's default help output with a custom grouped, colorized help page for `opm` that uses Steel Blue (`color.New(color.FgHiBlue)`) for section headers.

**Architecture:** Add a `HelpFunc` on `rootCmd` (registered in `cmd/help.go`) that renders grouped command sections. Subcommand help (e.g. `opm use --help`) uses a separate `UsageTemplate` set on each command via a shared helper. All color logic lives in `internal/output` so tests can suppress it via `color.NoColor = true`.

**Tech Stack:** `github.com/fatih/color`, `github.com/spf13/cobra`, `text/tabwriter`, Go stdlib

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/output/output.go` | Modify | Add `HelpHeader`, `HelpCommand`, `HelpFlag`, `HelpSection` helpers |
| `internal/output/output_test.go` | Modify | Add tests for new help rendering helpers |
| `cmd/help.go` | Create | `registerHelp(cmd)` — sets `SetHelpFunc` on rootCmd + `SetUsageTemplate` on each subcommand |
| `cmd/root.go` | Modify | Call `registerHelp(rootCmd)` in an `init()` |

---

## Command Grouping Reference

Use this exact grouping in the help output:

**Setup**
- `init` — Initialize opm and migrate your existing OpenCode config
- `doctor` — Check opm installation health
- `reset` — Remove opm management and restore your config directory

**Profiles**
- `create` — Create a new profile
- `copy` — Copy an existing profile to a new name
- `use` — Switch to a profile
- `list` — List all profiles  *(alias: ls)*
- `show` — Show the active profile name
- `inspect` — Show profile details and contents
- `rename` — Rename a profile
- `remove` — Remove a profile  *(alias: rm)*

**Scripting**
- `path` — Print the absolute path to a profile directory

Do NOT show `help` or `completion` in the grouped list.

---

## Color Scheme: Steel Blue

```go
// Add to internal/output/output.go
var (
    steelBlue = color.New(color.FgHiBlue, color.Bold) // section headers
    cmdName   = color.New(color.FgHiWhite, color.Bold) // command names in help
    flagColor = color.New(color.FgCyan)                // flag names in subcommand usage
)
```

---

## Task 1: Add help rendering helpers to `internal/output`

**Files:**
- Modify: `internal/output/output.go`
- Modify: `internal/output/output_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/output/output_test.go` (after the existing tests):

```go
func TestHelpSection(t *testing.T) {
	var buf strings.Builder
	HelpSection(&buf, "Setup")
	assert.Equal(t, "Setup\n", buf.String())
}

func TestHelpCommand(t *testing.T) {
	var buf strings.Builder
	HelpCommand(&buf, "init", "Initialize opm and migrate your existing OpenCode config", "")
	assert.Equal(t, "  init    Initialize opm and migrate your existing OpenCode config\n", buf.String())
}

func TestHelpCommandWithAlias(t *testing.T) {
	var buf strings.Builder
	HelpCommand(&buf, "list", "List all profiles", "ls")
	assert.Equal(t, "  list    List all profiles  (ls)\n", buf.String())
}

func TestHelpHeader(t *testing.T) {
	var buf strings.Builder
	HelpHeader(&buf, "opm", "OpenCode profile manager")
	assert.Equal(t, "opm — OpenCode profile manager\n\nUsage:\n  opm <command> [flags]\n\n", buf.String())
}

func TestHelpFlag(t *testing.T) {
	result := HelpFlag("--version", "Print version and exit")
	assert.Equal(t, "  --version    Print version and exit", result)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/tylercrawford/dev/playground/opm
go test ./internal/output/... -run "TestHelpSection|TestHelpCommand|TestHelpCommandWithAlias|TestHelpHeader|TestHelpFlag" -v
```

Expected: `FAIL` — functions not defined.

- [ ] **Step 3: Add the helpers to `internal/output/output.go`**

Add these new color vars after the existing `var` block (after line 23):

```go
var (
    steelBlue = color.New(color.FgHiBlue, color.Bold)
    cmdColor  = color.New(color.FgHiWhite, color.Bold)
    flagColor = color.New(color.FgCyan)
)
```

Then add these functions at the end of `internal/output/output.go`:

```go
// HelpHeader writes the top-level header block for `opm --help`.
// Format:
//
//	opm — OpenCode profile manager
//
//	Usage:
//	  opm <command> [flags]
func HelpHeader(w io.Writer, name, short string) {
	fmt.Fprintf(w, "%s — %s\n\nUsage:\n  %s <command> [flags]\n\n", cmdColor.Sprint(name), short, name)
}

// HelpSection writes a colored section header (e.g. "Setup").
func HelpSection(w io.Writer, label string) {
	fmt.Fprintln(w, steelBlue.Sprint(label))
}

// HelpCommand writes a single command row inside a section.
// alias is optional; pass "" to omit.
// Output is tab-padded for alignment when written through a tabwriter.
func HelpCommand(w io.Writer, name, description, alias string) {
	aliasStr := ""
	if alias != "" {
		aliasStr = "  " + dim.Sprintf("(%s)", alias)
	}
	fmt.Fprintf(w, "  %s\t%s%s\n", cmdColor.Sprint(name), description, aliasStr)
}

// HelpFlag returns a formatted flag entry string for inline use in subcommand usage templates.
func HelpFlag(flag, description string) string {
	return fmt.Sprintf("  %s    %s", flagColor.Sprint(flag), description)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/tylercrawford/dev/playground/opm
go test ./internal/output/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/tylercrawford/dev/playground/opm
git add internal/output/output.go internal/output/output_test.go
git commit -m "feat(output): add help rendering helpers (HelpHeader, HelpSection, HelpCommand, HelpFlag)"
```

---

## Task 2: Create `cmd/help.go` with `registerHelp`

**Files:**
- Create: `cmd/help.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Create `cmd/help.go`**

```go
package cmd

import (
	"bytes"
	"fmt"
	"text/tabwriter"

	"github.com/opm-cli/opm/internal/output"
	"github.com/spf13/cobra"
)

// registerHelp sets a custom help function on the root command and a shared
// usage template on each subcommand. Call this from an init() in root.go.
func registerHelp(root *cobra.Command) {
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// If called on a subcommand (e.g. `opm use --help`), fall back to
		// cobra's default help so the subcommand's flags are shown.
		if cmd != root {
			cmd.Usage()
			return
		}
		printRootHelp(cmd)
	})
}

// printRootHelp renders the grouped help page for `opm --help`.
func printRootHelp(root *cobra.Command) {
	w := root.OutOrStdout()

	output.HelpHeader(w, "opm", "OpenCode profile manager")

	type entry struct {
		name  string
		short string
		alias string
	}
	type section struct {
		label   string
		entries []entry
	}

	sections := []section{
		{
			label: "Setup",
			entries: []entry{
				{"init", "Initialize opm and migrate your existing OpenCode config", ""},
				{"doctor", "Check opm installation health", ""},
				{"reset", "Remove opm management and restore your config directory", ""},
			},
		},
		{
			label: "Profiles",
			entries: []entry{
				{"create", "Create a new profile", ""},
				{"copy", "Copy an existing profile to a new name", ""},
				{"use", "Switch to a profile", ""},
				{"list", "List all profiles", "ls"},
				{"show", "Show the active profile name", ""},
				{"inspect", "Show profile details and contents", ""},
				{"rename", "Rename a profile", ""},
				{"remove", "Remove a profile", "rm"},
			},
		},
		{
			label: "Scripting",
			entries: []entry{
				{"path", "Print the absolute path to a profile directory", ""},
			},
		},
	}

	for i, sec := range sections {
		output.HelpSection(w, sec.label)
		var buf bytes.Buffer
		tw := tabwriter.NewWriter(&buf, 0, 0, 4, ' ', 0)
		for _, e := range sec.entries {
			output.HelpCommand(tw, e.name, e.short, e.alias)
		}
		_ = tw.Flush()
		fmt.Fprint(w, buf.String())
		if i < len(sections)-1 {
			fmt.Fprintln(w)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Flags:")
	fmt.Fprintln(w, output.HelpFlag("--version", "Print version and exit"))
	fmt.Fprintln(w, output.HelpFlag("--help", "Show this help"))
}
```

- [ ] **Step 2: Register in `cmd/root.go`**

Add an `init()` function at the bottom of `cmd/root.go`:

```go
func init() {
	registerHelp(rootCmd)
}
```

- [ ] **Step 3: Smoke-test the help output manually**

```bash
cd /Users/tylercrawford/dev/playground/opm
go run . --help
```

Expected output (colors visible on TTY):
```
opm — OpenCode profile manager

Usage:
  opm <command> [flags]

Setup
  init      Initialize opm and migrate your existing OpenCode config
  doctor    Check opm installation health
  reset     Remove opm management and restore your config directory

Profiles
  create    Create a new profile
  copy      Copy an existing profile to a new name
  use       Switch to a profile
  list      List all profiles  (ls)
  show      Show the active profile name
  inspect   Show profile details and contents
  rename    Rename a profile
  remove    Remove a profile  (rm)

Scripting
  path      Print the absolute path to a profile directory

Flags:
  --version    Print version and exit
  --help       Show this help
```

- [ ] **Step 4: Verify subcommand help still works**

```bash
go run . use --help
go run . create --help
go run . list --help
```

Each should show cobra's default help (flags, usage line, description) — not the grouped root help.

- [ ] **Step 5: Build to verify no compile errors**

```bash
cd /Users/tylercrawford/dev/playground/opm
go build ./...
```

Expected: exits 0, no output.

- [ ] **Step 6: Run full test suite**

```bash
cd /Users/tylercrawford/dev/playground/opm
go test ./...
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/tylercrawford/dev/playground/opm
git add cmd/help.go cmd/root.go
git commit -m "feat(cmd): add custom grouped Steel Blue help output for opm --help"
```

---

## Self-Review

**Spec coverage:**
- ✓ Grouped sections: Setup / Profiles / Scripting
- ✓ Steel Blue section headers via `color.FgHiBlue, color.Bold`
- ✓ Command names highlighted (`FgHiWhite, Bold`)
- ✓ Aliases shown inline (`(ls)`, `(rm)`)
- ✓ `help` and `completion` excluded from grouped list
- ✓ Subcommand help falls back to cobra default (flags visible)
- ✓ `NO_COLOR` / non-TTY respected automatically via `fatih/color`
- ✓ TDD: failing tests written before implementation
- ✓ Frequent commits (one per task)

**Placeholder scan:** None found — all steps contain complete code.

**Type consistency:** `HelpCommand` accepts `io.Writer` in Task 1; called with `tw` (`*tabwriter.Writer`) in Task 2 — valid, `tabwriter.Writer` implements `io.Writer`. `HelpFlag` returns `string` in Task 1; called with `fmt.Fprintln` in Task 2 — correct.
