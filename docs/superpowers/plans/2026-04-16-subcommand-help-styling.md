# Subcommand Help Styling Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Apply the same Steel Blue styled output to every subcommand's `--help` page, replacing cobra's default plain-text template.

**Architecture:** Add a `SubcmdHelp(cmd)` renderer to `internal/output` that formats a single command's help page (name, short description, usage line, flags). Update `registerHelp` in `cmd/help.go` so the subcommand branch calls `printSubcmdHelp` instead of cobra's default. Each subcommand's flags are read live from `cmd.Flags()` and `cmd.InheritedFlags()` so the output stays in sync automatically.

**Tech Stack:** `github.com/fatih/color`, `github.com/spf13/cobra`, `text/tabwriter`, `github.com/spf13/pflag`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/output/output.go` | Modify | Add `SubcmdHelp(w, cmd)` renderer |
| `internal/output/output_test.go` | Modify | Add tests for `SubcmdHelp` |
| `cmd/help.go` | Modify | Call `printSubcmdHelp` in the subcommand branch |

---

## Styled subcommand help format

For `opm use --help`, the output should look like:

```
opm use — Switch to a profile

Usage:
  opm use <name> [flags]

Flags:
  --help    help for use
```

For `opm create --help` (has a non-trivial flag):

```
opm create — Create a new profile

Usage:
  opm create <name> [flags]

Flags:
  --from    Copy an existing profile as the starting point
  --help    help for create
```

For `opm list --help` (has `-l`/`--long`):

```
opm list — List all profiles

Usage:
  opm list [flags]

Flags:
  -l, --long    Show profile paths
  --help        help for list
```

Rules:
- Header: `opm <subcommand> — <Short description>` (styled via existing `HelpHeader`)
- "Usage:" and "Flags:" are Steel Blue section headers (via `HelpSection`)
- Each flag row uses `HelpFlag` — show shorthand as `-x, --flag` when present, else just `--flag`
- `--help` is always included (from cobra's inherited flags)
- If a command has a `Long` description, show it between the header and "Usage:"
- Hidden flags are skipped

---

## Task 1: Add `SubcmdHelp` to `internal/output`

**Files:**
- Modify: `internal/output/output.go`
- Modify: `internal/output/output_test.go`

- [ ] **Step 1: Write failing tests**

Add to `internal/output/output_test.go`. Add the import `"github.com/spf13/cobra"` and `"github.com/spf13/pflag"` to the import block.

```go
func TestSubcmdHelp_NoFlags(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "use <name>",
		Short: "Switch to a profile",
	}
	var buf bytes.Buffer
	SubcmdHelp(&buf, cmd)
	out := buf.String()
	assert.Contains(t, out, "opm use — Switch to a profile\n")
	assert.Contains(t, out, "Usage:\n")
	assert.Contains(t, out, "  opm use <name> [flags]\n")
	assert.Contains(t, out, "Flags:\n")
	assert.Contains(t, out, "--help")
}

func TestSubcmdHelp_WithLong(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize opm",
		Long:  "Migrates ~/.config/opencode to a default profile.",
	}
	var buf bytes.Buffer
	SubcmdHelp(&buf, cmd)
	out := buf.String()
	assert.Contains(t, out, "Migrates ~/.config/opencode to a default profile.\n")
}

func TestSubcmdHelp_WithFlag(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new profile",
	}
	cmd.Flags().String("from", "", "Copy an existing profile as the starting point")
	var buf bytes.Buffer
	SubcmdHelp(&buf, cmd)
	out := buf.String()
	assert.Contains(t, out, "--from")
	assert.Contains(t, out, "Copy an existing profile as the starting point")
}

func TestSubcmdHelp_WithShorthand(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all profiles",
	}
	cmd.Flags().BoolP("long", "l", false, "Show profile paths")
	var buf bytes.Buffer
	SubcmdHelp(&buf, cmd)
	out := buf.String()
	assert.Contains(t, out, "-l, --long")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/tylercrawford/dev/playground/opm
go test ./internal/output/... -run "TestSubcmdHelp" -v
```

Expected: FAIL — `SubcmdHelp` undefined.

- [ ] **Step 3: Add `SubcmdHelp` to `internal/output/output.go`**

Add the import `"github.com/spf13/cobra"` to the import block in `output.go`.

Add this function at the end of `output.go`:

```go
// SubcmdHelp renders a styled help page for a single subcommand.
// It prints the command name and short description, optional long description,
// usage line, and flag table. Hidden flags are omitted.
func SubcmdHelp(w io.Writer, cmd *cobra.Command) {
	// Derive the full command path (e.g. "opm use") from the Use field.
	// cmd.Use may be "use <name>" — take only the first word.
	useParts := strings.Fields(cmd.Use)
	cmdName := "opm"
	if len(useParts) > 0 {
		cmdName = "opm " + useParts[0]
	}

	// Header: "opm use — Switch to a profile"
	fmt.Fprintf(w, "%s — %s\n", steelBlue.Sprint(cmdName), cmd.Short)

	// Long description (optional)
	if cmd.Long != "" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, cmd.Long)
	}

	// Usage section
	fmt.Fprintln(w)
	HelpSection(w, "Usage:")
	fmt.Fprintf(w, "  %s [flags]\n", cmd.UseLine())

	// Flags section — merge local + inherited, skip hidden
	allFlags := &pflag.FlagSet{}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden {
			allFlags.AddFlag(f)
		}
	})
	cmd.InheritedFlags().VisitAll(func(f *pflag.Flag) {
		if !f.Hidden && allFlags.Lookup(f.Name) == nil {
			allFlags.AddFlag(f)
		}
	})

	fmt.Fprintln(w)
	HelpSection(w, "Flags:")
	var buf bytes.Buffer
	tw := tabwriter.NewWriter(&buf, 0, 0, 4, ' ', 0)
	allFlags.VisitAll(func(f *pflag.Flag) {
		if f.Shorthand != "" {
			fmt.Fprintln(tw, HelpFlag("-"+f.Shorthand+", --"+f.Name, f.Usage))
		} else {
			fmt.Fprintln(tw, HelpFlag("--"+f.Name, f.Usage))
		}
	})
	_ = tw.Flush()
	fmt.Fprint(w, buf.String())
}
```

Also add `"bytes"` and `"github.com/spf13/pflag"` to the import block if not already present. Check `go.mod` — `pflag` is an indirect dependency of cobra, so it's available.

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
git commit -m "feat(output): add SubcmdHelp renderer for styled subcommand help pages"
```

---

## Task 2: Wire `SubcmdHelp` into `registerHelp`

**Files:**
- Modify: `cmd/help.go`

- [ ] **Step 1: Update `registerHelp` in `cmd/help.go`**

Change the subcommand branch (lines 19–21) from calling `defaultHelp` to calling `printSubcmdHelp`:

Current:
```go
func registerHelp(root *cobra.Command) {
	defaultHelp := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd != root {
			defaultHelp(cmd, args)
			return
		}
		printRootHelp(cmd)
	})
}
```

Replace with:
```go
func registerHelp(root *cobra.Command) {
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd != root {
			printSubcmdHelp(cmd)
			return
		}
		printRootHelp(cmd)
	})
}

// printSubcmdHelp renders the styled help page for a subcommand.
func printSubcmdHelp(cmd *cobra.Command) {
	output.SubcmdHelp(cmd.OutOrStdout(), cmd)
}
```

Remove the now-unused `defaultHelp` capture. The `output` import is already present.

- [ ] **Step 2: Build to verify no compile errors**

```bash
cd /Users/tylercrawford/dev/playground/opm
go build ./...
```

Expected: exits 0.

- [ ] **Step 3: Smoke-test all subcommands**

```bash
cd /Users/tylercrawford/dev/playground/opm
go run . use --help
go run . create --help
go run . list --help
go run . init --help
go run . reset --help
go run . copy --help
go run . rename --help
go run . remove --help
go run . inspect --help
go run . show --help
go run . path --help
go run . doctor --help
```

For each, verify:
- Header line: `opm <name> — <Short>`
- "Usage:" section header (Steel Blue on TTY)
- Usage line with correct arguments from `Use` field
- "Flags:" section with at least `--help`
- Commands with flags show them (create shows `--from`, list shows `-l, --long`)
- Commands with `Long` (doctor, init, reset) show it between header and Usage

- [ ] **Step 4: Verify root help unchanged**

```bash
go run . --help
```

Must still show the grouped sections output.

- [ ] **Step 5: Run full test suite**

```bash
cd /Users/tylercrawford/dev/playground/opm
go test ./...
```

Expected: all PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/tylercrawford/dev/playground/opm
git add cmd/help.go
git commit -m "feat(cmd): apply styled help to all subcommands via SubcmdHelp"
```

---

## Self-Review

**Spec coverage:**
- ✓ Header line with Steel Blue styled command name + short description
- ✓ Long description shown when present (doctor, init, reset)
- ✓ "Usage:" as a section header (Steel Blue)
- ✓ Usage line from `cmd.UseLine()` (includes correct arg shape from `Use` field)
- ✓ "Flags:" as a section header
- ✓ Flags read live from `cmd.Flags()` + `cmd.InheritedFlags()` — stays in sync automatically
- ✓ Shorthand rendered as `-x, --flag` when present
- ✓ Hidden flags skipped
- ✓ `--help` always present (it's in cobra's inherited flags)
- ✓ Root help (`opm --help`) unaffected
- ✓ TDD: failing tests before implementation
- ✓ Frequent commits (one per task)

**Placeholder scan:** None found.

**Type consistency:** `SubcmdHelp(w io.Writer, cmd *cobra.Command)` defined in Task 1, called as `output.SubcmdHelp(cmd.OutOrStdout(), cmd)` in Task 2 — correct.
