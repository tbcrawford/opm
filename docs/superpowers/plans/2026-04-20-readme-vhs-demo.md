# README VHS Demo Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a checked-in VHS quick-start demo GIF plus editable `.tape` source, wire it into `README.md`, and expose one `just` command to regenerate it.

**Architecture:** Keep the implementation narrow: one tape file, one generated GIF, one README embed, and one `just` recipe. The tape should run real `opm` commands in an isolated temporary `HOME`, with hidden setup and cleanup so the visible recording stays focused on `init -> create -> use -> list`.

**Tech Stack:** VHS, GIF output, Markdown, `just`, existing Go CLI via `just run`

---

## File Map

- Create: `demo/readme-quick-start.tape`
  - canonical VHS source for the README quick-start demo
- Create: `assets/demo/readme-quick-start.gif`
  - generated demo asset embedded in the README
- Modify: `README.md`
  - embed the generated GIF directly under `## Quick Start`
- Modify: `justfile`
  - add a repo-standard regeneration command for the demo asset
- Reference: `docs/superpowers/specs/2026-04-20-readme-vhs-demo-design.md`
  - source of truth for placement, scope, and maintenance expectations

### Task 1: Add The VHS Tape Source And Render Command

**Files:**
- Create: `demo/readme-quick-start.tape`
- Modify: `justfile`
- Reference: `docs/superpowers/specs/2026-04-20-readme-vhs-demo-design.md:46-102`

- [ ] **Step 1: Confirm there is no existing demo tape or generated GIF path yet**

Run: `rg -n "readme-quick-start|vhs|assets/demo" README.md justfile demo assets`
Expected: no existing `readme-quick-start` tape or README demo asset wiring.

- [ ] **Step 2: Create the tape file with real-command, temp-home setup**

Write `demo/readme-quick-start.tape` with this exact starting content:

```tape
Output assets/demo/readme-quick-start.gif

Require just
Require mkdir
Require rm
Require env

Set Shell "zsh"
Set FontSize 22
Set Width 920
Set Height 640
Set Padding 24
Set Margin 12
Set BorderRadius 8
Set Framerate 30
Set PlaybackSpeed 1.0
Set CursorBlink false
Set Theme "GitHub Dark"
Set WindowBar Colorful
Set TypingSpeed 90ms

# Regenerate with: just demo-readme

Hide
Type "export DEMO_HOME=$(mktemp -d)"
Enter
Type "mkdir -p \"$DEMO_HOME/.config/opencode\""
Enter
Type "printf 'model = \"gpt-5\"\n' > \"$DEMO_HOME/.config/opencode/config.toml\""
Enter
Type "clear"
Enter
Show

Type "HOME=$DEMO_HOME just run init"
Enter
Sleep 1.2s

Type "HOME=$DEMO_HOME just run create work"
Enter
Sleep 1.0s

Type "HOME=$DEMO_HOME just run use work"
Enter
Sleep 1.0s

Type "HOME=$DEMO_HOME just run list"
Enter
Sleep 1.5s

Hide
Type "rm -rf \"$DEMO_HOME\""
Enter
```

- [ ] **Step 3: Validate the tape syntax before rendering**

Run: `vhs validate demo/readme-quick-start.tape`
Expected: validation succeeds with no parse errors.

- [ ] **Step 4: Add a `just` recipe to regenerate the GIF**

Update `justfile` by adding this recipe near the other development commands:

```just
# Regenerate the README quick-start demo GIF from its VHS tape
demo-readme:
    mkdir -p assets/demo
    vhs demo/readme-quick-start.tape
```

- [ ] **Step 5: Verify the recipe is visible and runnable in principle**

Run: `just --list`
Expected: output includes `demo-readme`.

- [ ] **Step 6: Commit**

```bash
git add demo/readme-quick-start.tape justfile
git commit -m "docs: add VHS demo source"
```

### Task 2: Render The GIF And Embed It In Quick Start

**Files:**
- Create: `assets/demo/readme-quick-start.gif`
- Modify: `README.md:16-38`
- Reference: `docs/superpowers/specs/2026-04-20-readme-vhs-demo-design.md:72-102`

- [ ] **Step 1: Render the demo GIF from the checked-in tape**

Run: `just demo-readme`
Expected: VHS completes successfully and writes `assets/demo/readme-quick-start.gif`.

- [ ] **Step 2: Confirm the generated GIF exists where the README will reference it**

Run: `rg -n "readme-quick-start\.gif" demo README.md justfile && ls assets/demo`
Expected: the GIF path is discoverable and `assets/demo/readme-quick-start.gif` exists.

- [ ] **Step 3: Embed the GIF directly under `## Quick Start`**

Update `README.md` so the section begins like this:

```markdown
## Quick Start

<p align="center">
  <img src="assets/demo/readme-quick-start.gif" alt="Terminal demo showing opm init, create work, use work, and list" width="720" />
</p>

```sh
```

Keep the existing quick-start code block and surrounding explanatory copy beneath the embed.

- [ ] **Step 4: Verify the embed sits in the correct location**

Run: `rg -n "## Quick Start|readme-quick-start\.gif" README.md`
Expected: the GIF reference appears immediately after `## Quick Start` and before the shell code block.

- [ ] **Step 5: Do a quick asset sanity check**

Read the generated GIF file metadata or open it locally to verify:

- it is the expected quick-start flow
- the terminal is legible at medium width
- the visible sequence is `init -> create -> use -> list`

- [ ] **Step 6: Commit**

```bash
git add README.md assets/demo/readme-quick-start.gif
git commit -m "docs: add README quick start demo"
```

### Task 3: Final Accuracy And Maintenance Verification

**Files:**
- Verify: `demo/readme-quick-start.tape`, `assets/demo/readme-quick-start.gif`, `README.md`, `justfile`
- Modify: same files only if verification finds a mismatch

- [ ] **Step 1: Re-run CLI verification for the commands shown in the demo**

Run these commands in the repo root:

```bash
just run init --help
just run create --help
just run use --help
just run list --help
```

Expected: the commands used in the tape remain valid and match the README quick-start story.

- [ ] **Step 2: Re-render once after any copy/timing fixes**

Run: `just demo-readme`
Expected: final committed GIF matches the final tape source.

- [ ] **Step 3: Run repository tests for final confidence**

Run: `just test`
Expected: all tests pass.

- [ ] **Step 4: Proofread the tape and README for maintainability**

Check specifically that:

- the tape comment points maintainers to `just demo-readme`
- the tape uses isolated temp-home setup and cleanup
- the README alt text is descriptive and accurate
- the embed path matches the committed GIF path exactly

- [ ] **Step 5: Commit**

```bash
git add demo/readme-quick-start.tape assets/demo/readme-quick-start.gif README.md justfile
git commit -m "docs: finalize README VHS demo"
```

## Self-Review

- Spec coverage: the plan covers checked-in tape source, checked-in GIF asset, Quick Start embed placement, a `just` regeneration command, isolated temp-home execution, and final verification against actual CLI commands.
- Placeholder scan: no `TODO`, `TBD`, or vague implementation steps remain; each task names exact files, concrete tape content, exact commands, and explicit verification targets.
- Type/signature consistency: file names, recipe names, asset paths, and command names are consistent throughout the plan (`demo/readme-quick-start.tape`, `assets/demo/readme-quick-start.gif`, `demo-readme`).
