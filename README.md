<div align="center">

<img src="assets/banner.png" alt="opm — OpenCode Profile Manager" width="720"/>

Switch between completely isolated OpenCode environments with one command.<br>
Different MCPs, agents, models, plugins, and `AGENTS.md` files. No manual config surgery.

```sh
brew install tbcrawford/tap/opm
```

[![License: MIT](https://img.shields.io/badge/License-MIT-000000?style=flat-square)](LICENSE)&nbsp;&nbsp;[![Go](https://img.shields.io/badge/Go_1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)&nbsp;&nbsp;[![Release](https://img.shields.io/github/v/release/tbcrawford/opm?style=flat-square&color=000000)](https://github.com/tbcrawford/opm/releases)

</div>

## Quick Start

```sh
# migrate your current OpenCode config once
❯ opm init
✓ Initialized opm
  Migrated ~/.config/opencode → profiles/default

# create a second environment for a different context
❯ opm create work
✓ Created profile work
  profiles/work/

# switch instantly
❯ opm use work
✓ default → work
  ~/.config/opencode → profiles/work

❯ opm list
○ default
● work
```

Each profile is a full OpenCode config directory. Switching changes what `~/.config/opencode` points to, so OpenCode keeps using the same path it already knows.

A short terminal walkthrough will show this exact flow so you can see how little ceremony is involved.

<br>

---

## Why opm

OpenCode setups tend to drift into roles.

- **Work** needs strict MCPs, specific models, and a locked-down `AGENTS.md`.
- **Personal projects** want different tools, different defaults, and less ceremony.
- **Experiments** should be free to break without touching the setup you actually rely on.

Without profiles, switching contexts means editing files by hand, remembering what you changed last time, and hoping you undo all of it correctly.

`opm` turns each context into a first-class profile: a complete, isolated `~/.config/opencode/` directory that you can switch to with a single command.

## What a profile isolates

Every profile is its own OpenCode environment.

- MCP configuration
- agents and prompts
- model selection
- plugins and local tweaks
- `AGENTS.md` rules and project-specific behavior

Nothing leaks between profiles unless you explicitly copy it.

## Why it feels good to use

- **Fast to switch**: `opm use <name>` updates the active profile in one step.
- **Safe to experiment**: copy a working profile, try whatever you want, and switch back.
- **Transparent**: OpenCode still reads and writes `~/.config/opencode` like it always has.
- **Low overhead**: no wrapper workflow, no special edit path, no new mental model after setup.

<br>

---

## Commands

### The complete surface area.

| Command | Description |
|---|---|
| `opm init [--as <name>]` | Migrate your existing config into opm management. Non-destructive. The initial profile is named `default` unless overridden with `--as`. |
| `opm create <name>` | Create a new empty profile. Use `--from` to clone an existing profile as the starting point. |
| `opm use <name>` | Switch the active profile via atomic symlink swap. Reload OpenCode to pick up the new profile. |
| `opm list [-l]` | List all profiles. Active marked `●`. Dangling marked `✗` and shown as missing. Pass `-l` to include paths. |
| `opm show` | Print the name of the currently active profile. |
| `opm copy <src> <dst>` | Clone a profile to a new name. |
| `opm rename <old> <new>` | Rename a profile. Updates the symlink atomically if active. |
| `opm remove <name...>` | Remove one or more profiles. Refuses the active profile without `--force`. |
| `opm path <name>` | Print the absolute path to a profile directory. Useful for scripting. |
| `opm inspect <name>` | Show profile details and directory contents. |
| `opm doctor` | Run installation health checks. Exits with code 1 on failure. |
| `opm reset` | Remove opm management and restore `~/.config/opencode` as a plain directory. |

**Shell completion** — profile names are tab-completed for `use`, `copy`, `rename`, `remove`, `path`, and `inspect`:

```sh
opm completion bash > /etc/bash_completion.d/opm   # bash
opm completion zsh  > "${fpath[1]}/_opm"           # zsh
opm completion fish > ~/.config/fish/completions/opm.fish  # fish
```

<br>

---

## Install

### Up and running in under a minute.

**Homebrew** (macOS)
```sh
brew install tbcrawford/tap/opm
```

**Go**
```sh
go install github.com/tbcrawford/opm@latest
```

**Binary** — download the latest release from [GitHub Releases](https://github.com/tbcrawford/opm/releases), extract, and place `opm` in your `$PATH`.

<br>

---

## How it works

`opm init` moves your existing `~/.config/opencode/` into a named profile directory — `default` by default, or a name of your choosing with `--as` — and replaces it with a symlink. From that point on, `opm use <name>` atomically repoints the symlink to a different profile:

```
~/.config/opencode  →  ~/.config/opm/profiles/work/
```

Everything OpenCode reads and writes goes to the active profile transparently.

```
~/.config/opm/
├── current                  # active profile name (plain text)
└── profiles/
    ├── default/             # full OpenCode config directories
    ├── work/
    └── personal/
```

<br>

---

<div align="center">

MIT License · Built with Go · [Report an issue](https://github.com/tbcrawford/opm/issues)

</div>
