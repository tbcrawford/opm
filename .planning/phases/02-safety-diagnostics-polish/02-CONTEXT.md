# Phase 2: Safety, Diagnostics & Polish — Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 2 adds the safety net, diagnostic tooling, and shell completion that make opm production-ready for daily use. After this phase: users are protected from common destructive mistakes, can diagnose a broken install without reading source code, and get tab completion on profile names.

SAFE-01 (`rm` active-profile guard + `--force`) was fully implemented in Phase 1 and requires no further work.

### In scope for Phase 2:
- **SAFE-02**: Dangling profile detection in `opm context ls` — when a profile dir has been manually deleted but the symlink still points to it
- **SAFE-03**: `opm doctor` health check command
- **POLISH-01**: Shell completion for all profile-name arguments

</domain>

<decisions>
## Implementation Decisions

### SAFE-01 (already done)
- **D-01:** `opm context rm` active-profile guard with `--force` auto-switch is **complete in Phase 1**. No further work needed. Confirmed by smoke tests.

---

### SAFE-02: Dangling symlink detection in `opm context ls`

The failure scenario:
1. User has profile `work` → `~/.config/opm/profiles/work/` exists
2. User manually runs `rm -rf ~/.config/opm/profiles/work/`
3. `~/.config/opencode` still symlinks to the (now-gone) `work/` dir
4. `opm context ls` must surface this, not silently omit it

- **D-02:** Add a `Dangling bool` field to `store.Profile`. For every profile returned by `ListProfiles`, this is `false` (profile dirs listed by `os.ReadDir` exist by definition). The dangling case is the active profile.
- **D-03:** In `store.ListProfiles`, after scanning the `profiles/` dir, call `symlink.Inspect(opencodeDir)`. If the symlink is dangling (target does not exist), AND the target basename is not in the scanned profiles list, synthesize a `Profile{Name: basename, Dangling: true, Active: true}` and prepend it to the list.
- **D-04:** `opm context ls` output for a dangling active profile: `! <name> (missing)` — using `!` as the marker (no emoji, shell-friendly, clearly not `*`). Non-dangling active profile keeps `*`.
  ```
  ! work (missing)    ← dangling active profile
    default           ← healthy profile
  ```
- **D-05:** `store.ListProfiles` returns profiles sorted alphabetically, but the synthesized dangling entry is included in the sort (not pinned to top). The `!` marker in ls makes it visually distinct regardless of position.

---

### SAFE-03: `opm doctor` health report

- **D-06:** `opm doctor` is a **top-level command** (`opm doctor`), not under `opm context`. It diagnoses the whole opm installation.
- **D-07:** `opm doctor` does NOT require opm to be managed (no `managedGuard`). It runs even on an uninitialized machine and reports the uninitialized state as the first finding.
- **D-08:** Output format — plain text with bracketed status indicators. No color (cross-terminal safe). Width: fixed labels, left-aligned. Example:
  ```
  opm health report:
    [OK]   ~/.config/opencode is an opm-managed symlink → "default"
    [OK]   Profile "default" directory exists
    [OK]   Profile "work" directory exists
    [WARN] current file says "old" but active symlink points to "work"
    [FAIL] Profile "work" directory is missing (dangling symlink)

  Status: 1 warning, 1 failure
  ```
- **D-09:** Exit code: 0 if all OK or only warnings; 1 if any `[FAIL]`s. This makes `opm doctor` scriptable.
- **D-10:** Checks performed by `opm doctor` (in order):
  1. Check `~/.config/opencode` exists and is an opm-managed symlink
  2. If not managed: report `[FAIL] ~/.config/opencode is not an opm-managed symlink — run 'opm init'` and stop (no further checks possible)
  3. Check symlink is not dangling: `[FAIL] Active profile symlink is dangling — profile directory is missing`
  4. For each profile in `profiles/` dir: check dir is a real directory: `[OK]` or `[FAIL]`
  5. Cross-check `current` file value against Readlink-derived active profile: `[WARN]` if they differ (harmless but indicative of state drift)
- **D-11:** `opm doctor` does not auto-fix anything in Phase 2. Read-only diagnostic.

---

### POLISH-01: Shell completion

- **D-12:** Use Cobra's `ValidArgsFunction` on each command that takes a `<name>` argument. This enables `opm context use <tab>` → profile names in zsh, bash, and fish automatically (Cobra generates the completion scripts).
- **D-13:** Commands that get `ValidArgsFunction` for profile names: `use`, `inspect`, `rm`. NOT `create` (taking a new name — no meaningful completion).
- **D-14:** The completion function calls `store.ListProfiles()` and returns `[]string` of profile names + `cobra.ShellCompDirectiveNoFileComp`. If `ListProfiles` fails or the store isn't managed, return `nil, cobra.ShellCompDirectiveError` (completion silently fails — not an error in UX terms).
- **D-15:** No custom completion for `opm init` or `opm doctor` (no arguments).
- **D-16:** The completion helper is defined once in `cmd/completion.go` as `profileNameCompletion(cmd, args, toComplete)` and registered on each command.

---

### Architecture (additions over Phase 1)

- **D-17:** `store.Profile` gets a `Dangling bool` field — backward-compatible addition.
- **D-18:** `store.ListProfiles` gains dangling-detection logic. All existing tests remain valid; new tests are added.
- **D-19:** `cmd/doctor.go` is a new file with `doctorCmd` registered on `rootCmd`.
- **D-20:** `cmd/completion.go` is a new file with `profileNameCompletion` helper registered on `use`, `inspect`, `rm` commands.
- **D-21:** No changes to `internal/symlink` or `internal/paths` — Phase 2 is purely additive.

### Agent Discretion

- Exact wording of `[OK]` / `[WARN]` / `[FAIL]` messages (beyond the examples above)
- Width of the label padding in doctor output (`text/tabwriter` or manual padding)
- Whether `opm doctor` uses `text/tabwriter` for alignment (recommended: yes)
- `ShellCompDirective` fallback behavior when store is unmanaged

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Context
- `.planning/PROJECT.md` — vision, constraints
- `.planning/REQUIREMENTS.md` — SAFE-01–03, POLISH-01 acceptance criteria
- `.planning/phases/01-foundation-core-cli/01-CONTEXT.md` — Phase 1 decisions (all apply)

### Existing Implementations (Phase 1 — do not break)
- `internal/store/store.go` — `Profile` struct, `ListProfiles`, `IsOpmManaged`
- `internal/symlink/symlink.go` — `Inspect`, `Status.Dangling`
- `cmd/root.go` — `managedGuard`, `newStore()`
- `cmd/context_ls.go` — output format to extend for dangling marker

### External Docs
- Cobra `ValidArgsFunction`: https://pkg.go.dev/github.com/spf13/cobra#Command (ShellCompDirective)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (Phase 1)
- `symlink.Status.Dangling` — already implemented; use it in `store.ListProfiles`
- `store.IsOpmManaged()` — use in `opm doctor` to gate further checks
- `store.ActiveProfile()` — Readlink-derived active name; use in cross-check against `current` file
- `cmd/root.go:newStore()` — use in all new commands and the completion helper

### Established Patterns
- Commands register themselves in `init()` on the relevant parent command
- `cmd.OutOrStdout()` for output, errors returned (not printed)
- `SilenceUsage: true` on all commands
- Docker convention: entity names in double quotes in error messages

### Integration Points (new)
- `store.Profile.Dangling` — new field; `cmd/context_ls.go` must check it
- `doctorCmd` → `rootCmd.AddCommand(doctorCmd)` (top-level, not under context)
- `ValidArgsFunction` wired in each command file's `init()` (not centralized in root)

</code_context>

<specifics>
## Specific Implementation Notes

- `opm doctor` summary line: `Status: healthy` if all OK, `Status: N warning(s)` if warnings only, `Status: N failure(s)` if any fails (warnings also counted separately)
- `opm context ls` with dangling active: exit code 0 (it's displaying information, not erroring)
- `opm doctor` with failures: exit code 1 (scriptable health check)
- Completion: `ShellCompDirectiveNoFileComp` prevents file-path completion fallback (profile names are not filenames from the shell's perspective)

</specifics>

<deferred>
## Deferred

- `opm doctor --fix` auto-repair — v2 requirement
- `--format json` on `opm doctor` — v2
- Completion for `opm doctor` — no arguments, nothing to complete
- Size warning on `opm context rm` large profiles — Phase 2 discussion decided it's a Phase 3 concern
- Fish completion script installation instructions — cobra handles it; no extra work needed

</deferred>

---

*Phase: 02-safety-diagnostics-polish*
*Context gathered: 2026-04-15*
