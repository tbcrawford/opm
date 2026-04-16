# Doctor Output Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Restyle `opm doctor` output to use labelled sections ("Symlink", "Profiles", "Consistency") with no top-level header, so checks are grouped by what they inspect and the summary line stays at the bottom.

**Architecture:** Add a `DoctorSection` helper to `internal/output/output.go` that prints a dim section label. Rewrite `cmd/doctor.go` to call `DoctorSection` before each group of checks, remove the `fmt.Fprintln(out, "opm doctor")` header, and drop the `— ok` / `— directory missing` suffixes from profile rows (profile name alone is enough, status is conveyed by the ✓/✗ symbol). The Consistency section is only printed when a mismatch exists. Early-exit on symlink failure still works — only the Symlink section appears.

**Tech Stack:** Go stdlib, `text/tabwriter`, existing `internal/output` package (`DoctorRow`, `DoctorSummary`), `fatih/color`.

---

### Task 1: Add `DoctorSection` to output package and rewrite doctor command

**Files:**
- Modify: `internal/output/output.go` (add `DoctorSection`)
- Modify: `internal/output/output_test.go` (add `TestDoctorSection`)
- Modify: `cmd/doctor.go` (rewrite output structure)

#### Target output — all clear

```
Symlink
  ✓  ~/.config/opencode → superpowers

Profiles
  ✓  default
  ✓  superpowers

✓ All checks passed
```

#### Target output — failure + warning

```
Symlink
  ✓  ~/.config/opencode → superpowers

Profiles
  ✓  default
  ✗  work — directory missing
  ✓  superpowers

Consistency
  ⚠  current file says "default" but active symlink points to "superpowers"

✗ 1 problem found
```

#### Target output — symlink check fails (early exit)

```
Symlink
  ✗  ~/.config/opencode is not an opm-managed symlink — run 'opm init'

✗ 1 problem found
```

---

- [ ] **Step 1: Write failing test for `DoctorSection` in `internal/output/output_test.go`**

Add this test to the existing test file:

```go
func TestDoctorSection(t *testing.T) {
	var buf bytes.Buffer
	DoctorSection(&buf, "Profiles")
	assert.Equal(t, "Profiles\n", buf.String())
}
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./internal/output/... -run TestDoctorSection -v
```

Expected: FAIL — `DoctorSection undefined`

- [ ] **Step 3: Add `DoctorSection` to `internal/output/output.go`**

Add after `DoctorRow`:

```go
// DoctorSection prints a dim section label for grouping doctor checks.
func DoctorSection(w io.Writer, label string) {
	fmt.Fprintln(w, dim.Sprint(label))
}
```

- [ ] **Step 4: Run test to confirm it passes**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./internal/output/... -run TestDoctorSection -v
```

Expected: PASS

- [ ] **Step 5: Rewrite `cmd/doctor.go`**

Replace the entire file with:

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

	// ── Symlink ──────────────────────────────────────────────────────────────
	output.DoctorSection(out, "Symlink")

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

	st, err := symlink.Inspect(opencodeDir)
	if err != nil {
		output.DoctorRow(w, output.StatusFail, fmt.Sprintf("inspect ~/.config/opencode: %v", err))
		failures++
	} else if st.Dangling {
		output.DoctorRow(w, output.StatusFail,
			fmt.Sprintf("~/.config/opencode → %q (profile directory missing)", st.Target))
		failures++
	} else {
		activeName, _ := s.ActiveProfile()
		output.DoctorRow(w, output.StatusOK,
			fmt.Sprintf("~/.config/opencode → %s", output.ProfileName(activeName)))
	}
	_ = w.Flush()
	fmt.Fprintln(out)

	// ── Profiles ─────────────────────────────────────────────────────────────
	output.DoctorSection(out, "Profiles")

	profiles, err := s.ListProfiles()
	if err != nil {
		output.DoctorRow(w, output.StatusFail, fmt.Sprintf("list profiles: %v", err))
		failures++
	} else {
		for _, p := range profiles {
			if p.Dangling {
				output.DoctorRow(w, output.StatusFail,
					fmt.Sprintf("%s — directory missing", output.ProfileName(p.Name)))
				failures++
				continue
			}
			fi, statErr := os.Lstat(p.Path)
			if statErr != nil || !fi.IsDir() {
				output.DoctorRow(w, output.StatusFail,
					fmt.Sprintf("%s — not a valid directory", output.ProfileName(p.Name)))
				failures++
			} else {
				output.DoctorRow(w, output.StatusOK, output.ProfileName(p.Name))
			}
		}
	}
	_ = w.Flush()

	// ── Consistency (only shown when mismatch exists) ─────────────────────────
	current, curErr := s.GetCurrent()
	active, actErr := s.ActiveProfile()
	mismatch := curErr == nil && actErr == nil && current != "" && active != "" && current != active
	if mismatch {
		fmt.Fprintln(out)
		output.DoctorSection(out, "Consistency")
		output.DoctorRow(w, output.StatusWarn,
			fmt.Sprintf("current file says %q but active symlink points to %q", current, active))
		warnings++
		_ = w.Flush()
	}

	fmt.Fprintln(out)
	output.DoctorSummary(out, warnings, failures)

	if failures > 0 {
		os.Exit(1)
	}
	return nil
}
```

- [ ] **Step 6: Build and smoke test**

```bash
cd /Users/tylercrawford/dev/playground/opm && go build -o opm . && ./opm doctor; rm opm
```

Expected output:
```
Symlink
  ✓  ~/.config/opencode → <active-profile>

Profiles
  ✓  <profile1>
  ✓  <profile2>

✓ All checks passed
```

No `opm doctor` header. No `— ok` suffix on profile rows. Blank line between sections. Summary at bottom.

- [ ] **Step 7: Run full test suite**

```bash
cd /Users/tylercrawford/dev/playground/opm && go test ./...
```

Expected: all packages pass.

- [ ] **Step 8: Commit**

```bash
cd /Users/tylercrawford/dev/playground/opm && git add internal/output/output.go internal/output/output_test.go cmd/doctor.go && git commit -m "feat(doctor): section-grouped output with no header"
```
