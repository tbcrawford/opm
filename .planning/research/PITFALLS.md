# Domain Pitfalls: opm — Symlink-Based CLI Config Manager

**Domain:** Symlink-based config directory management in Go
**Researched:** 2026-04-15
**Confidence:** HIGH (verified against Go stdlib docs, POSIX syscall man pages, and OS behavior)

---

## Critical Pitfalls

Mistakes that cause data loss, silent corruption, or broken state that is hard to recover from.

---

### Pitfall 1: `os.Symlink` Fails If Target Already Exists (EEXIST)

**What goes wrong:**
`os.Symlink(src, dst)` returns `EEXIST` if `dst` already exists — even if `dst` is a dangling symlink or an existing symlink pointing elsewhere. There is no "replace if exists" variant. Callers who ignore this error or assume idempotency will silently leave the old symlink in place.

**Why it happens:**
The POSIX `symlink(2)` syscall makes no provision for overwriting. Unlike files, there is no `O_TRUNC` equivalent for symlinks. The Go stdlib passes this error through directly as `*os.LinkError`.

**Concrete failure scenario for opm:**
`opm context use work` is called. `~/.config/opencode` already points to `profiles/personal`. Without explicitly removing the old symlink first, `os.Symlink` will fail — and if that error is mishandled, the config stays on `personal` while `current` is updated to `work`. State is now inconsistent.

**Consequences:**
- State file (`current`) says one profile; actual symlink points to another
- Silent: no visible error if the error is swallowed
- OpenCode launches with the wrong profile, user edits wrong profile's files

**Prevention:**
Use the atomic swap pattern:
```go
// Atomic profile switch — never leaves a dangling window
tmp := dst + ".opm-tmp"
os.Remove(tmp)                        // clean up any leftover tmp
os.Symlink(profilePath, tmp)          // create new symlink at temp name
os.Rename(tmp, dst)                   // atomic replace (POSIX guarantee)
```
`os.Rename` on the same filesystem is atomic per POSIX (man 2 rename: "guarantees that an instance of new will always exist, even if the system should crash in the middle"). Both `tmp` and `dst` must be on the same filesystem — they will be (both under `~/.config/`).

**Warning signs:**
- Any code that calls `os.Symlink` without first calling `os.Remove` on the target
- Any code that calls `os.Symlink` and checks `os.IsExist(err)` as non-fatal

**Phase:** Core symlink management (Phase 1 / foundation)

---

### Pitfall 2: `opm init` Moves the Directory Before the Symlink Is Created

**What goes wrong:**
`opm init` must: (1) rename `~/.config/opencode` → `~/.config/opm/profiles/default`, then (2) create the symlink `~/.config/opencode` → new location. If the process dies, is killed, or the filesystem errors between step 1 and step 2, the user is left with no `~/.config/opencode` at all — a completely missing config directory. OpenCode will create a blank config on next start, silently discarding everything the user had.

**Why it happens:**
There is no transaction primitive spanning two directories. Move and symlink-create are separate syscalls.

**Concrete failure scenario:**
User runs `opm init`. Power goes out after the rename but before the symlink. On restart: `~/.config/opencode` does not exist. OpenCode starts fresh. User loses all MCP configurations, plugins, AGENTS.md.

**Consequences:** Permanent data loss if user doesn't notice before OpenCode reinitializes.

**Prevention:**
Use a 3-step order that makes every intermediate state recoverable:
```
Step 1: Create symlink at ~/.config/opencode.opm-new → profiles/default  (new temp symlink)
Step 2: Move ~/.config/opencode → profiles/default  (move real dir to profile store)
Step 3: os.Rename(~/.config/opencode.opm-new, ~/.config/opencode)  (atomic symlink install)
```
Before step 2, detect and handle partially-complete prior runs:
- If `profiles/default` exists AND `~/.config/opencode` is a symlink → already done, skip
- If `profiles/default` exists AND `~/.config/opencode` is a real dir → step 2 already happened, retry from step 3
- If `profiles/default` does not exist AND `~/.config/opencode` is missing → prior crash during step 2; the only unrecoverable case

**Warning signs:**
- `opm init` moves the directory and symlinks in a single sequential block without crash-recovery logic
- No idempotency check at the start of `init`

**Phase:** `opm init` command (Phase 1)

---

### Pitfall 3: Deleting an Active Profile Leaves a Dangling Symlink

**What goes wrong:**
`opm context rm work` while `work` is the active profile removes `~/.config/opm/profiles/work/` but leaves `~/.config/opencode` pointing to the now-deleted directory. The symlink becomes dangling. OpenCode launches, finds the config dir missing, silently initializes a blank one *inside the dangling symlink path* — which fails because the symlink target doesn't exist.

**Why it happens:**
`os.Remove` on the profile directory removes the target; the symlink itself has no awareness of this and is not updated.

**Concrete failure scenario:**
```
$ opm context rm work   # deletes ~/.config/opm/profiles/work/
$ opencode              # follows ~/.config/opencode → deleted dir → ENOENT
                        # OpenCode may crash or create blank state
```

**Consequences:**
- OpenCode fails to start or starts with blank state
- User confused because `opm context ls` may show nothing as active (symlink is dangling)
- If user creates a new profile named `work`, it gets the same path — appearing to "restore" the deleted profile with empty content

**Prevention:**
In `opm context rm`:
1. Check if the profile being deleted is active: `os.Readlink("~/.config/opencode")` and compare to profile path
2. If active: refuse deletion with a clear error: `"Cannot remove active profile 'work'. Switch to another profile first with: opm context use <name>"`
3. If not active: proceed with deletion

**Warning signs:**
- `rm` subcommand does not call `os.Readlink` to check current active target before deleting

**Phase:** `opm context rm` command (Phase 1)

---

### Pitfall 4: Using `os.Stat` Instead of `os.Lstat` to Detect Symlink State

**What goes wrong:**
`os.Stat` follows symlinks. `os.Lstat` does not. If `~/.config/opencode` is a dangling symlink, `os.Stat` returns `ENOENT` (target not found). Code using `os.Stat` to check "does `~/.config/opencode` exist?" cannot distinguish between three different states:
- (A) Path does not exist at all
- (B) Path is a dangling symlink (target missing)
- (C) Path is an opm-managed symlink (normal operation)

**Why it happens:**
Go's `os.Stat` silently follows symlinks, consistent with Unix convention. Developers unfamiliar with the distinction reach for `os.Stat` by default.

**Concrete failure scenario:**
`opm init` checks: "does `~/.config/opencode` exist?" using `os.Stat`. On a machine where a previous init crashed and left a dangling symlink, `os.Stat` returns `ENOENT`. The code treats this as "no config exists, safe to proceed" — and overwrites the dangling symlink without cleaning up properly.

**Prevention:**
Use `os.Lstat` everywhere opm inspects `~/.config/opencode`. The decision tree:
```go
fi, err := os.Lstat(configPath)
if os.IsNotExist(err) {
    // Path does not exist at all — safe state
} else if fi.Mode()&os.ModeSymlink != 0 {
    // Is a symlink — check if opm-managed
    target, _ := os.Readlink(configPath)
    if strings.HasPrefix(target, opmProfilesDir) {
        // opm-managed symlink
    } else {
        // foreign symlink — warn user
    }
} else if fi.IsDir() {
    // Real directory — needs init migration
} else {
    // Unexpected file type — error
}
```

**Warning signs:**
- Any call to `os.Stat(configPath)` in opm's detection/init logic
- Missing `os.ModeSymlink` check in file mode inspection

**Phase:** All commands that inspect `~/.config/opencode` (Phase 1, applies everywhere)

---

### Pitfall 5: Relative vs. Absolute Symlink Target — macOS Finder Breaks Relative

**What goes wrong:**
Symlinks can point to relative or absolute paths. Relative symlinks are resolved relative to the directory containing the symlink, not the current working directory. If `~/.config/opencode` is a symlink with a relative target like `../opm/profiles/default`, it resolves correctly from a shell (`~/.config/` + `../opm/profiles/default` = `~/.config/opm/profiles/default`). However:
- macOS Finder and some GUI apps resolve relative symlinks from their own working directory, not the symlink's directory — producing wrong paths
- If the user moves `~/.config/` (rare but possible), relative symlinks break; absolute ones remain valid
- Relative targets are harder to validate with `os.Readlink` + comparison logic

**Why it happens:**
Relative symlinks look "portable" but have subtle resolution rules that differ between tools.

**Prevention:**
Always use **absolute paths** for symlink targets:
```go
profilePath := filepath.Join(os.UserHomeDir(), ".config", "opm", "profiles", name)
os.Symlink(profilePath, configPath)  // absolute target
```
Use `filepath.Abs()` when accepting any user-provided path before storing it.

**Warning signs:**
- `os.Symlink` called with a relative target derived from `filepath.Rel`
- Profile paths constructed without anchoring to `os.UserHomeDir()`

**Phase:** Symlink creation logic (Phase 1)

---

## Moderate Pitfalls

Mistakes that produce visible but recoverable problems.

---

### Pitfall 6: `os.Rename` Across Filesystems Fails Silently in Some Cases

**What goes wrong:**
The atomic swap pattern (`os.Symlink(tmp) + os.Rename(tmp, dst)`) requires both paths to be on the same filesystem. On macOS, `~/.config/` is almost always on the same APFS volume, but in containerized environments or with unusual mount configurations (e.g., Docker-in-Docker, home on NFS), this assumption breaks. `os.Rename` across filesystems returns `EXDEV` (invalid cross-device link).

**Prevention:**
Detect `EXDEV` explicitly:
```go
err := os.Rename(tmp, dst)
if err != nil {
    var linkErr *os.LinkError
    if errors.As(err, &linkErr) && linkErr.Err == syscall.EXDEV {
        // Fall back: remove old symlink, create new one (non-atomic)
        os.Remove(dst)
        os.Symlink(target, dst)
    }
}
```
For opm's use case (both paths under `~/.config/`), EXDEV will be extremely rare in practice. But the fallback prevents a hard crash.

**Phase:** Core symlink swap utility (Phase 1)

---

### Pitfall 7: `opm context ls` Shows Wrong Active Profile After External Interference

**What goes wrong:**
`opm context ls` reads `~/.config/opm/current` to mark the active profile. But the user (or another tool) may have manually changed the symlink without updating `current`. The displayed "active" profile is now stale.

**Why it happens:**
Two sources of truth: the `current` file and the actual symlink target. They can diverge.

**Prevention:**
Make `current` derived, not authoritative. Read the actual symlink target at display time:
```go
target, err := os.Readlink(configPath)
if err != nil {
    // dangling or not a symlink
}
activeProfile := filepath.Base(target)  // derive from symlink, not current file
```
Write `current` only as a convenience for scripts/shell prompts that don't want to call `readlink`. Always resolve from the actual symlink for UI display.

**Warning signs:**
- `opm context ls` only reads `current` without also calling `os.Readlink` to verify

**Phase:** `ls` command (Phase 1), also affects any prompt integration

---

### Pitfall 8: Profile Name Colliding with Filesystem-Reserved Names

**What goes wrong:**
Profile names become directory names under `~/.config/opm/profiles/<name>/`. Names like `.`, `..`, names with `/`, names with null bytes, or names exceeding 255 characters are either invalid or dangerous on POSIX filesystems. A profile named `../../../etc` would create a directory outside the profiles store.

**Prevention:**
Validate profile names at creation time with an allowlist pattern:
```go
var validName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`)
if !validName.MatchString(name) {
    return fmt.Errorf("invalid profile name %q: must match [a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}", name)
}
```
Mirror docker context naming rules (alphanumeric, hyphens, underscores, dots, ≤63 chars).

**Phase:** Input validation, `create` and `init` commands (Phase 1)

---

### Pitfall 9: `opm init` on a Machine Where OpenCode Has Never Run

**What goes wrong:**
`~/.config/opencode` may not exist at all on a fresh machine. `opm init` must handle this — creating the `default` profile directory and the symlink without copying from a source that doesn't exist. The common mistake: the init code checks "is it a dir?" and branches to "migrate it" — but if the path doesn't exist, `os.Rename` will fail with ENOENT.

**Prevention:**
Explicit three-way branch at init time:
```
Case A: ~/.config/opencode does not exist
  → mkdir profiles/default (empty)
  → create symlink
  → done (OpenCode will initialize on first run)

Case B: ~/.config/opencode is a real directory
  → rename to profiles/default
  → create symlink (migrate existing config)

Case C: ~/.config/opencode is already an opm symlink
  → already initialized, error: "opm is already initialized"
```

**Phase:** `opm init` (Phase 1)

---

### Pitfall 10: `git` Inside `~/.config/opencode` Sees Symlink as Root

**What goes wrong:**
If a user runs `git init` inside `~/.config/opencode/` (to version their config), git will operate on the real profile directory (since it follows symlinks). This is the desired behavior. However, `git status` in the *actual profile directory* path (`~/.config/opm/profiles/default/`) will show the same repo. No breakage — just potential user confusion when they see two paths referencing the same files.

More critically: `git` will **not** track the symlink itself — it tracks the content inside the profile directory. If the user runs `git` in `~/.config/` expecting to track the symlink `opencode → ...`, git will not see it unless `.config/` itself is a git repo.

**Prevention:**
Document in help text: "Your profile's git history tracks content, not the symlink. `opm` manages the symlink." No code prevention needed — this is a UX clarity issue.

**Phase:** Documentation / help text (Phase 1 or 2)

---

### Pitfall 11: `node_modules` Inside Profiles — `rm -rf` During Delete Is Destructive

**What goes wrong:**
Profiles contain `node_modules/` (npm-installed OpenCode plugins). When `opm context rm <name>` deletes a profile, `os.RemoveAll` will recursively delete `node_modules/` — which may contain large plugin installations. This is expected behavior, but users who assume "rm just removes the opm record" will be surprised. No undo.

**Prevention:**
`opm context rm` should:
1. Print the profile path and a size estimate before deletion
2. Require `--force` or confirmation: `"This will permanently delete ~/.config/opm/profiles/work/ including all plugins. Continue? [y/N]"`
3. Never allow deletion of the active profile (see Pitfall 3)

**Phase:** `opm context rm` (Phase 1)

---

## Minor Pitfalls

Friction points that are annoying but don't cause data loss.

---

### Pitfall 12: `ls -la ~/.config/` Showing Arrow Notation Confuses Users

**What goes wrong:**
`ls -la ~/.config/` shows `opencode -> /Users/you/.config/opm/profiles/work` — users who don't understand symlinks may be confused or alarmed. Not a bug, but a support burden.

**Prevention:** Add `opm doctor` or verbose output in `opm context ls` that explains the symlink. Document the expected `ls` output in the README.

**Phase:** Polish / docs (Phase 2+)

---

### Pitfall 13: macOS Spotlight Indexing the Profile Store

**What goes wrong:**
macOS Spotlight will index both `~/.config/opencode/` (via the symlink) and `~/.config/opm/profiles/*/` directly, potentially indexing plugin content twice. This is harmless but wastes index space. More importantly, Spotlight follows symlinks, so indexed paths via the symlink may show `opencode/` paths that suddenly point to different content after a profile switch — causing stale Spotlight results.

**Prevention:** Add a `.metadata_never_index` file to `~/.config/opm/` on macOS if this becomes a user complaint. Not worth preemptive implementation.

**Phase:** Not worth addressing in v1

---

### Pitfall 14: Shell Tab Completion of Paths Inside `~/.config/opencode/` After Switch

**What goes wrong:**
Some shells cache directory completion paths. After `opm context use <name>`, the shell's completion may briefly show files from the old profile if it has cached the directory listing. This resolves on next tab-press but looks broken immediately after a switch.

**Prevention:** None needed — this is a shell behavior, not an opm bug. Document that a new shell or `rehash` may be needed for completion cache refresh.

**Phase:** Not addressable

---

## Phase-Specific Warnings

| Phase Topic | Likely Pitfall | Mitigation |
|-------------|---------------|------------|
| `opm init` implementation | Crash mid-migration leaves no config dir (Pitfall 2) | 3-step order with crash recovery check |
| `opm init` + `os.Stat` detection | Cannot detect dangling symlink (Pitfall 4) | Always use `os.Lstat` |
| `opm context use` symlink swap | EEXIST on re-use (Pitfall 1) | Atomic rename pattern |
| `opm context rm` implementation | Deleting active profile → dangling symlink (Pitfall 3) | Refuse deletion of active profile |
| Profile naming | Path traversal via profile name (Pitfall 8) | Allowlist validation on create |
| `opm context ls` accuracy | Stale `current` file (Pitfall 7) | Derive active from `os.Readlink` |
| `opm context rm` UX | Silent `node_modules` destruction (Pitfall 11) | Require `--force` + size warning |
| Symlink target format | Relative symlinks broken by GUI tools (Pitfall 5) | Always use absolute paths |
| Fresh machine / no OpenCode | Missing source dir for init (Pitfall 9) | Explicit 3-case branch |
| Cross-filesystem edge cases | `EXDEV` on `os.Rename` (Pitfall 6) | Detect and fall back gracefully |

---

## Sources

- Go `os` package: `os.Symlink`, `os.Rename`, `os.Lstat`, `os.Readlink` — https://pkg.go.dev/os (go1.26.2, verified 2026-04-15)
- POSIX `rename(2)` atomicity guarantee — macOS man page (verified locally 2026-04-15)
- POSIX `symlink(2)` EEXIST behavior — macOS man page (verified locally 2026-04-15)
- `os.Rename` note: "rename guarantees that an instance of new will always exist, even if the system should crash" — confirmed from macOS man 2 rename
- chezmoi issues (searched, no relevant symlink-specific issues in closed/open state as of 2026-04-15)
- Domain expertise from dotfile manager ecosystem (stow, chezmoi, yadm, mackup — common failure modes)
