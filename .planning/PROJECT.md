# opm — OpenCode Profile Manager

## What This Is

A Go CLI tool that manages multiple OpenCode configurations by symlinking `~/.config/opencode` to named profile directories. Users switch between completely isolated OpenCode environments (different MCPs, plugins, agents, models, AGENTS.md) with a single command, using a UX modeled on `docker context`.

## Core Value

Switching OpenCode profiles should be one command — `opm context use <name>` — and take effect immediately without restarting anything.

## Requirements

### Validated

(None yet — ship to validate)

### Active

- [ ] User can initialize opm, migrating their existing `~/.config/opencode` into a named `default` profile
- [ ] User can create a new empty profile (OpenCode initializes it on first start)
- [ ] User can switch profiles via `opm context use <name>` (symlinks `~/.config/opencode` → profile dir)
- [ ] User can list profiles with the active one marked
- [ ] User can inspect a profile's details
- [ ] User can delete a profile
- [ ] Editing files in `~/.config/opencode/` while a profile is active directly edits that profile

### Out of Scope

- Env var / shell integration approach — symlinks are simpler and persistent across shells without shell config
- Syncing profiles across machines — local only for v1
- Profile "base" / inheritance — each profile is fully independent

## Context

OpenCode stores all global configuration in `~/.config/opencode/`, including:
- `opencode.json` — main config (model, MCPs, plugins, providers, permissions)
- `tui.json` — TUI settings (theme, keybinds)
- `AGENTS.md` — global AI instructions
- `agents/`, `commands/`, `skills/`, `plugins/` — custom tooling subdirs
- `node_modules/` — npm-installed plugins (e.g. GSD framework)

The entire directory is profile-scope — different contexts (work, personal, client-X) may have incompatible plugin sets (e.g. `superpowers` vs `get-shit-done`). Full directory isolation prevents conflicts.

OpenCode handles an empty config dir gracefully — it initializes defaults on first start. New profiles can be empty dirs.

**opm storage:**
- Profiles live at `~/.config/opm/profiles/<name>/`
- Active profile tracked in `~/.config/opm/current`
- `~/.config/opencode` is a symlink managed by opm

## Constraints

- **Language**: Go — single binary, fast, no runtime dependencies
- **Mechanism**: Directory-level symlink (`~/.config/opencode` → profile dir), not file-level
- **UX**: Mirror `docker context` command surface (create, use, ls, inspect, rm)
- **Compatibility**: Must not break existing OpenCode setup — `opm init` migrates current config non-destructively

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Symlink entire `~/.config/opencode/` dir | Full plugin/agent isolation between profiles. File-level symlinking can't isolate node_modules or plugin conflicts. | — Pending |
| Go binary | Single binary distribution, fast startup, no Node/Python runtime required | — Pending |
| Docker context UX | Familiar mental model for devs. `create`, `use`, `ls`, `inspect`, `rm` | — Pending |
| Empty new profiles | OpenCode initializes config on first start — no need to pre-populate | — Pending |
| `opm init` migrates existing config | Non-destructive onboarding — existing config becomes the `default` profile | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd-transition`):
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd-complete-milestone`):
1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-04-15 after initialization*
