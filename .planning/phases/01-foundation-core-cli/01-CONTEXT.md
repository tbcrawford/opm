# Phase 1: Foundation + Core CLI — Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 1 delivers a working CLI binary: `opm init`, `opm context create/use/ls/inspect/rm/show`, and profile name validation. After this phase, users can initialize opm, create and switch between profiles, and inspect their profile environment. Safety features (rm --force guard, dangling symlink detection, opm doctor) and shell completions are Phase 2.

</domain>

<decisions>
## Implementation Decisions

### `opm context rm --force` (active profile)

- **D-01:** `--force` on the active profile **auto-switches before deleting** — it does not leave a dangling symlink.
- **D-02:** Auto-switch target order: `default` profile first (if it exists and is not the profile being deleted), then first available profile alphabetically.
- **D-03:** If no other profile exists at all, `rm --force` errors: "Cannot remove the only profile. Create another profile first."

### `opm context ls` output

- **D-04:** Shows **name + active marker only** — one profile per line, active profile prefixed with `*`. No path column, no size info. Clean and shell-prompt friendly.
- **D-05:** Output uses `text/tabwriter` for alignment. Example:
  ```
  * default
    work
    personal
  ```

### `opm context inspect` output

- **D-06:** Shows: name, path, active status, and a **first-level directory listing** of the profile directory (e.g. `agents/`, `plugins/`, `opencode.json`, `AGENTS.md`). No deep recursion.
- **D-07:** No model extraction from `opencode.json` in v1 — directory listing is sufficient.

### Unmanaged directory handling

- **D-08:** If `~/.config/opencode` is a **real directory** (not an opm-managed symlink), any command other than `opm init` errors with: `~/.config/opencode is not managed by opm. Run 'opm init' first.`
- **D-09:** This applies to `opm context use`, `opm context create`, and any other context subcommand. Only `opm init` is designed to take over an unmanaged directory.
- **D-10:** Silent auto-migration is explicitly rejected — it could destroy data without user awareness.

### Architecture (locked from research)

- **D-11:** Build order: `internal/paths` → `internal/store` + `internal/symlink` (parallel) → `cmd/` → `main.go`. Fully implement and test internal packages before any command code.
- **D-12:** Active profile is derived from `os.Readlink()` on `~/.config/opencode`, NOT from `~/.config/opm/current`. The `current` file is a write-through cache for shell prompt speed only.
- **D-13:** Symlink swap uses atomic temp+rename: `os.Symlink(target, tmp)` then `os.Rename(tmp, dst)`. Never remove-then-create.
- **D-14:** `os.Lstat` everywhere for path inspection — never `os.Stat` (which follows symlinks and hides dangling state).
- **D-15:** All symlink targets are absolute paths anchored to `os.UserHomeDir()`. No relative symlinks.
- **D-16:** `opm init` uses crash-safe 3-step sequence: (1) create temp symlink, (2) move real dir, (3) atomic rename temp symlink into place. Add idempotency checks for intermediate states.
- **D-17:** Profile names validated at `context create` time: `^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`. Path traversal prevention is a security requirement.
- **D-18:** macOS path gotcha: use `os.UserHomeDir()` + `filepath.Join(".config", ...)` — NOT `os.UserConfigDir()` (returns `~/Library/Application Support` on macOS, not `~/.config`).

### the agent's Discretion

- Error message formatting (exact wording beyond the patterns established above)
- `go.mod` minimum Go version (research recommends `go 1.22` for broad compatibility)
- Test coverage targets and table-driven test structure
- `text/tabwriter` column padding values

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Project Context
- `.planning/PROJECT.md` — vision, constraints, core value, key decisions
- `.planning/REQUIREMENTS.md` — CORE-01 through CORE-08, POLISH-02 acceptance criteria

### Research
- `.planning/research/SUMMARY.md` — stack decisions, architecture patterns, critical pitfalls (HIGH confidence)
- `.planning/research/STACK.md` — exact library versions, Go stdlib API references, macOS UserConfigDir gotcha
- `.planning/research/ARCHITECTURE.md` — component boundaries, data flow, build order, `opm init` decision tree
- `.planning/research/PITFALLS.md` — 5 critical pitfalls with prevention strategies (EEXIST, init crash window, active profile deletion, Lstat vs Stat, relative symlinks)

### Prior Art (external)
- `github.com/docker/cli` `cli/command/context/use.go` — reference implementation for context switching UX
- `github.com/spf13/cobra` `ValidArgsFunction` — shell completion pattern (Phase 2, but command structure must support it)

No project-specific ADRs yet — all decisions captured above.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- None yet — this is a greenfield Go project. No existing codebase to scout.

### Established Patterns
- None established yet. This phase sets the patterns all subsequent phases follow.

### Integration Points
- `~/.config/opencode` — the managed symlink; opm owns this path
- `~/.config/opm/` — opm's own state directory; created by `opm init`
- `~/.config/opm/profiles/<name>/` — profile directories
- `~/.config/opm/current` — plain text file: active profile name (one line)

</code_context>

<specifics>
## Specific Ideas

- Error messages follow docker/kubectl convention: entity names in double quotes, errors to stderr, success to stdout. Example: `context "foo" does not exist`
- `opm context show` must be fast (< 5ms) — reads from `current` file for shell prompt integration
- `opm context ls` success output example:
  ```
  * default
    work
    personal
  ```
- `rm --force` auto-switch message: `Switched to context "default". Removed context "work".` (two-line output)

</specifics>

<deferred>
## Deferred Ideas

- `--format json` and `-q` flags on `ls`/`inspect` — v2 requirement, no discussion needed in Phase 1
- `opm doctor` — Phase 2
- Shell completion — Phase 2
- Broken symlink detection in `ls` — Phase 2
- `opm context copy` — v2 backlog
- Profile size warning on `rm` — could add in Phase 2 alongside other safety features

</deferred>

---

*Phase: 01-foundation-core-cli*
*Context gathered: 2026-04-15*
