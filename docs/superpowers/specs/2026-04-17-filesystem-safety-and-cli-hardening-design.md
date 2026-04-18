# Filesystem Safety and CLI Hardening — Design Spec

**Date:** 2026-04-17  
**Status:** Approved

## Goal

Harden `opm`'s filesystem and symlink handling so profile management only trusts verified in-store targets, fix the reviewed command-layer correctness issues around broken state and interrupted recovery, and align custom help output with the actual CLI surface.

## Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Safety architecture | Centralize managed-symlink resolution in `internal/store` | One place decides whether `~/.config/opencode` is truly opm-managed |
| Managed target check | Resolve relative links + normalize + verify containment with `filepath.Rel` | Raw string-prefix checks are not safe enough |
| Active profile source of truth | Managed, non-dangling symlink only | Prevent foreign or dangling links from being treated as healthy state |
| `current` cache write failures | Warning after successful symlink change | The symlink is authoritative; do not report full command failure after success |
| Atomic symlink temp path | Unique temp symlink per call | Avoid collisions between concurrent operations |
| Help output | Keep custom help, but include `completion` | README and CLI help must agree |

## Architecture

### Managed symlink resolution in `internal/store`

Add a private helper in `internal/store/store.go` that resolves the state of `s.opencodeDir` into verified managed-state information. This helper is the only code path allowed to classify a symlink as opm-managed.

The helper must:

1. Inspect `s.opencodeDir` without following it.
2. Return early for non-symlink or missing paths.
3. Read the raw symlink target.
4. Resolve relative targets against `filepath.Dir(s.opencodeDir)`.
5. Normalize the target path with `filepath.Clean`.
6. Verify containment under `s.profilesDir()` using `filepath.Rel`, not `strings.HasPrefix`.
7. Derive the profile name only after containment is proven.
8. Detect whether the resolved target is dangling.

The helper should expose enough internal state for store methods to answer these questions consistently:

- does the link exist?
- is it a symlink?
- is it opm-managed?
- what resolved profile path does it point to?
- what profile name does that imply?
- is the target dangling?

This remains an internal store concern. No new exported package or cross-package abstraction is needed.

### Store methods built on the resolver

The following methods must delegate to the shared resolver instead of making their own trust decisions:

- `IsOpmManaged()` returns true only for verified managed symlinks.
- `ActiveProfile()` returns the profile name only when the link is both managed and non-dangling.
- `ListProfiles()` marks active profiles only from verified managed state.
- `ListProfiles()` may synthesize a dangling active entry only when the link is verified managed but its target directory is missing.
- `Reset()` copies from the resolved managed target only after the same verification succeeds.

Store methods that currently trust callers for path safety must harden themselves:

- `GetProfile(name)` validates `name` before joining paths.
- `DeleteProfile(name, force)` validates `name` before touching the filesystem.
- `CopyProfile(src, dst)` validates both names and rejects a symlink or non-directory source root before recursive copy begins.

### Command behavior built on safer store state

Commands should consume the hardened store behavior rather than reproducing filesystem logic themselves.

#### `opm show`

`show` must stop treating a dangling symlink target basename as a valid active profile.

Behavior:

- if the symlink is managed and healthy, print the active profile name
- if the symlink is managed but dangling or absent, warn on stderr and fall back to `current` when available
- if neither a healthy symlink nor cached current name exists, return a clear error
- if `~/.config/opencode` is not opm-managed, keep the existing not-managed error behavior

#### `opm init`

`init` must treat unexpected pre-existing filesystem state as unsafe instead of trying to continue.

Behavior:

- if `~/.config/opencode` exists as a regular file or other non-directory, non-symlink path, fail with a recovery message instead of replacing it
- the interrupted-init resume path only succeeds when `~/.config/opencode.opm-new` exists and is the exact expected temp symlink to the requested profile directory
- stale, foreign, or malformed `.opm-new` artifacts cause a recovery error rather than being renamed into place

#### Commands that update `current`

`use`, `rename`, forced `remove`, and the successful branches of `init` should treat `SetCurrent` failure as a warning after a successful symlink change.

Behavior:

- if the symlink swap fails, return an error
- if the symlink swap succeeds but `current` cannot be written, print a warning to stderr and return success
- warning text should make it clear that the active profile changed successfully and only the cache file update failed

This keeps command exit status aligned with the true source of state.

### Atomic symlink implementation

`internal/symlink.SetAtomic` keeps the existing create-temp-plus-rename pattern, but the temporary link name must be unique per call.

Requirements:

- temp link lives in the destination directory so `os.Rename` stays atomic
- temp name includes enough uniqueness to avoid collisions between concurrent calls
- best-effort cleanup still happens on failure

No locking is introduced in this pass. Unique temp names are enough to eliminate the reviewed collision risk.

### Help and discoverability

Custom root help remains in place, but it must reflect the real Cobra command tree closely enough that documented commands are discoverable.

In this pass, root help must include `completion` so the CLI surface shown by `opm --help` matches the README and actual runnable commands.

## File Responsibilities

- `internal/store/store.go`
  - add the private managed-symlink resolver
  - route managed-state decisions through it
  - harden `GetProfile`, `DeleteProfile`, and `CopyProfile`
- `internal/symlink/symlink.go`
  - switch `SetAtomic` to unique temp link names
- `cmd/show.go`
  - use safer active-profile behavior and fallback rules
- `cmd/init.go`
  - reject unexpected pre-existing file state and validate `.opm-new` before recovery
- `cmd/use.go`
  - warn instead of failing when only `current` cache update fails
- `cmd/remove.go`
  - same cache-write warning behavior after successful auto-switch
- `cmd/rename.go`
  - same cache-write warning behavior after successful active-profile rename update
- `cmd/help.go`
  - include `completion` in root help output
- `cmd/cmd_test.go`
  - command regression tests for `show`, `init`, help output, and cache warning behavior
- `internal/store/store_test.go`
  - regression tests for containment, validation, and copy safety
- `internal/symlink/symlink_test.go`
  - regression tests for unique-temp concurrent safety

## Test Plan

All reviewed issues must get regression coverage before production code changes.

### `cmd/cmd_test.go`

Add tests for:

- `show` warning + fallback behavior when the symlink is dangling
- `show` failure behavior when there is no healthy symlink and no cached current profile
- `init` refusing a pre-existing regular file at the managed path
- `init` refusing a stale or incorrect `.opm-new`
- root help output including `completion`
- command success with stderr warning when `SetCurrent` fails after a successful switch/update

### `internal/store/store_test.go`

Add tests for:

- `IsOpmManaged` rejecting symlink targets that escape `profiles/` via `..`
- `IsOpmManaged` handling relative symlink targets correctly
- `ActiveProfile` returning empty for foreign or dangling symlinks
- `GetProfile` rejecting traversal names internally
- `DeleteProfile` rejecting traversal names internally
- `CopyProfile` rejecting a symlink source root
- `Reset` refusing forged managed-looking targets outside `profiles/`

### `internal/symlink/symlink_test.go`

Add a concurrency-oriented test that calls `SetAtomic` in parallel and asserts that:

- no call fails because of shared temp-name collision
- the final symlink points to one of the requested valid targets
- no `.opm-tmp-...` artifact remains afterward

## Out of Scope

- introducing file locking across commands
- adding new public CLI commands beyond surfacing existing `completion`
- changing the flat command surface
- changing how profile switching fundamentally works
- adding JSON output or other new scripting modes

## Success Criteria

- opm only treats `~/.config/opencode` as managed when the resolved target is actually inside `profiles/`
- dangling links are no longer reported as healthy active profiles by `show`
- `init` no longer risks overwriting a regular file or trusting stale temp symlinks
- cache-file write failures no longer cause false command failures after successful symlink updates
- concurrent `SetAtomic` calls no longer collide on a shared temp name
- `opm --help` shows `completion`
- the new regression tests pass along with the existing full suite
