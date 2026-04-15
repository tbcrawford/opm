# opm — Roadmap

## Phases

- [ ] **Phase 1: Foundation + Core CLI** - Internal packages + all profile commands working end-to-end
- [ ] **Phase 2: Safety, Diagnostics & Polish** - Guard rails, health checks, and shell completions
- [ ] **Phase 3: Power Features + Distribution** - Rename command and shippable release pipeline

---

## Phase Details

### Phase 1: Foundation + Core CLI
**Goal**: Users can initialize opm and perform all core profile operations from the terminal
**Depends on**: Nothing (first phase)
**Requirements**: CORE-01, CORE-02, CORE-03, CORE-04, CORE-05, CORE-06, CORE-07, CORE-08, POLISH-02
**Success Criteria** (what must be TRUE):
  1. User can run `opm init` on a machine with existing `~/.config/opencode` and it becomes a named `default` profile with a working symlink — existing config is never lost
  2. User can run `opm init` on a fresh machine and get a valid opm directory structure ready for OpenCode
  3. User can run `opm context create work && opm context use work` and immediately have `~/.config/opencode` point to the new profile — confirmed by `opm context show` printing `work`
  4. User can run `opm context ls` and see all profiles with the active one marked `*`, then `opm context inspect <name>` and see path and contents summary, then `opm context rm <name>` to remove a non-active profile
**Plans**: TBD

### Phase 2: Safety, Diagnostics & Polish
**Goal**: Users are protected from destructive mistakes and can diagnose broken installs; shell completion works
**Depends on**: Phase 1
**Requirements**: SAFE-01, SAFE-02, SAFE-03, POLISH-01
**Success Criteria** (what must be TRUE):
  1. User who tries `opm context rm <active-profile>` gets a clear error and the profile is not deleted; `--force` flag overrides and succeeds
  2. User who runs `opm context ls` after manually deleting a profile directory sees that profile flagged as dangling (not silently omitted or crashing)
  3. User who runs `opm doctor` gets a structured health report showing whether `~/.config/opencode` is an opm-managed symlink and whether all profiles resolve correctly
  4. User who types `opm context use <tab>` in zsh, bash, or fish sees their profile names as completions
**Plans**: TBD

### Phase 3: Power Features + Distribution
**Goal**: Users can rename profiles and install opm via Homebrew from a published GitHub Release
**Depends on**: Phase 2
**Requirements**: POWER-01, DIST-01, DIST-02
**Success Criteria** (what must be TRUE):
  1. User can run `opm context rename personal personal-old` and the profile directory is renamed, the `current` file updates if it was active, and `opm context ls` reflects the new name immediately
  2. User on macOS can run `brew install <tap>/opm` and get a working `opm` binary in their PATH
  3. A merged PR to `main` automatically triggers a GitHub Actions release pipeline that publishes binaries for macOS arm64, macOS amd64, and Linux via GoReleaser with checksums
**Plans**: TBD

---

## Progress

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation + Core CLI | 0/0 | Not started | - |
| 2. Safety, Diagnostics & Polish | 0/0 | Not started | - |
| 3. Power Features + Distribution | 0/0 | Not started | - |
