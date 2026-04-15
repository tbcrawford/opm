# Architecture Patterns

**Project:** opm — OpenCode Profile Manager
**Domain:** Go CLI / symlink-based config profile switching
**Researched:** 2026-04-15
**Confidence:** HIGH (Go stdlib, verified patterns from docker/cli source)

---

## Recommended Architecture

opm is a small, single-purpose CLI. The architecture follows the docker/cli pattern: a thin Cobra command layer delegates to a small number of internal packages. There is no server, no daemon, no network — just filesystem operations coordinated through a central `store` package.

```
opm/
├── main.go                  ← entry point, executes root command
├── cmd/                     ← cobra command wiring only (no business logic)
│   ├── root.go              ← root command, global flags, adds subcommands
│   ├── init.go              ← opm init
│   ├── context.go           ← opm context (parent)
│   ├── context_create.go    ← opm context create <name>
│   ├── context_use.go       ← opm context use <name>
│   ├── context_ls.go        ← opm context ls
│   ├── context_inspect.go   ← opm context inspect <name>
│   └── context_rm.go        ← opm context rm <name>
├── internal/
│   ├── store/               ← all state reads/writes (profiles, current)
│   │   ├── store.go         ← Store type, paths, open/load
│   │   ├── profile.go       ← Profile type, CRUD
│   │   └── current.go       ← current-file read/write
│   ├── symlink/             ← all symlink operations (detect, create, swap)
│   │   └── symlink.go
│   └── paths/               ← canonical path resolution (~/.config/opm, etc.)
│       └── paths.go
└── go.mod
```

---

## Component Boundaries

| Component | Responsibility | Communicates With |
|-----------|---------------|-------------------|
| `cmd/` | Parse CLI args, validate inputs, call store/symlink, print output | `internal/store`, `internal/symlink` |
| `internal/store` | Read/write `~/.config/opm/` — profiles dir, current file | `internal/paths` |
| `internal/symlink` | Detect, create, and atomically swap `~/.config/opencode` symlink | Go `os` stdlib only |
| `internal/paths` | Return canonical absolute paths (`OpmDir()`, `ProfileDir(name)`, `OpencodeConfigDir()`) | Nothing (pure computation) |

**Rule:** `cmd/` never touches the filesystem directly. `internal/store` never calls symlink operations. `internal/symlink` knows nothing about profile names — it only operates on paths it receives.

---

## Data Flow

### `opm init`

```
cmd/init.go
  → paths.OpencodeConfigDir()     → "~/.config/opencode" (absolute)
  → symlink.Inspect(path)         → {Exists: true, IsSymlink: false, ...}
  → store.Init()                  → creates ~/.config/opm/ structure
  → os.Rename(opencodeDir, profileDir)  → moves real dir to profile location
  → symlink.SetAtomic(profileDir, opencodeConfigPath)  → creates symlink
  → store.SetCurrent("default")   → writes ~/.config/opm/current
  → print confirmation
```

### `opm context use <name>`

```
cmd/context_use.go
  → store.GetProfile(name)        → validates profile exists
  → symlink.SetAtomic(profileDir, opencodeConfigPath)  → atomic swap
  → store.SetCurrent(name)        → updates ~/.config/opm/current
  → print confirmation
```

### `opm context ls`

```
cmd/context_ls.go
  → store.ListProfiles()          → []Profile from filesystem scan
  → store.GetCurrent()            → string from ~/.config/opm/current
  → format and print table
```

---

## Key Design Decisions

### 1. Symlink Detection During `opm init`

Use `os.Lstat()` (not `os.Stat()`). `Lstat` inspects the link itself rather than following it, revealing `fs.ModeSymlink` in the `FileMode`.

```go
// internal/symlink/symlink.go
type Status struct {
    Exists    bool
    IsSymlink bool
    Target    string // empty if not a symlink
    IsDir     bool   // true if real directory
}

func Inspect(path string) (Status, error) {
    info, err := os.Lstat(path)
    if os.IsNotExist(err) {
        return Status{Exists: false}, nil
    }
    if err != nil {
        return Status{}, err
    }
    isSymlink := info.Mode()&os.ModeSymlink != 0
    target := ""
    if isSymlink {
        target, err = os.Readlink(path)
        if err != nil {
            return Status{}, err
        }
    }
    return Status{
        Exists:    true,
        IsSymlink: isSymlink,
        Target:    target,
        IsDir:     info.IsDir(),
    }, nil
}
```

**`opm init` decision tree:**
| Condition | Action |
|-----------|--------|
| `~/.config/opencode` does not exist | Create empty profile dir, create symlink, done |
| `~/.config/opencode` is already an opm symlink | Error: "already initialized" |
| `~/.config/opencode` is a real directory | Move to profile dir, create symlink |
| `~/.config/opencode` is a symlink to a non-opm dir | Error: "unrecognized symlink, please back up and remove first" |

### 2. Atomic Symlink Swap

`os.Rename()` is atomic on POSIX (Linux, macOS) for same-filesystem moves. The canonical pattern for atomic symlink replacement:

```go
// internal/symlink/symlink.go
func SetAtomic(target, linkPath string) error {
    // Create temp symlink in the same directory (same filesystem guaranteed)
    dir := filepath.Dir(linkPath)
    tmpLink := filepath.Join(dir, ".opm_tmp_"+filepath.Base(linkPath))

    // Clean up any stale tmp on entry
    _ = os.Remove(tmpLink)

    if err := os.Symlink(target, tmpLink); err != nil {
        return fmt.Errorf("create temp symlink: %w", err)
    }

    // Atomic replace: rename(2) replaces atomically on POSIX
    if err := os.Rename(tmpLink, linkPath); err != nil {
        _ = os.Remove(tmpLink) // cleanup on failure
        return fmt.Errorf("atomic swap: %w", err)
    }
    return nil
}
```

**Why this is safe:**
- Between the `Symlink` and `Rename` calls, the old symlink still points to the old profile
- After `Rename`, any process opening `~/.config/opencode` sees the new profile immediately
- There is never a moment where `~/.config/opencode` does not exist or is broken
- If the process is killed between the two calls, only the temp file is orphaned (harmless)

### 3. State File Design: `~/.config/opm/current`

**Use plain text, not JSON.** The value is a single profile name — a string. JSON is overhead with no benefit for a single scalar value. Mirrors Docker's approach where `currentContext` is a string field in `~/.docker/config.json`, but for opm the entire file IS the current profile name.

```
~/.config/opm/current
```
Contents (just the name, no newline ceremony required, trim on read):
```
work
```

**Read:**
```go
func (s *Store) GetCurrent() (string, error) {
    data, err := os.ReadFile(s.currentPath())
    if os.IsNotExist(err) {
        return "", nil // uninitialized
    }
    return strings.TrimSpace(string(data)), err
}
```

**Write:**
```go
func (s *Store) SetCurrent(name string) error {
    return os.WriteFile(s.currentPath(), []byte(name+"\n"), 0o644)
}
```

### 4. Profile Storage Layout

```
~/.config/opm/
├── current                        ← plain text: active profile name
└── profiles/
    ├── default/                   ← actual opencode config files live here
    │   ├── opencode.json
    │   ├── AGENTS.md
    │   ├── agents/
    │   └── ...
    └── work/
        ├── opencode.json
        └── ...
```

**No per-profile metadata file.** The profile name IS the directory name. There is no `profile.json` or sidecar metadata — the profile is fully opaque from opm's perspective (OpenCode owns the contents). The only discoverable metadata is:
- Name: directory name under `profiles/`
- Active: whether it matches `current`
- Created: directory mtime (available from `os.Stat`)

If future metadata is needed (description, aliases), add `~/.config/opm/profiles/<name>/.opm` as an opt-in file. Don't create it in v1.

**Profile validation:** A profile is valid if `~/.config/opm/profiles/<name>/` exists as a real directory. No deeper validation.

### 5. Command → Package Mapping

Following kubectl and docker/cli patterns: each `cmd/*.go` file wires one command and calls one or two `internal/` functions. The command file is glue, not logic.

```
opm init
  cmd/init.go → store.Init() + symlink.Inspect() + symlink.SetAtomic() + store.SetCurrent()

opm context create <name>
  cmd/context_create.go → store.CreateProfile(name)

opm context use <name>
  cmd/context_use.go → store.GetProfile(name) + symlink.SetAtomic() + store.SetCurrent()

opm context ls
  cmd/context_ls.go → store.ListProfiles() + store.GetCurrent()

opm context inspect <name>
  cmd/context_inspect.go → store.GetProfile(name) + symlink.Inspect()

opm context rm <name>
  cmd/context_rm.go → store.GetCurrent() [guard] + store.DeleteProfile(name)
```

**Cobra tree:**

```go
// cmd/root.go
rootCmd
  ├── init          (cmd/init.go)
  └── context       (cmd/context.go — parent, no-op RunE)
        ├── create  (cmd/context_create.go)
        ├── use     (cmd/context_use.go)
        ├── ls      (cmd/context_ls.go)
        ├── inspect (cmd/context_inspect.go)
        └── rm      (cmd/context_rm.go)
```

Each command file has its own `init()` that calls `contextCmd.AddCommand(...)`. The root init registers `contextCmd`. This is the standard Cobra modular pattern — no cyclic imports, each file is self-contained.

### 6. `~/.config/opencode` Does Not Exist

This is the simplest and cleanest case during `opm init`. Decision:

```
opencode dir does not exist:
  → Create an empty profile dir at ~/.config/opm/profiles/default/
  → Create symlink: ~/.config/opencode → ~/.config/opm/profiles/default/
  → Write "default" to ~/.config/opm/current
  → Print: "Initialized opm. Active profile: default (empty — OpenCode will populate on first start)"
```

OpenCode handles empty config dirs gracefully per PROJECT.md. No pre-population needed.

---

## Store Interface

The `internal/store` package should expose a `Store` struct initialized with an `OpmDir` path, not global variables. This makes testing straightforward (inject a temp dir).

```go
// internal/store/store.go
type Store struct {
    root string // e.g. ~/.config/opm
}

func New(root string) *Store {
    return &Store{root: root}
}

func (s *Store) ProfileDir(name string) string {
    return filepath.Join(s.root, "profiles", name)
}

func (s *Store) ListProfiles() ([]string, error) { ... }
func (s *Store) GetProfile(name string) (string, error) { ... } // returns dir path
func (s *Store) CreateProfile(name string) error { ... }
func (s *Store) DeleteProfile(name string) error { ... }
func (s *Store) GetCurrent() (string, error) { ... }
func (s *Store) SetCurrent(name string) error { ... }
func (s *Store) Init() error { ... } // creates ~/.config/opm/ structure
```

Commands instantiate the store from `paths.OpmDir()`:

```go
// cmd/root.go
func newStore() *store.Store {
    return store.New(paths.OpmDir())
}
```

---

## Paths Package

```go
// internal/paths/paths.go
func OpmDir() string {
    // Uses os.UserConfigDir() for correctness (respects XDG_CONFIG_HOME)
    // Falls back to ~/.config/opm
    base, err := os.UserConfigDir()
    if err != nil {
        base = filepath.Join(os.Getenv("HOME"), ".config")
    }
    return filepath.Join(base, "opm")
}

func OpencodeConfigDir() string {
    base, err := os.UserConfigDir()
    if err != nil {
        base = filepath.Join(os.Getenv("HOME"), ".config")
    }
    return filepath.Join(base, "opencode")
}
```

Using `os.UserConfigDir()` (HIGH confidence — Go stdlib) means opm correctly respects `XDG_CONFIG_HOME` on Linux without special-casing it.

---

## Patterns to Follow

### Pattern: Thin Commands, Fat Internal Packages

**What:** cmd/ files contain only cobra wiring and output formatting. All logic lives in `internal/`.

**When:** Always.

```go
// Good — cmd/context_use.go
func runContextUse(s *store.Store, name string) error {
    profileDir, err := s.GetProfile(name)
    if err != nil {
        return err
    }
    if err := symlink.SetAtomic(profileDir, paths.OpencodeConfigDir()); err != nil {
        return err
    }
    return s.SetCurrent(name)
}
```

### Pattern: Fail-Fast on Dangerous Operations

**What:** `context rm` must guard against deleting the active profile. Check before delete.

```go
func (s *Store) DeleteProfile(name string) error {
    current, _ := s.GetCurrent()
    if current == name {
        return fmt.Errorf("cannot delete active profile %q — switch to another profile first", name)
    }
    return os.RemoveAll(s.ProfileDir(name))
}
```

### Pattern: Store Init is Idempotent

**What:** `store.Init()` uses `os.MkdirAll()` — safe to call multiple times.

```go
func (s *Store) Init() error {
    return os.MkdirAll(filepath.Join(s.root, "profiles"), 0o755)
}
```

---

## Anti-Patterns to Avoid

### Anti-Pattern 1: Using `os.Stat` to Detect Symlinks

**What:** `os.Stat` follows the symlink and reports the target's info. `info.Mode()&os.ModeSymlink` will always be false.
**Why bad:** Init logic incorrectly treats a symlink as a real directory and tries to move it.
**Instead:** Always use `os.Lstat` to inspect the link itself.

### Anti-Pattern 2: Remove-Then-Create for Symlink Swap

**What:** `os.Remove(linkPath)` then `os.Symlink(target, linkPath)`
**Why bad:** There is a window between Remove and Symlink where `~/.config/opencode` does not exist. Any process opening that path during the gap sees an error.
**Instead:** Use the atomic create-temp + rename pattern (see above).

### Anti-Pattern 3: Storing Business Logic in `cmd/`

**What:** Putting symlink inspection or profile validation directly in cobra `RunE` functions.
**Why bad:** Untestable. All non-trivial tests require a real cobra command invocation.
**Instead:** Extract to `internal/` functions that accept injected paths/stores.

### Anti-Pattern 4: Hardcoding `~/.config/opencode`

**What:** `const opencodeDir = os.Getenv("HOME") + "/.config/opencode"`
**Why bad:** Breaks on Linux with non-standard `XDG_CONFIG_HOME`, and is wrong for users with custom configs.
**Instead:** Use `os.UserConfigDir()` from Go stdlib.

### Anti-Pattern 5: Profile Name as Path Component Without Validation

**What:** Allowing `/`, `..`, spaces in profile names passed directly to `filepath.Join`.
**Why bad:** Path traversal — a profile named `../evil` would escape the profiles directory.
**Instead:** Validate profile names at the cmd layer: `^[a-zA-Z0-9][a-zA-Z0-9_.-]*$` (same pattern docker/cli uses for context names).

---

## Scalability Considerations

opm is a CLI tool invoked once per switch event — there are no scalability concerns in the traditional sense. The relevant "scale" is number of profiles:

| Concern | With 5 profiles | With 50 profiles | Notes |
|---------|----------------|------------------|-------|
| `ls` performance | Instant | Instant | Directory scan, no recursion |
| Storage | Megabytes | Gigabytes (node_modules!) | Not opm's concern |
| Symlink switch speed | <1ms | <1ms | Single atomic rename |

---

## Suggested Build Order

This order matters for the roadmap because each layer depends on the one below it.

```
Layer 1: internal/paths  ← no dependencies, pure computation
  ↓
Layer 2: internal/store  ← depends on paths
  ↓
Layer 3: internal/symlink ← depends on nothing (pure os calls)
  ↓
Layer 4: cmd/init + cmd/context/* ← depends on store + symlink
  ↓
Layer 5: main.go ← wires cmd/ only
```

**Implied phase order:**

1. **Foundation** — `paths`, `store` (Init, ListProfiles, GetCurrent, SetCurrent), `symlink` (Inspect, SetAtomic). These are testable in isolation.
2. **Core commands** — `opm init`, `opm context use`, `opm context ls`. The three commands covering the full loop: onboard → switch → list.
3. **Remaining commands** — `opm context create`, `opm context inspect`, `opm context rm`. Less critical but complete the surface.
4. **Polish** — Profile name validation, error messages, shell completions, `--help` tuning.

Build Layer 1–3 fully (with tests) before touching any `cmd/` code. Commands are thin; the core complexity is all in `store` and `symlink`.

---

## Sources

- Docker CLI source — `cli/context/store/store.go`, `cli/command/context/use.go` (github.com/docker/cli, master)
- Kubectl config command — `pkg/cmd/config/config.go` (github.com/kubernetes/kubectl, master)
- Go stdlib — `os.Lstat`, `os.Rename`, `os.Symlink`, `os.UserConfigDir` (pkg.go.dev, go1.26.2)
- Cobra documentation — command tree organization patterns (github.com/spf13/cobra)
- POSIX specification — `rename(2)` atomicity guarantee (HIGH confidence, well-established)
