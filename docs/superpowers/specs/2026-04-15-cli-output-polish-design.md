# CLI Output Polish — Design Spec

**Date:** 2026-04-15  
**Status:** Approved

## Goal

Polish opm's CLI output into a modern, readable experience without being verbose. Add ANSI color and symbols to communicate state at a glance, add brief contextual detail lines to state-changing commands, and consolidate all rendering into a single package.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Color library | `fatih/color` | Lightweight, widely used, respects `NO_COLOR` automatically |
| Visual style | Structured Blocks | ✓ confirmation + dim detail line for state changes; terse for reads |
| `ls` density | Name + active marker only | Fast to scan; no extraneous columns |
| Architecture | Centralized `internal/output` package | One place for all rendering, style, and TTY detection |

## Architecture

### `internal/output` package

New package that owns all terminal rendering. Commands stop formatting output themselves — they call typed helpers and pass `cmd.OutOrStdout()`.

**TTY / NO_COLOR detection** — at package `init()`, if stdout is not a TTY (i.e. being piped), disable color globally via `color.NoColor = true`. `fatih/color` additionally respects the `NO_COLOR` env var automatically. This means `opm context ls | grep foo` produces clean plain text.

**Path abbreviation** — all paths shown to the user are home-abbreviated (`/Users/tyler/...` → `~/...`). The output package exposes a `output.ShortenHome(path string) string` helper used by any command that renders a path.

**Public API:**

```go
// Success prints a green ✓ line + optional dim detail line.
// Used by all state-changing commands.
output.Success(w io.Writer, msg string, detail ...string)

// Failure prints a red ✗ line + optional dim detail line.
// Used for non-fatal command-level errors printed before returning.
output.Failure(w io.Writer, msg string, detail ...string)

// ProfileName returns a bold blue-formatted profile name for inline use in strings.
output.ProfileName(name string) string

// ProfileTable writes the `ls` output: ●/○/✗ + name per row.
output.ProfileTable(w io.Writer, profiles []store.Profile)

// InspectProfile writes the `inspect` block: name header, path, active status, contents.
output.InspectProfile(w io.Writer, name, path string, active bool, entries []os.DirEntry)

// DoctorRow writes a single tabwriter-aligned doctor check line.
output.DoctorRow(tw *tabwriter.Writer, status DoctorStatus, msg string)

// DoctorSummary writes the final summary line for `doctor`.
output.DoctorSummary(w io.Writer, warnings, failures int)
```

### Error handling

Add `SilenceErrors: true` to `rootCmd`. Update `Execute()` to catch the returned error and print it using a styled `output.Error()` helper (red `✗` prefix, hints indented with dim color). This replaces Cobra's plain `Error: <message>` prefix and makes multi-line hint messages render cleanly.

## Per-Command Output Spec

### `opm init`

**Migrating existing config:**
```
✓ Initialized opm
  Migrated ~/.config/opencode → profiles/default
```

**Fresh machine (no existing config):**
```
✓ Initialized opm
  Created default profile at ~/.config/opm/profiles/default/
```

**Already initialized (error):**
```
✗ Already initialized (active: default)
```

---

### `opm context create <name>`

```
✓ Created profile work
  ~/.config/opm/profiles/work/
```

---

### `opm context use <name>`

```
✓ Switched to work
  ~/.config/opencode → profiles/work
```

---

### `opm context ls`

```
● work
○ personal
○ research
✗ staging (missing)
```

- `●` green, profile name bold blue — active profile
- `○` dim — inactive profiles  
- `✗` red, profile name red — dangling/missing profiles

---

### `opm context inspect <name>`

```
work ● active

Path     ~/.config/opm/profiles/work
Contents config.json
         AGENTS.md
         plugins/
```

- Name rendered bold blue, active badge green
- Labels dim, values normal weight
- Contents list under the `Contents` label, subsequent files indented to align

---

### `opm context rename <old> <new>`

**Inactive profile:**
```
✓ Renamed work → work-v2
```

**Active profile:**
```
✓ Renamed work → work-v2
  Active profile updated
```

---

### `opm context rm <name>`

**Successful removal:**
```
✓ Removed profile work
```

**With `--force` (auto-switches first):**
```
✓ Switched to default
  Auto-switched before removal
✓ Removed profile work
```

**Error — removing active without `--force`:**
```
✗ Cannot remove the active profile

  Switch first:     opm context use <name>
  Or force remove:  opm context rm --force work
```

---

### `opm doctor`

**With failures:**
```
opm doctor

✓  ~/.config/opencode → work
✓  Profile work — ok
✓  Profile personal — ok
✗  Profile staging — directory missing

✗ 1 problem found
```

**All healthy:**
```
opm doctor

✓  ~/.config/opencode → work
✓  Profile work — ok
✓  Profile personal — ok

✓ All checks passed
```

- `✓` green, `✗` red, `⚠` yellow (warnings)
- Summary line colored to match severity
- Profile names bold blue in check rows

## Symbol & Color Reference

| Element | Symbol | Color |
|---------|--------|-------|
| Success / active | `✓` / `●` | Green |
| Inactive | `○` | Dim |
| Error / missing | `✗` | Red |
| Warning | `⚠` | Yellow |
| Profile name (inline) | — | Bold blue |
| Detail / hint lines | — | Dim |
| Labels (`inspect`) | — | Dim |

## Out of Scope

- Interactive selection / TUI (not needed for v1)
- JSON output mode (YAGNI)
- Spinner/progress animation (all operations are instantaneous)
- Any change to `opm context show` — it intentionally prints bare name for scripting
