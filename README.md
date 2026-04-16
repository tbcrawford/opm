<div align="center">

<img src="assets/banner.png" alt="opm — OpenCode Profile Manager" width="720"/>

Switch between completely isolated OpenCode environments —<br>different MCPs, agents, models, and configs — with a single command.

```sh
brew install tbcrawford/tap/opm
```

[![License: MIT](https://img.shields.io/badge/License-MIT-000000?style=flat-square)](LICENSE)&nbsp;&nbsp;[![Go](https://img.shields.io/badge/Go_1.21+-00ADD8?style=flat-square&logo=go&logoColor=white)](https://go.dev)&nbsp;&nbsp;[![Release](https://img.shields.io/github/v/release/tbcrawford/opm?style=flat-square&color=000000)](https://github.com/tbcrawford/opm/releases)

</div>

```
# migrate your existing config once
❯ opm init
✓ Initialized opm
  Created default profile at ~/.config/opm/profiles/default/

# create profiles for every context
❯ opm create work
✓ Created profile work
  ~/.config/opm/profiles/work/

❯ opm create experiments --from work
✓ Created profile experiments from work
  ~/.config/opm/profiles/experiments/

# switch instantly
❯ opm use work
✓ default → work
  ~/.config/opencode → profiles/work

❯ opm list
○ default
○ experiments
● work
```

<br>

---

## The Problem

### Config files shouldn't slow you down.

You use OpenCode for work with strict MCPs and a locked model. Then for personal projects with different tools and a relaxed `AGENTS.md`. Then to experiment with new tooling without touching what's working. Each switch means editing files by hand — tedious, error-prone, and risky.

opm treats each context as a first-class **profile**: a full, isolated `~/.config/opencode/` directory. Switching is a single symlink swap.

<br>

**Completely isolated** — Each profile has its own MCPs, agents, models, plugins, and `AGENTS.md`. Nothing bleeds between contexts.

**Atomic switching** — The symlink swap is atomic. There is no window where `~/.config/opencode` is absent or in a bad state.

**Safe to experiment** — Clone a working profile, break things freely. Your production config is never touched. Switch back to restore it instantly.

<br>

---

## Commands

### The complete surface area.

| Command | Description |
|---|---|
| `opm init` | Migrate your existing config into opm management. Non-destructive. |
| `opm create <name>` | Create a new empty profile. Use `--from` to clone an existing profile as the starting point. |
| `opm use <name>` | Switch the active profile via atomic symlink swap. Restart OpenCode to pick up the new profile. |
| `opm list [-l]` | List all profiles. Active marked `●`. Dangling marked `!`. Pass `-l` to include paths. |
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

`opm init` moves your existing `~/.config/opencode/` into a named profile directory and replaces it with a symlink. From that point on, `opm use <name>` atomically repoints the symlink to a different profile:

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
