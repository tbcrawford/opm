<!-- GSD:project-start source:PROJECT.md -->
## Project

**opm — OpenCode Profile Manager**

A Go CLI tool that manages multiple OpenCode configurations by symlinking `~/.config/opencode` to named profile directories. Users switch between completely isolated OpenCode environments (different MCPs, plugins, agents, models, AGENTS.md) with a single command.

**Core Value:** Switching OpenCode profiles should be one command — `opm use <name>` — and take effect immediately without restarting anything.

### Command API

All subcommands are flat (no `context` grouping):

| Command | Description |
|---------|-------------|
| `opm init` | Initialize opm and migrate existing OpenCode config |
| `opm use <name>` | Switch to a profile |
| `opm create <name>` | Create a new profile |
| `opm list` | List all profiles |
| `opm show` | Print the name of the currently active profile |
| `opm inspect <name>` | Show detailed information about a profile |
| `opm rename <old> <new>` | Rename a profile |
| `opm copy <src> <dst>` | Copy a profile to a new name |
| `opm remove <name> [name...]` | Remove one or more profiles |
| `opm path <name>` | Print the filesystem path to a profile directory |
| `opm reset` | Restore `~/.config/opencode` to a plain directory |
| `opm doctor` | Check opm installation health |

### Constraints

- **Language**: Go — single binary, fast, no runtime dependencies
- **Mechanism**: Directory-level symlink (`~/.config/opencode` → profile dir), not file-level
- **UX**: Flat subcommand surface (not `docker context`-style grouping)
- **Compatibility**: Must not break existing OpenCode setup — `opm init` migrates current config non-destructively
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

## Recommended Stack
### CLI Framework
| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/spf13/cobra` | v1.10.2 | Command routing, subcommands, shell completions | Industry standard for `docker context`-style CLIs; used by kubectl, docker CLI, gh, Helm, and thousands of others. Native `ValidArgsFunction` for tab completion of profile names. Zero ceremony for the `context <subcommand>` pattern. |
- `urfave/cli` — simpler but less ecosystem support; worse shell completion story; `docker context` doesn't use it
- `kong` — elegant but niche; optimized for config-heavy CLIs, not command-palette tools; less prior art for context-management tools
- `bubbletea` — wrong abstraction layer; it's a TUI framework for interactive UIs, not a command dispatcher. opm commands are non-interactive (no interactive selection needed for v1). Don't add a TUI framework unless you need animated progress or interactive pickers.
### Go Standard Library (Zero External Dependencies for Core Logic)
| Package | Functions | Purpose |
|---------|-----------|---------|
| `os` | `Symlink`, `Readlink`, `Lstat`, `Remove`, `MkdirAll`, `Rename` | Symlink create/switch/read/delete |
| `os` | `UserConfigDir()` | Returns `~/.config` cross-platform |
| `os` | `ReadFile`, `WriteFile` | Read/write `~/.config/opm/current` |
| `path/filepath` | `Join`, `EvalSymlinks` | Path construction and symlink resolution |
| `encoding/json` | `Marshal`, `Unmarshal` | Profile metadata if needed |
| `fmt` | — | Output formatting |
| `errors` | `Is`, `As` | Error wrapping/inspection |
### Output Formatting
| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Go stdlib `fmt` + `text/tabwriter` | stdlib | Tabular output for `opm context ls` | Zero dependency. `text/tabwriter` produces aligned columns (like `docker context ls` output). |
- `github.com/charmbracelet/lipgloss` — overkill for a non-interactive tool; adds dep for cosmetic output
- `github.com/olekukonko/tablewriter` — popular but unnecessary when `text/tabwriter` covers the `ls` use case perfectly
### State / Config Storage
| Approach | Format | Why |
|----------|--------|-----|
| Plain text file for `current` | One line: profile name | Simplest possible state. Docker uses a JSON config; we don't need JSON for a single string. |
| Plain directory structure | `~/.config/opm/profiles/<name>/` | No database, no config format, just directories |
- `github.com/spf13/viper` — config management for app configuration (flags + env + files); opm IS a config manager, not an app that needs one. Viper is a source of over-engineering for simple tools.
- `github.com/BurntSushi/toml` / YAML — unnecessary; plain text is more debuggable and grep-friendly
### Distribution
| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| GoReleaser | v2.15.3 | Build + release automation | Industry standard. Produces: macOS (amd64 + arm64 universal binary), Linux (amd64 + arm64), checksums. One `.goreleaser.yaml` file = GitHub Release + Homebrew tap + archives. |
| GitHub Actions | — | CI/CD trigger | Free for public repos; GoReleaser GitHub Action v6 is current. |
| Homebrew tap | — | `brew install <user>/tap/opm` | Primary install path for macOS devs. GoReleaser generates formula automatically. |
### Testing
| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `testing` (stdlib) | stdlib | Unit tests, table-driven tests | Go's built-in testing is excellent; no framework needed for logic tests |
| `github.com/stretchr/testify` | v1.10.x | Assertions (`assert`, `require`) | Significantly reduces test boilerplate; `assert.FileExists`, `assert.DirExists` are directly useful for symlink tests. `require` stops test immediately on failure (important for setup assertions). |
| `os.MkdirTemp` (stdlib) | stdlib | Isolated tmp directories per test | Each test gets `t.TempDir()` for isolated filesystem — no cleanup code needed (auto-cleaned after test) |
- `github.com/vektra/mockery` — for mocking interfaces; opm's filesystem operations are thin enough that `os.MkdirTemp` + real filesystem in tests is simpler and more trustworthy than mocking `os.Symlink`
- `gocheck` — obsolete; testify is the modern replacement
### Linting / Code Quality (Dev Tooling)
| Tool | Version | Purpose |
|------|---------|---------|
| `golangci-lint` | v2.x | Aggregated linter (vet, staticcheck, errcheck, etc.) |
## Alternatives Considered
| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| CLI framework | cobra v1.10.2 | urfave/cli v3 | Less ecosystem; worse completions story; no prior art in context-management CLIs |
| CLI framework | cobra v1.10.2 | kong | Elegant but niche; optimized for heavy config, not command dispatch |
| CLI framework | cobra v1.10.2 | bubbletea | TUI framework, wrong layer; opm doesn't need interactive UI |
| Output | `text/tabwriter` | lipgloss/tablewriter | Unnecessary dependency for simple table |
| State | Plain files | viper | opm IS a config manager; using a config library inside it is circular |
| State | Plain files | SQLite | Massive overkill for 5 profile names |
| Distribution | goreleaser | manual Makefile | GoReleaser handles cross-compilation, signing, checksums, Homebrew; no reason to DIY |
## Prior Art: docker context vs. opm
- **Cobra** for the command definition (identical pattern to what we'll build)
- **JSON config file** for storing current context name in `~/.docker/config.json`
- **No symlinks** — docker context uses a config key, not filesystem symlinks
## Installation
# Initialize Go module
# CLI framework
# Testing assertions
# Dev: release tooling (install separately, not as a dep)
# brew install goreleaser
# or: go install github.com/goreleaser/goreleaser/v2@latest
## Go Version
## Sources
- Cobra releases: https://github.com/spf13/cobra/releases (v1.10.2 confirmed current)
- GoReleaser releases: https://github.com/goreleaser/goreleaser/releases (v2.15.3, 2026-04-15)
- GoReleaser Homebrew docs: Context7 `/goreleaser/goreleaser` (HIGH reputation)
- Testify: Context7 `/stretchr/testify` (HIGH reputation, v1 stable)
- Docker context use.go source: https://github.com/docker/cli/blob/master/cli/command/context/use.go (verified Cobra usage)
- Go os package: https://pkg.go.dev/os (stdlib, HIGH confidence)
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:skills-start source:skills/ -->
## Project Skills

No project skills found. Add skills to any of: `.claude/skills/`, `.agents/skills/`, `.cursor/skills/`, or `.github/skills/` with a `SKILL.md` index file.
<!-- GSD:skills-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd-quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd-debug` for investigation and bug fixing
- `/gsd-execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd-profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->
