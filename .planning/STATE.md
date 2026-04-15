# opm — Project State

## Project Reference

**Core value**: `opm context use <name>` switches OpenCode profiles instantly — one command, no restart
**Milestone**: v1 (initial release)
**Roadmap**: 3 phases

---

## Current Position

**Phase**: 1 — Foundation + Core CLI
**Plan**: None started
**Status**: Not started
**Progress**: ░░░░░░░░░░░░░░░░░░░░ 0%

---

## Performance Metrics

| Metric | Value |
|--------|-------|
| Phases complete | 0/3 |
| Plans complete | 0/0 |
| Requirements done | 0/16 |

---

## Accumulated Context

### Decisions

- Use `os.UserHomeDir()` + `filepath.Join` to construct `~/.config/opm` and `~/.config/opencode` — NOT `os.UserConfigDir()` (returns `~/Library/Application Support` on macOS)
- Atomic symlink swap: `os.Symlink(target, tmp)` then `os.Rename(tmp, dst)` — never remove-then-create
- `opm init` crash safety: 3-step sequence — (1) create temp symlink, (2) move real dir, (3) atomic rename symlink into place
- Active profile derived from `os.Readlink()` on the actual symlink, not from `current` file (`current` is a fast-path cache only)
- Profile name validation allowlist: `^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`
- `opm context rm --force` behavior on active profile: TBD (decide in Phase 1 planning)

### Open Questions

- `opm context rm --force` on active profile: switch to `default` first, or leave dangling symlink?
- Profile size warning on `rm`: implement `filepath.Walk` sum, or just warn by path?

### Blockers

*(None)*

### Key Todos

*(Set by plan agent)*

---

## Session Continuity

**Last updated**: 2026-04-15 (roadmap created)
**Next action**: `/gsd-plan-phase 1`
