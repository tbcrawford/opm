# opm — OpenCode Profile Manager

Switch between completely isolated OpenCode environments with a single command.

```
opm context use work
```

Each profile is an independent `~/.config/opencode/` directory — different MCPs, agents, models, plugins, and `AGENTS.md` files. Switching is instant and takes effect without restarting anything.

## How it works

`opm init` moves your existing `~/.config/opencode/` into a named profile directory and replaces it with a symlink. From that point on, `opm context use <name>` atomically repoints the symlink to a different profile directory.

```
~/.config/opencode  →  ~/.config/opm/profiles/work/
```

All OpenCode reads/writes go to the active profile transparently.

## Installation

### Homebrew (macOS)

```sh
brew install opm-cli/tap/opm
```

### Download binary

Download the latest release from [GitHub Releases](https://github.com/opm-cli/opm/releases), extract, and place `opm` somewhere in your `$PATH`.

### Build from source

```sh
go install github.com/opm-cli/opm@latest
```

## Quick start

```sh
# 1. Migrate your existing OpenCode config into a profile named "default"
opm init

# 2. Create additional profiles
opm context create work
opm context create personal

# 3. Switch profiles
opm context use work

# 4. See what's active
opm context show

# 5. List all profiles
opm context ls
```

## Commands

### `opm init`

Migrates your existing `~/.config/opencode/` into opm management. Creates a profile named `default` from your current config and installs the symlink. Safe to run on an existing setup — non-destructive.

### `opm context create <name>`

Creates a new empty profile directory. Profile names may contain letters, digits, `-`, `_`, and `.` (max 63 characters, must start with a letter or digit).

### `opm context use <name>`

Switches the active profile. The symlink swap is atomic — no window where `~/.config/opencode` is absent.

### `opm context ls`

Lists all profiles. The active profile is marked with `*`. Dangling profiles (directory missing) are marked with `!`.

```
  NAME       PATH
* work       /Users/you/.config/opm/profiles/work
  personal   /Users/you/.config/opm/profiles/personal
  default    /Users/you/.config/opm/profiles/default
```

### `opm context show`

Prints the name of the currently active profile.

### `opm context inspect <name>`

Shows detailed information about a profile: name, path, active status, and whether the directory exists.

### `opm context rename <old> <new>`

Renames a profile. If the renamed profile is currently active, the symlink is updated atomically.

### `opm context rm <name>`

Removes a profile. Refuses to remove the active profile unless `--force` is passed, in which case opm auto-switches to another available profile first.

### `opm doctor`

Runs a series of health checks on the opm installation and reports any issues. Exits with code 1 if any check fails.

```sh
opm doctor
✓ ~/.config/opencode is a symlink
✓ Symlink target exists (not dangling)
✓ All profile directories are valid
✓ Current file matches active symlink
```

### Shell completion

```sh
# bash
opm completion bash > /etc/bash_completion.d/opm

# zsh
opm completion zsh > "${fpath[1]}/_opm"

# fish
opm completion fish > ~/.config/fish/completions/opm.fish
```

Profile names are tab-completed for `use`, `inspect`, `rm`, and `rename`.

## Storage layout

```
~/.config/opm/
├── current              # name of the active profile (plain text)
└── profiles/
    ├── default/         # profile directories (full OpenCode configs)
    ├── work/
    └── personal/

~/.config/opencode  →  ~/.config/opm/profiles/work/   # symlink
```

## License

MIT
