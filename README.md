# opm — OpenCode Profile Manager

> Switch between completely isolated OpenCode environments with a single command.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/opm-cli/opm)](https://github.com/opm-cli/opm/releases)

---

## The problem

You use OpenCode for work. Different MCPs, a strict `AGENTS.md`, a particular model locked in. Then you want to use it for a personal project — different tools, relaxed settings, different agents. Or you want to experiment with a new MCP or agent setup without touching your working config. Right now, all of that means manually editing config files every time you switch. It's tedious. It's risky. It breaks your flow.

## The solution

```sh
opm use work
```

One command. Each **profile** is a completely isolated `~/.config/opencode/` directory — its own MCPs, agents, models, plugins, and `AGENTS.md`. Switching atomically repoints the symlink. Restart OpenCode and you're in a different environment entirely.

---

## How it works

`opm init` migrates your existing config into a named profile and replaces `~/.config/opencode` with a symlink. From that point on, switching profiles is a single symlink swap:

```
~/.config/opencode  →  ~/.config/opm/profiles/work/
```

Everything OpenCode reads and writes goes to the active profile. No duplication. No manual copying. No state drift.

---

## Installation

**Homebrew** (macOS)
```sh
brew install opm-cli/tap/opm
```

**Download binary**

Grab the latest from [GitHub Releases](https://github.com/opm-cli/opm/releases), extract, and place `opm` in your `$PATH`.

**Build from source**
```sh
go install github.com/opm-cli/opm@latest
```

---

## Quick start

```sh
# Migrate your existing config into a profile named "default"
opm init

# Create profiles for different contexts
opm create work
opm create personal
opm create experiments

# Or clone an existing profile as a starting point
opm create sandbox --from work

# Switch profiles (then restart OpenCode)
opm use work

# See what's active
opm show

# List everything
opm list
```

```
  NAME          
* work          
  personal      
  experiments   
  default       
```

---

## Commands

### `opm init`

Migrates your existing `~/.config/opencode/` into opm management. Creates a profile named `default` from your current config and installs the symlink. Non-destructive — safe to run on an existing setup.

### `opm create <name>`

Creates a new, empty profile directory. Use `--from <profile>` to clone an existing profile as the starting point.

```sh
opm create experiments --from work
```

Profile names support letters, digits, `-`, `_`, and `.` (max 63 characters, must start with a letter or digit).

### `opm use <name>`

Switches the active profile. The symlink swap is atomic — there is no window where `~/.config/opencode` is absent or invalid. Restart OpenCode to pick up the new profile.

### `opm list`

Lists all profiles. The active profile is marked `*`. Dangling profiles (missing directory) are marked `!`. Pass `-l` to include paths.

```sh
opm list -l
```

```
  NAME          PATH
* work          ~/.config/opm/profiles/work
  personal      ~/.config/opm/profiles/personal
  default       ~/.config/opm/profiles/default
```

### `opm show`

Prints the name of the currently active profile.

### `opm inspect <name>`

Shows detailed information about a profile: name, path, active status, and its contents.

### `opm copy <src> <dst>`

Copies an existing profile to a new name. Useful for snapshotting a known-good config before experimenting.

### `opm rename <old> <new>`

Renames a profile. If the renamed profile is currently active, the symlink is updated atomically.

### `opm remove <name> [name...]`

Removes one or more profiles. Refuses to remove the active profile unless `--force` is passed — in that case, opm switches to another available profile first.

### `opm path <name>`

Prints the absolute filesystem path to a profile directory. Useful for scripting.

```sh
code $(opm path work)
```

### `opm doctor`

Runs health checks on your opm installation and reports any issues. Exits with code 1 if any check fails.

```
Symlink
✓ ~/.config/opencode → work

Profiles
✓ work
✓ personal
✓ default

All checks passed.
```

### `opm reset`

Removes opm's symlink and restores `~/.config/opencode` as a plain directory (copied from the active profile). All profiles in `~/.config/opm/profiles/` are left intact. Use this to hand control back to OpenCode directly.

### Shell completion

```sh
# bash
opm completion bash > /etc/bash_completion.d/opm

# zsh
opm completion zsh > "${fpath[1]}/_opm"

# fish
opm completion fish > ~/.config/fish/completions/opm.fish
```

Profile names are tab-completed for `use`, `inspect`, `remove`, `copy`, `path`, and `rename`.

---

## Storage layout

```
~/.config/opm/
├── current                  # active profile name (plain text)
└── profiles/
    ├── default/             # full OpenCode config directories
    ├── work/
    └── personal/

~/.config/opencode  →  ~/.config/opm/profiles/work/
```

---

## License

MIT
