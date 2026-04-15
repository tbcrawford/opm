# Technology Stack

**Project:** opm — OpenCode Profile Manager
**Researched:** 2026-04-15
**Research Mode:** Ecosystem

---

## Recommended Stack

### CLI Framework

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `github.com/spf13/cobra` | v1.10.2 | Command routing, subcommands, shell completions | Industry standard for `docker context`-style CLIs; used by kubectl, docker CLI, gh, Helm, and thousands of others. Native `ValidArgsFunction` for tab completion of profile names. Zero ceremony for the `context <subcommand>` pattern. |

**Confidence: HIGH** — Verified via Context7 + official GitHub releases (latest: v1.10.2, released Dec 2025). Docker's `docker context use` is literally implemented with Cobra (verified by reading `docker/cli` source).

**Why NOT alternatives:**
- `urfave/cli` — simpler but less ecosystem support; worse shell completion story; `docker context` doesn't use it
- `kong` — elegant but niche; optimized for config-heavy CLIs, not command-palette tools; less prior art for context-management tools
- `bubbletea` — wrong abstraction layer; it's a TUI framework for interactive UIs, not a command dispatcher. opm commands are non-interactive (no interactive selection needed for v1). Don't add a TUI framework unless you need animated progress or interactive pickers.

---

### Go Standard Library (Zero External Dependencies for Core Logic)

All symlink management is handled entirely by `os` package — **no third-party filesystem library needed**.

| Package | Functions | Purpose |
|---------|-----------|---------|
| `os` | `Symlink`, `Readlink`, `Lstat`, `Remove`, `MkdirAll`, `Rename` | Symlink create/switch/read/delete |
| `os` | `UserConfigDir()` | Returns `~/.config` cross-platform |
| `os` | `ReadFile`, `WriteFile` | Read/write `~/.config/opm/current` |
| `path/filepath` | `Join`, `EvalSymlinks` | Path construction and symlink resolution |
| `encoding/json` | `Marshal`, `Unmarshal` | Profile metadata if needed |
| `fmt` | — | Output formatting |
| `errors` | `Is`, `As` | Error wrapping/inspection |

**Confidence: HIGH** — `os.Symlink` and `os.Readlink` are stable Go stdlib, documented at pkg.go.dev. `os.UserConfigDir()` returns `$XDG_CONFIG_HOME` on Linux, `~/Library/Application Support` on macOS (note: **NOT** `~/.config` on macOS). See critical note below.

**Critical note on `os.UserConfigDir()`:**  
On macOS, `os.UserConfigDir()` returns `~/Library/Application Support`, NOT `~/.config`. Since opm's storage is explicitly `~/.config/opm/` and `~/.config/opencode/` (XDG paths), **hard-code the XDG path**: use `os.UserHomeDir()` + `filepath.Join(".config", ...)` rather than `os.UserConfigDir()`. This matches how OpenCode itself stores config.

**Symlink swap pattern** (atomic-safe on most filesystems):
```go
// Safe switch: write new symlink to temp name, then rename atomically
tmpLink := targetLink + ".tmp"
os.Remove(tmpLink)
os.Symlink(profilePath, tmpLink)
os.Rename(tmpLink, targetLink)  // atomic on same filesystem
```

---

### Output Formatting

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Go stdlib `fmt` + `text/tabwriter` | stdlib | Tabular output for `opm context ls` | Zero dependency. `text/tabwriter` produces aligned columns (like `docker context ls` output). |

**Confidence: HIGH** — Standard Go. No external library needed for simple table output.

**Why NOT:**
- `github.com/charmbracelet/lipgloss` — overkill for a non-interactive tool; adds dep for cosmetic output
- `github.com/olekukonko/tablewriter` — popular but unnecessary when `text/tabwriter` covers the `ls` use case perfectly

---

### State / Config Storage

| Approach | Format | Why |
|----------|--------|-----|
| Plain text file for `current` | One line: profile name | Simplest possible state. Docker uses a JSON config; we don't need JSON for a single string. |
| Plain directory structure | `~/.config/opm/profiles/<name>/` | No database, no config format, just directories |

**Confidence: HIGH** — opm's state is minimal: one symlink + one "current profile name" file. No ORM, no database, no config library needed.

**Do NOT use:**
- `github.com/spf13/viper` — config management for app configuration (flags + env + files); opm IS a config manager, not an app that needs one. Viper is a source of over-engineering for simple tools.
- `github.com/BurntSushi/toml` / YAML — unnecessary; plain text is more debuggable and grep-friendly

---

### Distribution

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| GoReleaser | v2.15.3 | Build + release automation | Industry standard. Produces: macOS (amd64 + arm64 universal binary), Linux (amd64 + arm64), checksums. One `.goreleaser.yaml` file = GitHub Release + Homebrew tap + archives. |
| GitHub Actions | — | CI/CD trigger | Free for public repos; GoReleaser GitHub Action v6 is current. |
| Homebrew tap | — | `brew install <user>/tap/opm` | Primary install path for macOS devs. GoReleaser generates formula automatically. |

**Confidence: HIGH** — GoReleaser v2.15.3 released 2026-04-15 (same day as this research). Homebrew tap configuration verified via Context7 docs.

**Distribution strategy:**
1. **Primary:** `brew install <user>/homebrew-tap/opm` — zero friction for macOS devs
2. **Secondary:** GitHub Releases with binary archives + checksums — Linux/manual install
3. **Future:** `go install github.com/<user>/opm@latest` — works automatically since it's a pure Go binary with no CGO

**Example `.goreleaser.yaml` skeleton:**
```yaml
builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X main.version={{.Version}} -X main.commit={{.Commit}}

universal_binaries:
  - replace: true   # merge darwin amd64+arm64 into one fat binary

brews:
  - repository:
      owner: <your-github-username>
      name: homebrew-tap
      token: "{{ .Env.GITHUB_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/<user>/opm"
    description: "OpenCode profile manager — switch configs like docker context"
    install: bin.install "opm"
    test: system "#{bin}/opm --version"
```

---

### Testing

| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| `testing` (stdlib) | stdlib | Unit tests, table-driven tests | Go's built-in testing is excellent; no framework needed for logic tests |
| `github.com/stretchr/testify` | v1.10.x | Assertions (`assert`, `require`) | Significantly reduces test boilerplate; `assert.FileExists`, `assert.DirExists` are directly useful for symlink tests. `require` stops test immediately on failure (important for setup assertions). |
| `os.MkdirTemp` (stdlib) | stdlib | Isolated tmp directories per test | Each test gets `t.TempDir()` for isolated filesystem — no cleanup code needed (auto-cleaned after test) |

**Confidence: HIGH** — Testify v1 is stable with no breaking changes planned. Verified via Context7. The `FileExists` / `DirExists` / `NoFileExists` assertions map perfectly to opm's test needs.

**Testing pattern for opm:**
```go
func TestContextSwitch(t *testing.T) {
    // Set up isolated fake HOME in tmp
    home := t.TempDir()
    opmDir := filepath.Join(home, ".config", "opm", "profiles", "work")
    require.NoError(t, os.MkdirAll(opmDir, 0755))
    
    symlinkTarget := filepath.Join(home, ".config", "opencode")
    
    err := switchProfile(symlinkTarget, opmDir)
    require.NoError(t, err)
    
    assert.FileExists(t, symlinkTarget)   // symlink exists
    resolved, _ := os.Readlink(symlinkTarget)
    assert.Equal(t, opmDir, resolved)
}
```

**Why NOT:**
- `github.com/vektra/mockery` — for mocking interfaces; opm's filesystem operations are thin enough that `os.MkdirTemp` + real filesystem in tests is simpler and more trustworthy than mocking `os.Symlink`
- `gocheck` — obsolete; testify is the modern replacement

---

### Linting / Code Quality (Dev Tooling)

| Tool | Version | Purpose |
|------|---------|---------|
| `golangci-lint` | v2.x | Aggregated linter (vet, staticcheck, errcheck, etc.) |

**Confidence: HIGH** — golangci-lint v2 is current standard for Go projects. GoReleaser's own projects use it.

---

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

---

## Prior Art: docker context vs. opm

Docker's `docker context use` (verified from source) uses:
- **Cobra** for the command definition (identical pattern to what we'll build)
- **JSON config file** for storing current context name in `~/.docker/config.json`
- **No symlinks** — docker context uses a config key, not filesystem symlinks

Key structural lesson from docker's implementation:
```go
// docker/cli source (docker context use.go)
func newUseCommand(dockerCLI command.Cli) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "use CONTEXT",
        Short: "Set the current docker context",
        Args:  cobra.ExactArgs(1),
        RunE:  func(cmd *cobra.Command, args []string) error {
            return runUse(dockerCLI, args[0])
        },
        ValidArgsFunction: completeContextNames(dockerCLI, 1, false),  // tab completion!
    }
    return cmd
}
```

opm's `context use` should follow this exact pattern. Note `ValidArgsFunction` — this is how profile names become tab-completable. This is Cobra's built-in completion system, no external library needed.

---

## Installation

```bash
# Initialize Go module
go mod init github.com/<user>/opm

# CLI framework
go get github.com/spf13/cobra@v1.10.2

# Testing assertions
go get github.com/stretchr/testify@latest

# Dev: release tooling (install separately, not as a dep)
# brew install goreleaser
# or: go install github.com/goreleaser/goreleaser/v2@latest
```

**No other dependencies required.** All filesystem operations, path handling, and I/O use Go stdlib exclusively.

---

## Go Version

**Use Go 1.22+** (minimum) — `t.TempDir()` automatic cleanup, improved error wrapping with `%w`. Current stable is **Go 1.26.2** (confirmed from local environment). Target `go 1.22` in `go.mod` for broad compatibility unless a specific 1.23+ feature is needed.

```
// go.mod
go 1.22
```

---

## Sources

- Cobra releases: https://github.com/spf13/cobra/releases (v1.10.2 confirmed current)
- GoReleaser releases: https://github.com/goreleaser/goreleaser/releases (v2.15.3, 2026-04-15)
- GoReleaser Homebrew docs: Context7 `/goreleaser/goreleaser` (HIGH reputation)
- Testify: Context7 `/stretchr/testify` (HIGH reputation, v1 stable)
- Docker context use.go source: https://github.com/docker/cli/blob/master/cli/command/context/use.go (verified Cobra usage)
- Go os package: https://pkg.go.dev/os (stdlib, HIGH confidence)
