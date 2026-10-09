# README restructure: a front page and a reference

Date: 2026-10-08. Beads: bd-gmaz.1.

The README had grown to 525 lines of reference prose with a 17-entry table of contents. It
repeated most of `docs/USER_GUIDE.md` word for word and was the only home of a few things
(attachments, the flat list, the install details). This note records what was looked at, what
was decided and why, so the next person who wants to add a key table to the README can see the
reasoning before they do.

## What successful terminal-tool repositories do

Measured from the raw README of each repository on 2026-10-08.

| Repo | Lines | Where the reference lives | First install command |
|---|---|---|---|
| cli/cli | 122 | Manual site; per-OS install in `docs/install_*.md` | line 21 |
| zellij | 118 | zellij.dev | line 65 |
| atuin | 127 | docs.atuin.sh | line 41 |
| steveyegge/beads | 216 | `docs/` and beads.gascity.com; one 7-row command table inline | line 30 |
| ripgrep | 541 | `GUIDE.md` and `FAQ.md`; README holds comparisons | line 246 |
| lazygit | 663 | `docs/keybindings/`, `docs/Config.md` | line 282 |
| bat | 941 | inline | line 59 |
| fzf | 1153 | inline | line 116 |
| k9s | 1519 | inline, although k9scli.io exists | line 299 |
| beads_viewer | 4525 | inline, with duplicate install sections | line 40 |

What the well-regarded ones share:

- A one-sentence pitch, a screenshot or demo in the first 40 lines, then install. The first
  copyable command sits in the first 120 lines everywhere except k9s.
- "Who is this for" is one or two sentences, never a section. ripgrep pairs "why" with "why not"
  and names the alternatives. lazygit and bat end with an Alternatives section.
- A table of contents appears only past 500 lines. bat and zellij use a one-line anchor bar
  instead. ripgrep uses a short "Documentation quick links" list that doubles as the docs map.
- Key tables, configuration reference and per-OS install detail live in `docs/`. lazygit shows a
  TUI's keys without a table: one paragraph per feature, starting with the key to press.
- The Beads README itself is short, imperative and agent-first. Its vocabulary: `bd`, bead,
  Dolt, agent, task, ready. Hierarchy is written as `bd-a3f8` (epic), `bd-a3f8.1`, `bd-a3f8.1.1`.

k9s is the model for b9s's interaction, not for its README. Its 1519 lines are the pattern the
old b9s README was drifting toward.

## What the audience research says

From `osenco/projects/b9s-launch/` (plan, comms plan, community-tools and Pullboard research,
release notes) and the Reddit research. The audience is existing Beads users, above all
developers who run coding agents against a Beads project. The research says about them:

- They try things, and the measure is repeat use, not stars. The one action to ask for is "try
  b9s on one of your projects". The ten-minute acceptance sequence is: install and open your
  project, find an issue and understand its status, change it and see the change through `bd`.
- They distrust generated prose and marketing adjectives. The brand guide says "technical,
  direct, no fluff". No emoji, no em dashes, no "robust" or "seamless". Say what works and where
  it does not: phone access is a browser page, not a native app; Windows is unsupported; `bd`
  must be on `PATH`, with the tested range stated.
- A one-line promise is a pain plus a mechanism, and the contrast with other tools is "no new
  tracker, your Beads project as it is". b9s is a companion to `bd`, never a replacement: every
  write goes through `bd`.
- Credit the lineage: Beads to Steve Yegge, beads_viewer as the project b9s started from, k9s
  for the interaction model.
- The README opening, in this order: benefit, terminal, browser and phone screenshots, install,
  first useful action. The feedback thread goes under install, where a first-time user looks when
  something fails. A "one email per release" link belongs there too once the list exists; until
  then, point at GitHub's Watch, Custom, Releases.
- Lead with the phone view for readers on a phone, with the install command and the terminal for
  readers at a desk. The README is read both ways, so it shows both in the first screen.

## Decision

**README.md is a front page of about 250 lines. `docs/USER_GUIDE.md` is the one reference.**
Anything a user needs to look up twice (a key, a gesture, a config key, an environment variable,
a CLI flag) is stated once, in the guide, and the README links to it. The README states a key
only inside a sentence that teaches one thing, as lazygit does.

The README's sections, in order:

1. Pitch, demo link, video, the three screenshots.
2. A one-line anchor bar in place of a table of contents.
3. Who it is for, and what it is not.
4. Install, the `bd` requirement with the tested range, the feedback thread, release emails.
5. First five minutes: the acceptance sequence against your own project or the sample project.
6. What you can do: one short paragraph per capability, each starting with its key and linking
   to the guide section.
7. If you know k9s: the shared interaction model in five lines.
8. How it reads and writes: the source table, writes through `bd`, where the password goes.
9. Phone and browser: the Tailscale route in four lines, and what it is not.
10. Documentation map, development, contributing, acknowledgments, licence.

Moved out of the README and into the user guide, where missing: attachments (keys, config, CLI),
the flat list and title wrapping, the update key, Mermaid in the terminal detail pane, the detail
card layout, `B9S_WEB`, and the install details that belong with the `make` targets.

## Guard

`tests/docs/readme_test.go` fails when the README passes 300 lines or when a relative link in
the README or the user guide points at a file or heading that does not exist. The line cap
is the mechanism that keeps the decision; the link check is what makes moving content safe.

## Rejected

- **Keep one long README with a table of contents.** This is what k9s does. It is also what
  nine out of ten first-time readers scroll past: the acceptance sequence asks for a ten-minute
  trial, and a 525-line README puts the first command at line 57 and the first useful action
  after the k9s comparison. The guide already held the same text, so the README was not even
  the only copy.
- **A docs site.** zellij, atuin and cli do this. b9s has no site and the launch plan rules out
  a separate marketing site. `docs/` on GitHub renders fine and is one click from the README.
- **Several docs pages instead of one guide.** lazygit splits keys, config and custom commands
  into three files. The b9s guide is 815 lines with a table of contents and one heading per
  concern, which GitHub renders with its own outline. Splitting it would create more link
  targets to keep alive for little gain. Revisit when the guide passes about 1500 lines.
