# README VHS Demo Design

**Date:** 2026-04-20

## Goal

Add a repo-owned VHS demo for the README that makes the `opm` quick-start flow visible at a glance, while keeping the source tape easy to edit and the generated asset easy to regenerate.

## Chosen Approach

Use a checked-in VHS tape plus a checked-in generated GIF.

- The tape runs real `opm` commands in a temporary `HOME` so the recording reflects actual behavior.
- The demo flow is `init -> create -> use -> list`.
- The GIF is embedded directly under `## Quick Start`, before the code block.
- The repo should expose a simple `just` command so the GIF can be regenerated without remembering the raw VHS invocation.

## Why This Approach

This keeps the demo trustworthy, maintainable, and aligned with the README's current structure.

- **Trustworthy** because the terminal output comes from real commands, not handwritten frames.
- **Maintainable** because the `.tape` file is committed and editable.
- **Scannable** because the GIF sits next to the quick-start section instead of pushing readers into a separate demo section.
- **Consistent with repo conventions** because regeneration should go through `just`.

## Demo Experience

The demo should show the same short adoption path the README teaches:

1. seed a minimal OpenCode config in a temporary `HOME`
2. run `opm init`
3. run `opm create work`
4. run `opm use work`
5. run `opm list`

The recording should favor readability over speed:

- medium-width terminal framing
- short pauses after each successful command
- no extra exploratory commands
- only the core path needed to understand the product

The output should stay close to the real CLI, but the README may still keep a lightly curated code block beneath the GIF for copyability and scannability.

## Files

- Add: `demo/readme-quick-start.tape`
  - canonical VHS source for the README quick-start demo
- Add: `assets/demo/readme-quick-start.gif`
  - generated asset embedded in the README
- Modify: `README.md`
  - embed the GIF directly under `## Quick Start`
- Modify: `justfile`
  - add a recipe to regenerate the demo asset from the tape

## Tape Design

The tape should:

- create and use an isolated temporary `HOME`
- export the temporary `HOME` during hidden setup so the visible demo can show plain `opm` commands
- seed `~/.config/opencode` with a minimal real directory so `opm init` migrates something meaningful
- run the built `opm` binary or the repo's standard developer entrypoint in a way that works from the repo checkout
- cleanly show the commands and success output for `init`, `create`, `use`, and `list`

The tape should not:

- rely on the user's real `~/.config`
- depend on already-initialized local state
- include unrelated commands or setup noise in the visible recording

## README Integration

The README should embed the GIF directly below `## Quick Start` and replace the now-redundant shell snippet.

The embed should:

- render at a medium width appropriate for the quick-start section
- use alt text that makes sense for readers and accessibility tools
- keep the short explanatory copy beneath it

The README should not add a large maintenance section. The demo should feel like a natural part of the quick-start section, not a separate documentation feature.

## Regeneration Workflow

Add a `just` recipe so maintainers can regenerate the GIF from the checked-in tape with one command.

The workflow should make it obvious that:

- the `.tape` file is the editable source of truth
- the GIF is derived output
- updating CLI output or README flow means editing the tape and rerunning the recipe

## Verification

Before considering the work complete, verify:

- `vhs` can render the tape successfully in the repo environment
- the generated GIF is committed and referenced correctly by `README.md`
- the demo runs in an isolated temp-home setup rather than using real user config
- the README still renders cleanly with the new GIF placement
- the quick-start copy and demo still match the actual CLI behavior closely enough to avoid misleading users

## Out of Scope

- adding a separate WebM asset in this pass
- creating a large demo documentation section
- expanding the demo beyond the `init -> create -> use -> list` flow
- changing product behavior to support the recording

## Success Criteria

- a new visitor sees a working terminal demo immediately under `## Quick Start`
- the demo reinforces the one-command-switch value without slowing the page down
- maintainers can update the demo by editing a checked-in `.tape` file and rerunning one `just` command
- the recording is isolated from the user's real OpenCode configuration
