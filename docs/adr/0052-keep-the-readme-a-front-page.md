---
type: ADR
id: "0052"
title: "Keep the README a front page and the user guide the one reference"
status: active
date: 2026-10-09
---

## Context

The README had grown to 525 lines with a 17-entry table of contents. It repeated most of
`docs/USER_GUIDE.md` word for word, and a few things (attachments, the flat list, the install
details) lived only in the README. The first copyable command sat at line 57 and the first
useful action after the k9s comparison.

The audience is existing Beads users who run coding agents against a project. The launch
research in the osenco repository asks them for one thing, "try b9s on one of your projects",
and measures repeat use. Its acceptance sequence is ten minutes long: install and open the
project, find an issue and understand its status, change it and see the change through `bd`.
A README that a reader scrolls for a minute before the first command works against that.

A survey of ten terminal-tool repositories (recorded in
`docs/plans/2026-10-08-readme-restructure.md`) found the well-regarded ones at 120 to 700
lines, with a pitch and a visual in the first 40 lines, install in the first 120, a one-line
anchor bar in place of a table of contents, and key tables and configuration in `docs/`.
k9s is the model for b9s's interaction, not for its 1519-line README.

## Decision

**`README.md` is a front page of at most 300 lines, and `docs/USER_GUIDE.md` is the one
reference.** Anything a user looks up twice (a key, a gesture, a config key, an environment
variable, a CLI flag) is stated once, in the guide, and the README links to it. The README
names a key only inside a sentence that teaches one thing.

`tests/docs/readme_test.go` enforces it: the README fails the build past 300 lines, and
every relative link and `#anchor` in the README and the guide must resolve to a file or a
heading. Code that points a user at documentation names the guide, not the README.

## Options considered

- **A front page plus one guide** (chosen): one place per fact, the README short enough to
  read on a phone, a mechanical cap so the decision survives the next ten features.
  Cost: a reader who wants a key table clicks once more.
- **One long README with a table of contents**: what k9s and fzf do. The reference was
  already duplicated in the guide, so the README was not even the only copy, and the first
  command kept drifting down.
- **A documentation site**: what zellij, atuin and cli do. The launch plan rules out a
  separate site, and `docs/` on GitHub renders one click from the README.
- **Several guide pages**: lazygit splits keys, config and custom commands into three files.
  The guide is about 900 lines with its own contents and one heading per concern, so a split
  adds link targets to keep alive for little gain. Revisit past about 1500 lines.

## Consequences

- A new key or setting is documented in the guide, and the README gains at most one sentence
  in "What you can do" that links to it.
- Renaming a heading in the guide breaks the build until the README link follows. That is
  the point: the two files cannot drift apart silently.
- The README now carries claims the audience research asked for and the guide does not
  repeat: what b9s is not (no tracker of its own, a browser page rather than a native app,
  no Windows), the tested `bd` range, and the feedback thread under install.
- Re-evaluate when the README needs a section the cap cannot hold, or when the guide passes
  about 1500 lines.
