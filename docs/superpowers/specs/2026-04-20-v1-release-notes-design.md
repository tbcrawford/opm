# v1.0.0 Release Notes Design

**Date:** 2026-04-20

## Goal

Write polished, copy-ready `v1.0.0` release notes in Markdown that summarize what `opm` is, preserve the spirit of the initial `v0.1.0` release, and explain the major user-facing improvements made on `main` since `v0.1.0`.

## Output

Create a single Markdown file at `RELEASE-v1.0.0.md`.

The file should be ready to paste into GitHub Releases with minimal or no editing.

## Audience

The release notes are for:

- new users landing on the `v1.0.0` release page who may not know what `opm` is yet
- existing users who tried `v0.1.0` and want to understand what matured between the initial release and `v1.0.0`

The writing should be polished and user-facing rather than internal or commit-by-commit.

## Scope

The notes should cover changes on `main` from `v0.1.0..HEAD`.

They should not cover side-branch-only work that has not landed on `main`.

## Structure

The file should use this overall shape:

1. release title and short summary
2. a concise “what opm is” introduction
3. highlights since `v0.1.0`
4. reliability and platform improvements
5. UX and documentation improvements
6. install or upgrade note

This should read like a polished GitHub release body, not a changelog appendix.

## Source Material

The release notes should be grounded in:

- the existing `v0.1.0` GitHub release body for the initial project framing
- commits on `main` from `v0.1.0..HEAD`
- current repository docs and README positioning where needed to confirm wording

The initial-release framing should be preserved in spirit, but rewritten more concisely and confidently for a `v1.0.0` audience.

## Editorial Rules

The notes should:

- emphasize user-visible outcomes over internal implementation details
- group changes into a few meaningful themes instead of listing every commit
- stay accurate to the shipped behavior on `main`
- sound polished and credible rather than hype-heavy
- be concise enough to work as a GitHub release body

The notes should not:

- read like a raw commit dump
- overemphasize internal refactors unless they clearly improved reliability or usability
- mention branch-only or unreleased work
- invent upgrade steps that are not actually needed

## Themes To Emphasize

The strongest themes to highlight are:

- `opm` has matured from an initial CLI release into a more production-ready profile manager
- profile operations and state handling have been hardened around edge cases and consistency
- Windows support and symlink handling improved substantially
- CLI help, completion, and key workflows were refined
- the onboarding and README experience improved, including the quick-start demo now present on `main`

The notes should de-emphasize:

- routine dependency bumps
- purely internal refactors without a clear user-facing payoff

## Success Criteria

- a reader can understand what `opm` is even if they did not read the `v0.1.0` release
- a returning user can quickly see why `v1.0.0` is meaningfully more mature than `v0.1.0`
- the final Markdown is polished enough to paste directly into GitHub Releases
- the release notes clearly reflect `main` rather than in-progress branch work
