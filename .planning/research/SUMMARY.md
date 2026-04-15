# Project Research Summary

**Project:** opm — OpenCode Profile Manager
**Domain:** Go CLI / symlink-based config profile switching
**Researched:** 2026-04-15
**Confidence:** HIGH

## Executive Summary

opm is a single-purpose Go CLI tool that manages multiple OpenCode environments by symlink-switching `~/.config/opencode` to isolated profile directories. The domain is well-understood: every major developer tool (docker, kubectl, gcloud, terraform, gh) solves this problem with an identical command surface — create, use, ls, inspect, rm — and the right approach is to copy it faithfully. Research confirms this is a small, focused tool that should stay small: Cobra for command routing, Go stdlib for all filesystem operations, plain files for state, GoReleaser for distribution. No framework, no database, no config library, no TUI — just a thin command layer over careful filesystem operations.

The core technical complexity is not the product design (that's solved) but the symlink implementation: three well-documented failure modes can cause data loss if not handled correctly. `opm init` must survive a crash mid-migration using a 3-step atomic sequence. `opm context use` must swap symlinks atomically (temp + rename) — never remove-then-create. `opm context rm` must refuse to delete the active profile. Handle these three cases correctly and the implementation is solid.

The recommended build order is bottom-up: implement and test internal packages (`paths`, `store`, `symlink`) first — before writing any command code. Commands are thin wiring; all complexity lives in the internal packages. Every function in `store` and `symlink` must be independently testable via `t.TempDir()`. This order eliminates untestable cobra-entangled logic and is the pattern used by docker/cli and kubectl.

## Key Findings

### Recommended Stack

A minimal, zero-bloat Go stack modeled on how docker/cli and kubectl are built. The only external dependencies are Cobra (command routing) and testify (test assertions). All filesystem operations use Go stdlib exclusively. GoReleaser handles cross-platform distribution and Homebrew tap generation automatically.

One critical macOS gotcha: `os.UserConfigDir()` returns `~/Library/Application Support` on macOS, NOT `~/.config`. Since opm explicitly manages XDG paths, use `os.UserHomeDir()` + filepath.Join to construct `~/.config/opm` and `~/.config/opencode` — do not use `os.UserConfigDir()` for these paths.

**Core technologies:**
- `github.com/spf13/cobra` v1.10.2: command routing, subcommands, shell completions — industry standard, used by docker CLI, kubectl, gh, Helm; `ValidArgsFunction` gives free tab-completion of profile names
- `os` / `path/filepath` (stdlib): all symlink and filesystem operations — no third-party fs library needed
- `text/tabwriter` (stdlib): aligned column output for `opm context ls` — zero dependency
- Plain text files: `~/.config/opm/current` (one-line profile name) + `~/.config/opm/profiles/<name>/` dirs — no database, no config library
- GoReleaser v2.15.3: macOS universal binary, Linux amd64/arm64, Homebrew tap, checksums — one config file replaces a hand-rolled Makefile
- `github.com/stretchr/testify` v1.10.x: `assert`/`require` + `FileExists`/`DirExists` — directly useful for symlink tests
- Go 1.22+ (targeting `go 1.22` in `go.mod` for broad compatibility; local environment has 1.26.2)

### Expected Features

Research verified against live inspection of docker, kubectl, gcloud, terraform, gh. The command surface is standardized across the ecosystem. opm should follow it exactly — deviation from the pattern will feel wrong to the target audience.

**Must have (table stakes — ship in v1):**
- `opm init` — migrate existing `~/.config/opencode` into `default` profile (non-destructive onboarding; unique to opm, no analog in other tools)
- `opm context create <name>` — create new empty profile; fails if exists, does NOT auto-switch
- `opm context use <name>` — atomic symlink swap; message: `Current context is now "name"`
- `opm context ls` — table with NAME + PATH, active marked with `*` (docker-style); `-q` for scripts; `--format json` for jq pipelines
- `opm context rm <name>` — refuse if active (without `--force`); warns before deleting node_modules
- `opm context show` — prints current profile name only; must be < 5ms (shell prompt integration)
- `opm context inspect <name>` — key-value block + `--format json`
- Shell completion — `opm completion <shell>` + `ValidArgsFunction` on all name-taking commands
- Non-zero exit codes + stderr errors everywhere

**Should have (differentiators — high value, include in v1 or early v2):**
- Broken symlink detection in `ls` — critical because opm uses actual symlinks (docker's approach doesn't have this failure mode)
- Profile name validation at create time — allowlist `^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,62}$`; prevents path traversal
- `opm doctor` — health check for broken installs; essential for a tool that manages filesystem state
- `opm context rename <old> <new>` — natural workflow; kubectl and gcloud both have it
- `opm context copy <src> <dst>` — clone a profile as starting point; high-value for "work → client-acme" workflow

**Defer (v2+):**
- Interactive TUI picker — scripts and tab-completion solve this better for the target audience
- Remote sync / backup — separate product; profiles contain sensitive keys
- Profile inheritance / merging — complex; `copy` + diverge is the right v1 answer
- Export/import archives — user can zip the directory themselves
- `OPM_CONTEXT` env var override — env var approach was explicitly rejected in PROJECT.md; revisit post-v1 if requested

**Error message conventions (from docker/kubectl):** Entity names in double quotes. `context "name" does not exist`. Error to stderr, success to stdout.

### Architecture Approach

opm follows the docker/cli architecture: a thin Cobra `cmd/` layer delegates entirely to internal packages. Commands contain zero business logic — they parse args, call internal functions, and format output. The `internal/store` package owns all profile state. The `internal/symlink` package owns all symlink operations. The `internal/paths` package provides canonical path resolution. This separation makes every piece independently testable without a cobra invocation.

**Major components:**
1. `internal/paths` — pure computation; returns absolute paths for `~/.config/opm`, `~/.config/opencode`, profile dirs; no side effects
2. `internal/store` — reads/writes `~/.config/opm/`; profile CRUD, current-file management; injected with root path for testing
3. `internal/symlink` — `Inspect()` (lstat + readlink), `SetAtomic()` (temp+rename); knows nothing about profile names
4. `cmd/` — one file per command; thin cobra wiring; calls store + symlink; formats output with `text/tabwriter`
5. `main.go` — entry point; executes root command

**Cobra tree:** `rootCmd` → `init` + `context` → `create` / `use` / `ls` / `inspect` / `rm`

**Build order (Layer → Layer):** `paths` → `store` + `symlink` (in parallel) → `cmd/` → `main.go`. Fully implement and test Layers 1–3 before touching any command code.

**`opm init` decision tree:** (A) path doesn't exist → create empty default + symlink; (B) real directory → move to profiles/default + create symlink; (C) already an opm symlink → "already initialized"; (D) foreign symlink → warn and abort.

**Active profile derivation:** Derive from `os.Readlink()` on the actual symlink, not from the `current` file. Write `current` as a fast-path cache for shell prompts only; always verify against the symlink for UI display.

### Critical Pitfalls

1. **`os.Symlink` fails with EEXIST if symlink already exists** — use atomic temp+rename pattern exclusively: `os.Symlink(target, tmp)` then `os.Rename(tmp, dst)`. Never remove-then-create. A window where `~/.config/opencode` doesn't exist will corrupt OpenCode state.

2. **`opm init` crash between dir-move and symlink-create** — leaves no `~/.config/opencode`, OpenCode reinitializes blank (data loss). Use 3-step crash-safe sequence: (1) create temp symlink, (2) move real dir, (3) atomic rename temp symlink into place. Add idempotency checks at the start for each intermediate state.

3. **Deleting the active profile leaves a dangling symlink** — `opm context rm` must call `os.Readlink()` to check if the profile is currently active and refuse deletion (without `--force`). A dangling symlink causes OpenCode to fail or silently create blank config.

4. **`os.Stat` vs `os.Lstat`** — `os.Stat` follows symlinks; a dangling symlink returns `ENOENT` as if the path doesn't exist. Use `os.Lstat` everywhere opm inspects `~/.config/opencode` — it's the only way to correctly distinguish: path missing / dangling symlink / opm-managed symlink / real directory.

5. **Relative vs absolute symlink targets** — always use absolute paths for symlink targets. Relative symlinks resolve incorrectly in macOS Finder and some GUI apps. Anchor all profile paths to `os.UserHomeDir()`.

## Implications for Roadmap

Based on research, the dependency structure clearly dictates 3 phases:

### Phase 1: Foundation — Internal Packages
**Rationale:** `cmd/` code is untestable without injectable internal packages. Build and fully test `paths`, `store`, and `symlink` before writing any command. This is how docker/cli is structured — the internal packages are the complexity; commands are glue.
**Delivers:** Fully tested `internal/paths`, `internal/store`, `internal/symlink` packages; atomic symlink swap; profile CRUD; current-file read/write
**Implements:** Layers 1–3 of the architecture build order
**Avoids:** Pitfalls 1 (EEXIST), 4 (os.Stat), 5 (relative symlinks) — all live in symlink package

### Phase 2: Core Command Loop
**Rationale:** Once internal packages are solid, commands are thin wiring. Build the minimum viable loop first: init → create → use → ls. These four commands cover the entire adoption story. `rm`, `show`, `inspect` follow immediately after.
**Delivers:** Working CLI with `opm init`, `opm context create/use/ls/rm/show/inspect`; proper exit codes; stderr/stdout split
**Addresses:** All 10 table-stakes features (TS-1 through TS-10)
**Avoids:** Pitfall 2 (init crash safety), Pitfall 3 (active profile guard on rm), Pitfall 11 (node_modules warning on rm)

### Phase 3: Shell Integration + Polish
**Rationale:** Shell completion requires working `ls` (Phase 2). Broken symlink detection enhances `ls`. `opm doctor` needs all other commands working to validate against. Name validation is a quick add to `create`.
**Delivers:** Shell completions (`bash`, `zsh`, `fish`); `--format json` on `ls`/`inspect`; broken symlink detection; profile name validation; `opm doctor`
**Addresses:** Differentiators D-1 through D-6; TS-8/TS-9
**Avoids:** Pitfall 7 (stale current file — derive from readlink), Pitfall 8 (name validation)

### Phase 4: Power Features
**Rationale:** `context rename` and `context copy` are independently shippable enhancements. Low risk, clear value, no new architectural dependencies.
**Delivers:** `opm context rename <old> <new>`, `opm context copy <src> <dst>`
**Addresses:** Differentiators D-3, D-4

### Phase 5: Distribution
**Rationale:** GoReleaser config, GitHub Actions workflow, and Homebrew tap are well-understood tasks that don't depend on each other but do need a working binary.
**Delivers:** `brew install <user>/tap/opm`; GitHub Releases with checksums; `go install` path
**Uses:** GoReleaser v2.15.3, GitHub Actions, macOS universal binary (amd64+arm64)

### Phase Ordering Rationale

- Foundation before commands: the most common mistake in Go CLIs is entangling logic in cobra RunE functions — that becomes untestable. Build the packages first.
- `opm init` is the first command users run — it must be crash-safe. It's also the hardest command. Building it in Phase 2 after symlink primitives are tested is the right order.
- Shell completion depends on working `ls` (needs profile list). Add completions in Phase 3 when the data source is stable.
- Distribution is last — you need a shippable binary, but GoReleaser config doesn't need to be perfect on Day 1.

### Research Flags

Phases with standard patterns (skip `/gsd-research-phase`):
- **Phase 1:** Symlink operations and Go stdlib file I/O are thoroughly documented. Patterns are verified from POSIX man pages and Go pkg.go.dev.
- **Phase 3 (shell completion):** Cobra's `ValidArgsFunction` pattern is well-documented; docker/cli source is the reference implementation.
- **Phase 5 (distribution):** GoReleaser documentation is comprehensive; Homebrew tap configuration is standard.

Phases that may benefit from targeted research:
- **Phase 2 (`opm init` crash safety):** The 3-step atomic init sequence is described in PITFALLS.md but has edge cases. Consider a focused spike on the intermediate-state detection logic before implementing.

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Cobra, Go stdlib, GoReleaser all verified via Context7 + official releases. One macOS gotcha (UserConfigDir) is documented. |
| Features | HIGH | Verified by live inspection of docker, kubectl, gcloud, terraform, gh on local machine. Command surface is standardized. |
| Architecture | HIGH | Patterns verified from docker/cli and kubectl source. Go stdlib APIs confirmed at pkg.go.dev (go1.26.2). |
| Pitfalls | HIGH | All critical pitfalls verified against POSIX man pages and Go stdlib docs. No inferred pitfalls — all are observed failure modes. |

**Overall confidence:** HIGH

### Gaps to Address

- **macOS `UserConfigDir()` path:** STACK.md documents that `os.UserConfigDir()` returns `~/Library/Application Support` on macOS, not `~/.config`. The `internal/paths` package must hard-code XDG path construction using `os.UserHomeDir()`. Validate this in the first test run on macOS.
- **`opm context rm --force` behavior:** Research documents that deletion of the active profile should be blocked without `--force`. The behavior of `--force` (switch to `default` first? or leave dangling?) needs a concrete decision during Phase 2 planning.
- **Profile size warning on `rm`:** PITFALLS.md recommends printing a size estimate before `os.RemoveAll`. Go stdlib has no single-call directory size function — needs a `filepath.Walk` sum. Decide whether to implement or just warn by path.
- **Cross-filesystem `EXDEV` fallback:** Moderately complex error handling for containerized environments. Low priority for v1 macOS-first use, but should be a Phase 1 consideration in the symlink package.

## Sources

### Primary (HIGH confidence)
- `github.com/spf13/cobra` v1.10.2 — command routing, ValidArgsFunction completion pattern
- `github.com/docker/cli` (master) — `cli/command/context/use.go`, `cli/context/store/store.go` — verified Cobra usage and store pattern
- `github.com/kubernetes/kubectl` (master) — `pkg/cmd/config/config.go` — context command patterns
- GoReleaser v2.15.3 — https://github.com/goreleaser/goreleaser/releases — Homebrew tap, universal binary config
- Go stdlib `os` package — https://pkg.go.dev/os (go1.26.2) — Symlink, Rename, Lstat, UserConfigDir, UserHomeDir
- POSIX `rename(2)`, `symlink(2)` man pages — atomicity guarantees verified locally 2026-04-15
- Live CLI inspection: `docker context`, `kubectl config`, `gcloud config configurations`, `terraform workspace`, `gh auth`

### Secondary (MEDIUM confidence)
- Context7 `/goreleaser/goreleaser` — Homebrew tap configuration details
- Context7 `/stretchr/testify` — v1 stability, FileExists/DirExists assertions
- Context7 `/spf13/cobra` — ValidArgsFunction shell completion docs

---
*Research completed: 2026-04-15*
*Ready for roadmap: yes*
