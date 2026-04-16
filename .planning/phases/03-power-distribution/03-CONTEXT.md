# Phase 3: Power Features + Distribution — Context

**Gathered:** 2026-04-15
**Status:** Ready for planning

<domain>
## Phase Boundary

Phase 3 completes v1. After this phase: users can rename profiles, install opm via Homebrew, and every merged tag triggers an automated GitHub release with signed binaries.

### Requirements
- **POWER-01**: `opm context rename <old> <new>`
- **DIST-01**: GoReleaser + GitHub Actions release pipeline (macOS universal + Linux arm64/amd64)
- **DIST-02**: Homebrew tap installable via `brew install tbcrawford/tap/opm`

</domain>

<decisions>
## Implementation Decisions

### POWER-01: `opm context rename <old> <new>`

- **D-01:** `rename` is implemented as a **new method on `store.Store`**: `RenameProfile(oldName, newName string) error`. The command is thin wiring only.
- **D-02:** Rename is a **3-step sequence** (crash-safe):
  1. Validate `newName` passes the allowlist regex (same as `CreateProfile`)
  2. Validate `oldName` profile exists; validate `newName` does NOT already exist
  3. `os.Rename(oldDir, newDir)` — atomic on same filesystem (profiles/ is always on one fs)
  4. If `oldName` was the active profile: call `symlink.SetAtomic(newDir, opencodeDir)` + `store.SetCurrent(newName)`
- **D-03:** `os.Rename` for directories is atomic on the same filesystem (which profiles/ always is). No temp-dir intermediate needed — unlike the init migration, the symlink doesn't need to be kept live during the rename of an inactive profile.
- **D-04:** If the renamed profile IS active: update symlink first (before any other state), then update `current` file. Crash between symlink update and current-file update is recoverable — symlink is the source of truth.
- **D-05:** Output:
  - Inactive profile: `Renamed context "work" to "work-old".`
  - Active profile: `Renamed context "work" to "work-old". Active context updated.`
- **D-06:** `rename` gets `ValidArgsFunction: profileNameCompletion` for the first argument (old name). The second argument (new name) has no completion (it's a new name).
- **D-07:** Command registration: `contextCmd.AddCommand(renameCmd)` in `init()`.
- **D-08:** `managedGuard` is attached as `PersistentPreRunE`.

---

### DIST-01: GoReleaser + GitHub Actions

- **D-09:** `.goreleaser.yaml` in project root. Targets: `darwin/amd64`, `darwin/arm64` (merged into universal binary), `linux/amd64`, `linux/arm64`.
- **D-10:** Version injection via ldflags: `-X main.version={{.Version}} -X main.commit={{.Commit}}`. Add `var version = "dev"` and `var commit = "none"` to `main.go`. Set `rootCmd.Version = version` so cobra's built-in `--version` / `-v` flag works.
- **D-11:** GitHub Actions workflow at `.github/workflows/release.yml`. Trigger: `push` to tags matching `v*`. Uses `goreleaser/goreleaser-action@v6` with `version: "~> v2"`.
- **D-12:** Workflow steps: `actions/checkout@v4` (with `fetch-depth: 0`), `actions/setup-go@v5`, `goreleaser-action@v6`. No Docker, no CGO, no signing (keep it simple for v1).
- **D-13:** `CGO_ENABLED=0` — pure Go binary, no C dependencies.
- **D-14:** Archives: `tar.gz` for Linux, `zip` for macOS (GoReleaser default behavior for these platforms).
- **D-15:** Checksums file included in release (GoReleaser default).
- **D-16:** Also add a `ci.yml` workflow for running `go test ./...` on every push/PR to `main`. This is separate from the release workflow. Uses `actions/setup-go@v5`.

---

### DIST-02: Homebrew Tap

- **D-17:** Tap repo: `tbcrawford/homebrew-tap` (GitHub org matches module path). GoReleaser pushes the formula automatically on release.
- **D-18:** Formula directory: `Formula/` inside the tap repo.
- **D-19:** Tap requires a `GH_PAT` secret in the main repo with `repo` scope to push to the tap repo. Document this in the workflow file as a comment.
- **D-20:** Install command in formula: `bin.install "opm"`.
- **D-21:** Test stanza: `system "#{bin}/opm", "--version"` — verifies the binary works after brew install.
- **D-22:** The `.goreleaser.yaml` `brews` section uses `repository.owner: tbcrawford` and `repository.name: homebrew-tap`.
- **D-23:** Homebrew tap setup is **configuration-only in this phase** — the actual tap repo (`tbcrawford/homebrew-tap`) is a prerequisite that must exist on GitHub for the formula push to succeed. Document this in the `.goreleaser.yaml` with a comment.

---

### Agent Discretion

- Exact ldflags format (GoReleaser template syntax is stable; match the docs)
- Go version in `actions/setup-go` (use `stable` or `^1.22`)
- GoReleaser `dist` directory name (default: `dist/`)
- Whether to add a `.gitignore` entry for `dist/`

</decisions>

<canonical_refs>
## Canonical References

### GoReleaser
- GoReleaser v2 YAML reference: Context7 `/websites/goreleaser` (HIGH, 2044 snippets)
- Key fields: `builds`, `universal_binaries`, `brews`, `archives`, `checksum`
- GitHub Actions action: `goreleaser/goreleaser-action@v6` with `version: "~> v2"`

### Existing Code
- `main.go` — add `version` and `commit` vars here; set `rootCmd.Version`
- `cmd/root.go` — `rootCmd.Version = version` wired at startup
- `internal/store/store.go` — add `RenameProfile` method
- `cmd/completion.go` — `profileNameCompletion` reused on rename's first arg

</canonical_refs>

<code_context>
## Existing Code Insights

### main.go current state
```go
package main

import "github.com/tbcrawford/opm/cmd"

func main() {
    cmd.Execute()
}
```
Needs: `var version = "dev"` and `var commit = "none"` added, plus `cmd.SetVersion(version)` or direct `rootCmd.Version` assignment.

### Integration Points (new)
- `store.RenameProfile(old, new string) error` → used by `cmd/context_rename.go`
- `rootCmd.Version` → set from `main.version` ldflags var → enables `opm --version`
- `.goreleaser.yaml` → consumed by `goreleaser/goreleaser-action@v6`
- `.github/workflows/release.yml` → triggered on `v*` tag push
- `.github/workflows/ci.yml` → triggered on push/PR to main

</code_context>

<specifics>
## Specific Notes

- `opm --version` output (cobra default): `opm version v0.1.0` — acceptable for v1
- GoReleaser `version_scheme: semver` — validates tags are valid semver (v0.1.0, v1.0.0, etc.)
- Add `dist/` to `.gitignore`
- The `goreleaser check` command validates the YAML locally: good to run in CI on non-tag pushes
- `fetch-depth: 0` in checkout is required for GoReleaser to read git history for changelog

</specifics>

<deferred>
## Deferred

- Code signing / notarization (macOS Gatekeeper) — v2
- Linux package managers (apt/yum) — v2
- `goreleaser check` in CI on PRs — nice to have but not blocking
- `opm context copy` — v2 backlog
- Windows support — explicitly out of scope for v1

</deferred>

---

*Phase: 03-power-distribution*
*Context gathered: 2026-04-15*
