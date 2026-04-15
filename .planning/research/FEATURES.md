# Feature Landscape

**Domain:** CLI config profile / context manager
**Project:** opm — OpenCode Profile Manager
**Researched:** 2026-04-15
**Research Mode:** Ecosystem — Features dimension
**Overall Confidence:** HIGH (primary evidence from live CLI inspection of docker, kubectl, gcloud, gh, terraform)

---

## Research Methodology

All feature patterns verified by directly running these tools on the local machine:

| Tool | Command Surface Inspected |
|------|--------------------------|
| `docker context` | `create`, `use`, `ls`, `inspect`, `rm`, `show`, `export`, `import`, `update`; actual table/JSON output |
| `kubectl config` | `use-context`, `get-contexts`, `current-context`, `rename-context`; output formats |
| `gcloud config configurations` | `activate`, `create`, `delete`, `describe`, `list`, `rename`; `--format=json` |
| `terraform workspace` | `select`, `new`, `list`, `delete`, `show` |
| `gh auth` | `switch`, `status` |
| `cobra-cli` | completion generation (`bash`, `zsh`, `fish`, `powershell`) |

---

## Command Surface Analysis

### Universal Commands (every tool has them)

| Command | docker | kubectl | gcloud | terraform | opm equivalent |
|---------|--------|---------|--------|-----------|----------------|
| Create | `context create <name>` | `config set-context <name>` | `configurations create <name>` | `workspace new <name>` | `context create <name>` |
| Switch | `context use <name>` | `config use-context <name>` | `configurations activate <name>` | `workspace select <name>` | `context use <name>` |
| List | `context ls` | `config get-contexts` | `configurations list` | `workspace list` | `context ls` |
| Delete | `context rm <name>` | `config delete-context <name>` | `configurations delete <name>` | `workspace delete <name>` | `context rm <name>` |
| Show current | `context show` | `config current-context` | — | `workspace show` | `context show` |
| Inspect | `context inspect <name>` | — | `configurations describe <name>` | — | `context inspect <name>` |

### Less Universal Commands (2 of 4 tools)

| Command | Present In | opm relevance |
|---------|-----------|---------------|
| Rename | kubectl, gcloud | USEFUL — renaming a profile is a natural workflow (e.g., `work` → `work-client-x`) |
| Copy / Clone | docker (`--from`), gcloud | USEFUL — clone a profile as a starting point for a new one |
| Export | docker only | SKIP v1 — only meaningful for network/endpoint configs, not file directories |
| Import | docker only | SKIP v1 — same rationale |
| Update / Edit description | docker only | LOW PRIORITY — description field is cosmetic; profiles are just directories |

---

## Table Stakes

Features users expect. Missing = product feels incomplete or broken.

### TS-1: `opm context create <name>` — Create a new empty profile

**What it does:** Creates a new directory `~/.config/opm/profiles/<name>/`, no content (OpenCode populates on first start).

**Why expected:** Every context manager has it. Users will immediately try `opm context create work`.

**UX conventions observed:**
- Takes exactly one positional argument (name)
- Fails with clear error if name already exists (no silent overwrite)
- Does NOT automatically switch to the new profile (docker, kubectl both leave current unchanged)
- Naming convention: lowercase, hyphens, no spaces (validate at creation time)

**Complexity:** Low  
**Dependencies:** None  
**Error cases:** Name already exists → non-zero exit + `context "name" already exists`

---

### TS-2: `opm context use <name>` — Switch active profile

**What it does:** Replaces `~/.config/opencode` symlink to point at `~/.config/opm/profiles/<name>/`. Writes profile name to `~/.config/opm/current`.

**Why expected:** The core value proposition of the tool. Without this, there's nothing.

**UX conventions observed (from docker `context use`):**
- Success message: `Current context is now "name"` (docker's exact wording; adopt this)
- Non-zero exit + error on missing profile (docker: `context "x": context not found`)
- Takes effect immediately — no restart required (symlink swap is atomic)
- **Atomic swap pattern:** Write to temp symlink, then `os.Rename()` (avoids broken state)

**Complexity:** Low-Medium (atomic symlink swap + state file write)  
**Dependencies:** Profiles must exist (TS-1)  
**Error cases:**
- Profile does not exist → `context "name" does not exist` + exit 1
- Symlink target path not writable → filesystem error
- Already using this profile → succeed silently (idempotent)

---

### TS-3: `opm context ls` — List all profiles

**What it does:** Prints all profiles with the active one marked.

**Why expected:** Users need to see what profiles exist. Every tool has this.

**UX conventions observed:**
- **Active marker:** `*` suffix in same NAME column (docker style: `orbstack *`) OR separate CURRENT column with `*` (kubectl style). Recommend: docker style (more compact, less noise for a single-purpose tool)
- **Columns:** NAME, CREATED (optional), DESCRIPTION (if supported) — for opm, just NAME and maybe OPENCODE_DIR (path to resolved profile dir)
- **Quiet mode (`-q`, `--quiet`):** Prints only names, one per line — used for scripting and tab completion piping
- **JSON output (`--format json`):** Machine-readable version of same data (docker, gcloud both support this)
- Header row: YES (matches docker, gcloud, kubectl)

**Columns for opm:**

```
NAME         ACTIVE   PATH
default      *        ~/.config/opm/profiles/default
work                  ~/.config/opm/profiles/work
```

Or docker-style (name + asterisk in one column):

```
NAME          PATH
default *     ~/.config/opm/profiles/default
work          ~/.config/opm/profiles/work
```

**Complexity:** Low  
**Dependencies:** Profiles must exist (TS-1)  
**Error cases:** No profiles → empty output (not an error); `~/.config/opm/` doesn't exist → suggest `opm init`

---

### TS-4: `opm context rm <name>` — Delete a profile

**What it does:** Removes `~/.config/opm/profiles/<name>/` directory and all contents.

**Why expected:** Cleanup is required. Every tool has this.

**UX conventions observed:**
- **Guard against deleting the active profile** (docker: `context "orbstack" is in use, set -f flag to force remove` → exit 1)
- **`--force` / `-f` flag:** Allows deletion of active profile (with implicit switch to `default` or prompt)
- **No confirmation prompt** (docker, kubectl both delete without prompting) — rely on `--force` for destructive guard
- Accepts multiple names: `opm context rm work client-x` (docker supports this)
- Cannot delete the only remaining profile (would leave opm in invalid state)

**Complexity:** Low  
**Dependencies:** Profiles must exist (TS-1); not the active profile (without `--force`)  
**Error cases:**
- Profile does not exist → `context "name" does not exist` + exit 1
- Profile is active (without `-f`) → error + hint to use `-f`
- Deleting last remaining profile → error

---

### TS-5: `opm context show` — Print current profile name

**What it does:** Prints just the current profile name to stdout.

**Why expected:** Needed for scripting and shell prompt integration. docker has `docker context show`, terraform has `terraform workspace show`, kubectl has `kubectl config current-context`.

**UX conventions observed:**
- Outputs just the name, no decoration — `default\n`
- Exit 0 even if symlink target doesn't exist (docker behavior) — callers check validity separately
- `--format json` would output `{"name": "default"}`

**Complexity:** Trivial  
**Dependencies:** `~/.config/opm/current` state file  
**Error cases:** State file missing → error (suggest `opm init`)

---

### TS-6: `opm init` — Bootstrap opm from existing OpenCode config

**What it does:** One-time migration. Moves `~/.config/opencode/` to `~/.config/opm/profiles/default/`, creates the symlink `~/.config/opencode → ~/.config/opm/profiles/default/`, writes `current = default`.

**Why expected:** Without this, a user with an existing OpenCode setup can't adopt opm without losing their config. The non-destructive migration path is the entire adoption story.

**UX conventions observed (no direct analogs — opm-specific init):**
- Must be idempotent — running twice shouldn't break anything
- Should detect if already initialized and give clear message
- Should handle the case where `~/.config/opencode` is already a symlink (don't re-migrate)

**Complexity:** Medium (careful error handling, must not data-lose)  
**Dependencies:** None (first command user runs)  
**Error cases:**
- `~/.config/opencode` doesn't exist → create opm structure, no migration needed
- Already initialized → `opm is already initialized (active: default)` + exit 0
- `~/.config/opencode` is already a symlink pointing OUTSIDE opm → warn user, abort

---

### TS-7: `opm context inspect <name>` — Show profile details

**What it does:** Shows detailed info about a profile — path, created date, symlink status, whether it's active.

**Why expected:** docker has `docker context inspect`, gcloud has `configurations describe`. Users want to verify what a profile contains.

**UX conventions observed:**
- Default output: pretty-printed (JSON or YAML-style key-value block)
- `--format json` for machine-readable (docker outputs JSON by default for inspect)
- When no name given → inspect current profile (some tools do this, useful)
- Includes symlink resolution path so users can verify the directory

**Output format (recommended):**
```
Name:     work
Active:   false
Path:     /Users/user/.config/opm/profiles/work
Contents: 12 files, 3 directories
```

**Complexity:** Low  
**Dependencies:** Profiles must exist (TS-1)  
**Error cases:** Profile does not exist → error + exit 1

---

### TS-8: Shell Completion — Tab-complete profile names

**Why expected:** Any professional CLI tool has tab completion. Without it, users will make typos on profile names. Tab completion for profile names is the entire reason `docker context use <TAB>` works.

**UX conventions observed:**
- Cobra's `ValidArgsFunction` provides dynamic completion (returns current profile list)
- Generated scripts for bash, zsh, fish, powershell via `opm completion <shell>`
- Completion for `context use`, `context rm`, `context inspect` should return profile names
- `opm completion bash` should print installable completion script
- Convention: `eval "$(opm completion zsh)"` in `.zshrc`

**Complexity:** Low (Cobra handles all the heavy lifting via `ValidArgsFunction`)  
**Dependencies:** `opm context ls` must work (completion reads same data source)

---

### TS-9: Machine-readable output — `--format json`

**Why expected:** Scripting users pipe output to `jq`. Docker, gcloud, gh all support it. Without it, `opm context ls` output is not script-safe.

**UX conventions observed:**
- `--format json` flag on `ls` and `inspect` (docker pattern)
- `--format table` is the default
- `--quiet` / `-q` on `ls` for name-only list (already noted in TS-3)
- gh uses `--json <fields>` + `--jq <expr>` for composable filtering; **overkill for v1**

**Complexity:** Low (marshal to JSON; stdlib `encoding/json`)  
**Dependencies:** TS-3 (ls), TS-7 (inspect)

---

### TS-10: Non-zero exit codes on errors

**Why expected:** Unix convention. Every tool tested exits non-zero on error. Scripts break silently if exit codes aren't right.

**UX conventions observed:**
- All tools: exit 1 on all errors (not a numeric error code system)
- Error text to stderr, not stdout
- Success output to stdout

**Complexity:** Trivial — Go `os.Exit(1)` pattern  
**Dependencies:** All commands

---

## Differentiators

Features not expected, but that would make opm demonstrably better than a naive implementation.

### D-1: Broken symlink detection in `opm context ls`

**What it does:** If `~/.config/opencode` points to a profile directory that was manually deleted, `ls` detects and flags it.

**Why differentiating:** Docker's `context ls` has an `ERROR` column for this. Most tools do not. Since opm uses filesystem symlinks (not config keys), broken symlinks are a real failure mode.

**Output with broken symlink:**
```
NAME          PATH                          ERROR
default *     ~/.config/opm/profiles/default  profile directory missing!
work          ~/.config/opm/profiles/work
```

**UX:** Print warning to stderr for broken profiles; exit 1 if the active profile is broken.

**Complexity:** Low (use `os.Lstat` on each profile path)  
**Dependencies:** TS-3

---

### D-2: `opm context show` designed for shell prompt integration

**What it does:** A simple `$(opm context show)` should be fast enough to include in shell prompt (PS1/starship) without latency.

**Why differentiating:** The most common use of `docker context show` and `kubectl config current-context` is shell prompt widgets. If `opm context show` is slow (e.g., > 50ms), prompt plugins won't adopt it.

**Implementation:** Read from `~/.config/opm/current` (a plain text file) — no directory scanning, no symlink resolution. Should be < 5ms.

**Complexity:** Trivial IF implemented correctly (just read one file, don't scan anything)  
**Dependencies:** TS-5

---

### D-3: `opm context rename <old> <new>` — Rename a profile

**What it does:** Renames the profile directory and updates the `current` state file if needed.

**Why differentiating:** kubectl and gcloud support this; docker does not. Users often name profiles generically at first (`client`) and want to rename later (`client-acme`).

**UX (kubectl pattern):** `opm context rename old-name new-name`
- Renames `~/.config/opm/profiles/<old>/` → `~/.config/opm/profiles/<new>/`
- If this was the active profile: re-point symlink to new path, update `current` file

**Complexity:** Low-Medium (directory rename + possible symlink update)  
**Dependencies:** TS-1, TS-2

---

### D-4: `opm context copy <src> <dst>` — Clone a profile

**What it does:** Deep-copies `~/.config/opm/profiles/<src>/` to `~/.config/opm/profiles/<dst>/`.

**Why differentiating:** docker supports `--from` on context create (shallow copy of endpoint config). For opm, cloning a full profile (including AGENTS.md, opencode.json config, agents/) is a high-value workflow — "start with my work config, customize for this client."

**UX:** `opm context copy work client-acme`
- `cp -r ~/.config/opm/profiles/work ~/.config/opm/profiles/client-acme`
- Does NOT switch to the new profile

**Complexity:** Medium (recursive directory copy in Go — `io/fs` walking)  
**Dependencies:** TS-1

---

### D-5: `opm doctor` — Validate installation health

**What it does:** Checks that `~/.config/opencode` is a symlink → an opm-managed profile, that the profile directory exists, that `current` file matches symlink target.

**Why differentiating:** Users who partially set up opm or manually moved files get confusing failures. A `doctor` command surfaces the issue immediately.

**Output:**
```
✓ opm initialized
✓ ~/.config/opencode is a symlink
✓ Active profile: work
✓ Profile directory exists: ~/.config/opm/profiles/work
✗ ~/.config/opencode points to /other/path — not managed by opm
  Run: opm init to fix
```

**Complexity:** Low (series of `os.Lstat` + `os.Readlink` checks)  
**Dependencies:** Depends on nothing — useful precisely when other things are broken

---

### D-6: Profile name validation at create time

**What it does:** Enforces profile names are lowercase alphanumeric + hyphens. Rejects names like `My Profile`, `../../../etc`, `CON` (Windows reserved).

**Why differentiating:** Docker silently allows problematic context names. Since profile names become directory names, invalid names cause filesystem issues. Fail fast with clear error.

**Complexity:** Trivial (single regex check)  
**Dependencies:** TS-1 (create command)

---

## Anti-Features (Deliberate Omissions for v1)

Features to explicitly NOT build — and why.

### AF-1: Interactive profile picker (TUI)

**What it would be:** `opm context use` with no argument → interactive fuzzy-select menu.

**Why NOT:**
- Requires bubbletea/promptui dependency (adds complexity, potential breakage)
- Non-composable in scripts (can't pipe)
- `docker context use` does not have this — it requires an explicit name
- Shell tab completion solves the discovery problem better for a CLI-native audience

**What to do instead:** Clear usage error + suggest `opm context ls` to see available profiles.

---

### AF-2: Remote sync / backup of profiles

**What it would be:** `opm sync` — push profiles to S3/git/cloud storage.

**Why NOT:**
- Profiles contain sensitive data (API keys in opencode.json, MCP credentials)
- Sync requires auth, conflict resolution, encryption — a separate product
- Out of scope explicitly in PROJECT.md

**What to do instead:** Users can put `~/.config/opm/profiles/` in their own dotfiles/git repo.

---

### AF-3: Profile inheritance / base profiles

**What it would be:** Profile `work-client` inherits from `work`, overriding specific keys.

**Why NOT:**
- Adds a config file format, a merge algorithm, and a debugging surface
- OpenCode's config format is not stable enough to build merging on top of
- Out of scope explicitly in PROJECT.md

**What to do instead:** `opm context copy` (D-4) — clone and diverge.

---

### AF-4: `opm context update` — Edit profile metadata

**What it would be:** Update description, tags, or other metadata on an existing profile.

**Why NOT:**
- Requires a profile metadata format (JSON/YAML sidecar per profile)
- Descriptions are low-value — profile names should be self-documenting
- Docker has this; it's rarely used in practice

**What to do instead:** Profiles are just directories. The name is the metadata.

---

### AF-5: Export / import to archive format

**What it would be:** `opm context export work work.tar.gz`, `opm context import client.tar.gz`

**Why NOT:**
- Complex: must handle node_modules (large), binary files, platform paths
- Use case is covered better by: user zips the profile directory themselves
- Docker has this; it's for network endpoint configs (small), not full directory trees

**What to do instead:** `cp -r ~/.config/opm/profiles/work ~/backup/opm-work-backup`

---

### AF-6: `OPM_CONTEXT` env var override

**What it would be:** Temporarily override the active profile per-shell via environment variable (like `AWS_PROFILE`, `DOCKER_CONTEXT`, `KUBECONFIG`).

**Why NOT for v1:**
- Symlink mechanism is persistent and global by design (that's the stated value)
- Env var override requires shell wrapper functions (not a plain Go binary behavior)
- Adds complexity to `opm context show` (must check env var first)
- The PROJECT.md explicitly chose symlinks OVER env var approach for this reason

**What to do instead:** `opm context use <name>` — takes one command, takes immediate effect.

> Note: This could be reconsidered post-v1 if users request per-shell isolation.

---

### AF-7: Profile locking / protection

**What it would be:** `opm context protect default` — prevents accidental deletion or switching away.

**Why NOT:** Premature. Add only if users report deleting the wrong profile.

---

## Feature Dependencies Map

```
opm init (TS-6)
    └── Required before all other commands

opm context create (TS-1)
    └── Enables: use (TS-2), ls (TS-3), rm (TS-4), inspect (TS-7), copy (D-4), rename (D-3)

opm context use (TS-2)
    └── Enables: show (TS-5), prompt integration (D-2)

opm context ls (TS-3)
    └── Enables: --format json (TS-9), shell completion (TS-8)
    └── Uses: broken symlink detection (D-1)

opm context show (TS-5)
    └── Fast path enables: shell prompt integration (D-2)

Shell completion (TS-8)
    └── Requires: ls data source (TS-3), cobra ValidArgsFunction wiring

opm doctor (D-5)
    └── No dependencies — standalone diagnostic
```

---

## MVP Recommendation

Build in this order — each layer enables the next:

### Phase 1: Core switching loop (all table stakes except completions)
1. **TS-6** `opm init` — required first; adoption path
2. **TS-1** `opm context create` — create profiles
3. **TS-2** `opm context use` — switch profiles
4. **TS-3** `opm context ls` — list profiles (with `*` active marker)
5. **TS-4** `opm context rm` — delete profiles
6. **TS-5** `opm context show` — print current (needed for doctor + prompt)
7. **TS-7** `opm context inspect` — profile details

### Phase 2: Polish and scripting
8. **TS-8** Shell completion — tab-complete profile names
9. **TS-9** `--format json` on `ls` and `inspect`
10. **TS-10** is implicit — enforce throughout Phase 1

### Phase 3: Differentiators
11. **D-1** Broken symlink detection (add to ls)
12. **D-2** Fast show (implement correctly in Phase 1, validate here)
13. **D-5** `opm doctor` — health check
14. **D-6** Profile name validation (add to create in Phase 1)
15. **D-3** `opm context rename`
16. **D-4** `opm context copy`

### Defer indefinitely
- AF-1 through AF-7 — explicitly out of scope

---

## Error Handling UX Conventions

Derived from observing docker, kubectl, gcloud behavior:

| Situation | Message Pattern | Exit Code | Output Stream |
|-----------|----------------|-----------|---------------|
| Profile not found | `context "name" does not exist` | 1 | stderr |
| Profile already exists (on create) | `context "name" already exists` | 1 | stderr |
| Active profile (on rm without -f) | `context "name" is in use, set -f flag to force remove` | 1 | stderr |
| opm not initialized | `opm is not initialized. Run: opm init` | 1 | stderr |
| Symlink target broken (on use) | `profile directory for "name" is missing at /path` | 1 | stderr |
| Invalid profile name | `invalid profile name "My Name": use lowercase letters, digits, and hyphens only` | 1 | stderr |
| Success (switch) | `Current context is now "name"` | 0 | stdout |
| Success (create) | `Context "name" created` | 0 | stdout |
| Success (rm) | `Context "name" removed` | 0 | stdout |

**Key convention:** Error messages name the entity in double quotes: `context "name"`. Adopted from both docker and kubectl.

---

## Output Format Conventions

### Table (default for `ls`)
```
NAME          PATH
default *     ~/.config/opm/profiles/default
work          ~/.config/opm/profiles/work
client-acme   ~/.config/opm/profiles/client-acme
```

Use `text/tabwriter` with tab stops. Active profile gets `*` suffix in NAME column (docker-style).

### JSON (`--format json` on `ls`)
```json
[
  {"name": "default", "active": true, "path": "/Users/user/.config/opm/profiles/default"},
  {"name": "work", "active": false, "path": "/Users/user/.config/opm/profiles/work"}
]
```

### Quiet (`-q` on `ls`)
```
default
work
client-acme
```
One name per line, no header, no asterisk. Used by shell completion and scripts.

### Inspect (default pretty-print)
```
Name:    work
Active:  false
Path:    /Users/user/.config/opm/profiles/work
```

### Inspect (`--format json`)
```json
{
  "name": "work",
  "active": false,
  "path": "/Users/user/.config/opm/profiles/work"
}
```

---

## Shell Completion Design

Cobra `ValidArgsFunction` pattern (from docker/cli source):

```go
// Completion returns profile names for tab-complete
func completeProfileNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
    profiles, err := loadProfiles()
    if err != nil {
        return nil, cobra.ShellCompDirectiveError
    }
    names := make([]string, 0, len(profiles))
    for _, p := range profiles {
        names = append(names, p.Name)
    }
    return names, cobra.ShellCompDirectiveNoFileComp
}
```

Apply to: `context use`, `context rm`, `context inspect`, `context rename`, `context copy`.

Install via:
```bash
# zsh
echo 'eval "$(opm completion zsh)"' >> ~/.zshrc

# bash
echo 'source <(opm completion bash)' >> ~/.bashrc
```

---

## opm-Specific Features (not present in analogous tools)

These are unique to the OpenCode + symlink-based approach:

| Feature | Rationale | Category |
|---------|-----------|---------|
| Broken symlink detection | Symlink mechanism = real failure mode docker's config approach doesn't have | Differentiator (D-1) |
| `opm doctor` | More failure modes than a config-key approach | Differentiator (D-5) |
| `opm init` migration | No analogous tool needs to migrate an existing dir — they start fresh | Table Stakes (TS-6) |
| Profile is a full directory (node_modules, etc.) | Docker context is lightweight metadata; opm profiles are gigabytes of plugins | Shapes copy/backup AF decisions |
| `opm context show` fast path | Must be prompt-safe (< 5ms); other tools are slower because they do more | Differentiator (D-2) |

---

## Sources

All evidence is HIGH confidence from live inspection:

- `docker context --help`, `docker context ls`, `docker context create --help`, `docker context inspect orbstack`, `docker context rm`, `docker context show` — local docker installation
- `kubectl config --help`, `kubectl config get-contexts`, `kubectl config current-context`, `kubectl config rename-context --help` — local kubectl installation  
- `gcloud config configurations --help`, `gcloud config configurations list`, `gcloud config configurations describe default`, `gcloud config configurations list --format=json` — local gcloud installation
- `terraform workspace --help`, `terraform workspace list` — local terraform installation
- `gh auth --help`, `gh auth switch --help`, `gh auth status --help` — local gh installation
- `cobra-cli completion --help` — local cobra-cli installation
- docker context error behavior: verified by running `docker context use nonexistent-context-xyz` (exit 1, specific message format)
- docker context rm guard: verified by running `docker context rm orbstack` (active context, exit 1, force flag suggestion)
- STACK.md: pre-existing research confirming Cobra `ValidArgsFunction` pattern from docker/cli source
