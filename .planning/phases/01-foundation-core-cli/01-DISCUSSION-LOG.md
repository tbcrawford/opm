# Phase 1: Foundation + Core CLI — Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-15
**Phase:** 01-Foundation + Core CLI
**Areas discussed:** rm --force behavior, ls output format, inspect output, Unmanaged dir handling

---

## rm --force behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Delete and leave dangling | Delete the dir, leave symlink dangling. User is responsible for running 'opm context use <other>' next. | |
| Auto-switch before delete | Before deleting, require another profile to exist and auto-switch to it (or 'default'). Error if no other profile available. | ✓ |
| Delete dir + remove symlink | Delete the dir AND remove the symlink. ~/.config/opencode no longer exists. OpenCode will reinitialize on next start. | |

**User's choice:** Auto-switch before delete

**Follow-up — switch target:**

| Option | Description | Selected |
|--------|-------------|----------|
| default first, then alphabetical | Switch to 'default' if it exists, otherwise pick the first available profile alphabetically. | ✓ |
| Error if no default exists | Error — user must explicitly run 'opm context use <name>' before force-removing the active profile. | |
| Error with suggestions | Error with list of available profiles so user knows what to switch to. | |

**User's choice:** default first, then alphabetical
**Notes:** If no other profile exists at all, rm --force errors: "Cannot remove the only profile. Create another profile first."

---

## ls output format

| Option | Description | Selected |
|--------|-------------|----------|
| Name + active marker only | NAME column only, active marked with *. One profile per line. Clean, fast, shell-prompt friendly. | ✓ |
| Name + path | NAME + PATH columns. User sees exactly where each profile lives on disk. | |
| Name + path + size hint | NAME + PATH + a count of items in the profile dir (e.g. '14 files'). Quick sanity check. | |

**User's choice:** Name + active marker only
**Notes:** Uses text/tabwriter for alignment. Active profile prefixed with `*`.

---

## inspect output

| Option | Description | Selected |
|--------|-------------|----------|
| Name + path + dir listing | Name, path, active status, and a first-level directory listing (agents/, plugins/, opencode.json etc). Shows what's installed. | ✓ |
| Name + path + active only | Name, path, active status only. Minimal. | |
| With model info from opencode.json | Name, path, active status, dir listing, plus opencode.json model field if it exists. | |

**User's choice:** Name + path + dir listing
**Notes:** No deep recursion, no model extraction from opencode.json in v1.

---

## Unmanaged dir handling

**Question: What should 'opm context use <name>' do if ~/.config/opencode is a real directory (not opm-managed)?**

| Option | Description | Selected |
|--------|-------------|----------|
| Error + suggest opm init | Error with clear message: '~/.config/opencode is not managed by opm. Run opm init first.' No silent takeover. | ✓ |
| Warn + confirm + migrate | Warn that it will take over, ask for confirmation, then migrate and switch in one step. | |
| Auto-migrate silently | Treat it the same as opm init — auto-migrate to default and then switch to the requested profile. | |

**User's choice:** Error + suggest opm init

**Follow-up — same for opm context create?**

| Option | Description | Selected |
|--------|-------------|----------|
| Same error as context use | '~/.config/opencode is a real directory, not managed by opm. Run opm init first.' | ✓ |
| Only init can take over | opm init is explicitly designed to take over. Any other command should error. | |

**User's choice:** Same error as context use
**Notes:** Silent auto-migration explicitly rejected — could destroy data without user awareness.

---

## the agent's Discretion

- Error message exact wording (beyond patterns established)
- go.mod minimum Go version
- Test coverage targets and table-driven test structure
- text/tabwriter column padding values

## Deferred Ideas

- `--format json` / `-q` flags — v2
- `opm doctor` — Phase 2
- Shell completion — Phase 2
- Broken symlink detection in `ls` — Phase 2
- `opm context copy` — v2 backlog
- Profile size warning on `rm` — Phase 2 candidate
