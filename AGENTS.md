<!-- GSD:project-start source:PROJECT.md -->
## Project

**opm — OpenCode Profile Manager**

A Go CLI tool that manages multiple OpenCode configurations by symlinking `~/.config/opencode` to named profile directories. Users switch between completely isolated OpenCode environments (different MCPs, plugins, agents, models, AGENTS.md) with a single command.

**Core Value:** Switching OpenCode profiles should be one command — `opm use <name>` — and take effect after reloading OpenCode.

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

| Layer | Choice |
|-------|--------|
| CLI framework | `github.com/spf13/cobra` v1.10.2 |
| Output color | `github.com/fatih/color` v1.19.0 |
| Test assertions | `github.com/stretchr/testify` v1.10.0 |
| Release | GoReleaser v2 (`just release` / `just snapshot`) |
| Linting | `golangci-lint` v2.x |

Do not add `bubbletea`, `viper`, `lipgloss`, or any TUI/config-management library — this tool has no interactive UI and IS a config manager.
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

### Developer commands (justfile)

Use `just` — not `make` or raw `go` commands — for all lifecycle tasks:

| Command | What it does |
|---------|-------------|
| `just test` | `go test ./...` |
| `just testv` | verbose tests |
| `just check` | `go vet ./...` + `golangci-lint run` |
| `just verify` | check + test (CI equivalent) |
| `just build` | binary with version/commit ldflags injected |
| `just assemble` | build without ldflags (fast dev build) |
| `just run <args>` | `go run . <args>` |
| `just cover` | coverage report → `coverage.html` |

Run a single package's tests: `go test ./internal/store/...`

### Linting

`golangci-lint` v2.x with `.golangci.yml`. Enabled linters: `errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`. All error returns must be handled (checked by `errcheck`) — use `_ =` explicitly for intentional ignores.

### Profile name validation

Names must match `^[a-zA-Z0-9][a-zA-Z0-9_.\-]{0,62}$`. Call `store.ValidateName(name)` before touching the filesystem. Do not invent ad-hoc validation.

### Error messages

User-facing errors refer to profiles as "profile" (e.g. `"profile %q does not exist"`). Match this convention for consistency with the existing surface.

### Version injection

`version` and `commit` are injected at build time via ldflags into `main.go` vars. `just build` does this automatically; `just assemble` does not.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

### Package layout

```
main.go                   — entry point; calls cmd.SetVersionInfo + cmd.Execute
cmd/                      — one file per subcommand; all registered via init()
  root.go                 — rootCmd, Execute, newStore(), managedGuard()
  help.go                 — custom help/usage output (registerHelp)
internal/
  paths/                  — filesystem path constants (OpmDir, OpencodeConfigDir, ProfilesDir, etc.)
  store/                  — all profile CRUD and state logic (Store struct)
  symlink/                — atomic symlink ops (Inspect, SetAtomic)
  output/                 — shared print helpers (color, icons)
```

### Key invariants

- **`~/.config/opencode`** is the managed symlink (never `~/Library/Application Support/opencode` — `os.UserConfigDir()` is NOT used; paths are resolved via `os.UserHomeDir()` + hardcoded `.config`).
- **Active profile source of truth**: `os.Readlink(~/.config/opencode)` — not the `current` file. The current file is a convenience cache only.
- **Atomic symlink swap**: `symlink.SetAtomic` uses create-tmp + `os.Rename` (never remove-then-create). Use it everywhere a symlink must be installed or replaced.
- **`managedGuard`**: commands that require opm management call `managedGuard` in `PreRunE` — it aborts early with a clear message if `~/.config/opencode` is not an opm-managed symlink.

### Testing

Tests use `t.TempDir()` for isolated filesystem — no mocking of `os.*` calls. `Store` is constructed with a temp root and temp opencodeDir so tests never touch real config. See `internal/store/store_test.go` for the pattern.

### State files

```
~/.config/opm/
├── current            # plain text, one line: active profile name
└── profiles/
    └── <name>/        # full OpenCode config directory
```

`~/.config/opencode` → `~/.config/opm/profiles/<name>/` (symlink)
<!-- GSD:architecture-end -->
