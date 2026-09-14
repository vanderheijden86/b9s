---
type: ADR
id: "0010"
title: "Take the tree sort from the user config, never from project state"
status: active
date: 2026-09-14
---

## Context

The tree sort used to be persisted per project in `.beads/tree-state.json`
(bd-2qw). Choosing a sort in the popup did not save it, but any later
expand/collapse did, capturing whatever sort was active at that moment. Every
data refresh rebuilds the tree and reloaded that file, so the persisted sort
replaced the one the user had just picked. With agents writing to Beads
continuously, refreshes arrive every few seconds and the sort kept snapping
back to a stale choice (Updated ascending, oldest first).

The file is also shared by every b9s session and worktree that opens the same
`.beads` directory, so one session's accidental sort became everyone's.

## Decision

**The tree sort at startup comes from `ui.sort` in `~/.config/b9s/config.yaml`
(default: created, descending). The `s` popup overrides it for the running
session only; the override survives tree rebuilds and is never persisted.**
`tree-state.json` keeps expand state and bookmarks only, and sort keys left in
older files are ignored.

## Options considered

- **User config default plus session override** (chosen): one predictable
  starting point everywhere; a deliberate choice in the session is respected
  until exit. A user who wants a different permanent default edits one file.
- **Keep per-project persistence, but save on every sort change**: fixes the
  flip-back, but a sort picked once for a quick look stays forever and leaks
  across sessions sharing the directory, which is the behaviour being removed.
- **Write the popup choice back to the user config**: makes every quick look a
  permanent change and turns a view action into a config write.

## Consequences

- Restarting b9s always shows the configured sort, in every project.
- Config names (`priority`, `created`, `updated`, `title`, `status`, `type`,
  `deps`, `pagerank`) live in `pkg/config` because `ui` imports `config`; a ui
  test fails if the two vocabularies drift.
- An invalid `ui.sort` value fails config loading, like an invalid refresh
  interval.
- Re-evaluate if users ask for per-project default sorts; that would belong in
  the user config keyed by project, not in shared project state.
