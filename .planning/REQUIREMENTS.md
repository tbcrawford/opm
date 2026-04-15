# opm — Requirements

## v1 Requirements

### Core Profile Operations

- [ ] **CORE-01**: User can run `opm init` to migrate existing `~/.config/opencode` into a `default` profile and symlink it — with crash-safe 3-step ordering
- [ ] **CORE-02**: User can run `opm init` on a fresh system (no existing `~/.config/opencode`) and get a valid opm structure
- [ ] **CORE-03**: User can run `opm context create <name>` to create a new empty profile directory
- [ ] **CORE-04**: User can run `opm context use <name>` to atomically switch the `~/.config/opencode` symlink to the named profile
- [ ] **CORE-05**: User can run `opm context ls` to list all profiles with the active one marked with `*`
- [ ] **CORE-06**: User can run `opm context inspect <name>` to view profile metadata (path, active status, contents summary)
- [ ] **CORE-07**: User can run `opm context rm <name>` to delete a non-active profile
- [ ] **CORE-08**: User can run `opm context show` to print the name of the currently active profile

### Safety & Diagnostics

- [ ] **SAFE-01**: `opm context rm <name>` is blocked when `<name>` is the active profile, with a clear error; user can override with `--force`
- [ ] **SAFE-02**: `opm context ls` detects and flags dangling symlinks (profile dir was deleted while symlink still points to it)
- [ ] **SAFE-03**: User can run `opm doctor` to check symlink health, confirm `~/.config/opencode` is an opm-managed symlink, and report on profile integrity

### Polish

- [ ] **POLISH-01**: Shell completion works for all commands and profile name arguments (zsh, bash, fish via cobra `ValidArgsFunction`)
- [ ] **POLISH-02**: Profile names are validated on create/use — alphanumeric, dash, underscore only; clear error on invalid name

### Power Features

- [ ] **POWER-01**: User can run `opm context rename <old> <new>` to rename a profile (including updating the symlink if it was active)

### Distribution

- [ ] **DIST-01**: Project ships via GoReleaser with GitHub Actions CI producing release binaries for macOS (arm64 + amd64) and Linux
- [ ] **DIST-02**: Project is installable via `brew install` from a Homebrew tap

---

## v2 Requirements (Deferred)

- `opm context copy <src> <dst>` — full profile duplication; deferred due to node_modules size concerns (needs progress indication)
- `--format json` / `-q` quiet flags — machine-readable output; useful for scripting but not blocking for v1
- `opm doctor --fix` auto-repair mode

---

## Out of Scope

- **Env var / shell integration approach** — symlinks are persistent across shell sessions; no shell rc integration needed
- **Profile sync across machines** — local filesystem only; git-based sync is user's responsibility
- **Profile inheritance / base profiles** — each profile is fully independent; merging OpenCode configs is OpenCode's job, not opm's
- **Windows support** — `os.Symlink` requires elevated privileges on Windows; macOS/Linux only for v1
- **Interactive profile picker (fzf-style)** — explicit naming is clearer for a system-config tool
- **Metadata editing** (`opm context update --description`) — profile contents speak for themselves

---

## Traceability

*(Filled by roadmap agent)*

| REQ-ID | Phase |
|--------|-------|
| CORE-01 | — |
| CORE-02 | — |
| CORE-03 | — |
| CORE-04 | — |
| CORE-05 | — |
| CORE-06 | — |
| CORE-07 | — |
| CORE-08 | — |
| SAFE-01 | — |
| SAFE-02 | — |
| SAFE-03 | — |
| POLISH-01 | — |
| POLISH-02 | — |
| POWER-01 | — |
| DIST-01 | — |
| DIST-02 | — |
