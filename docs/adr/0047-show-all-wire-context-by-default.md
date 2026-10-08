---
type: ADR
id: "0047"
title: "Show all wire context by default and toggle it with Shift+Tab"
status: active
date: 2026-10-08
---

## Context

ADR 0042 collapses unrelated runs into counted context rows whenever the focus
wires exceed the terminal height, and `x` expands them. In use, the collapsed
default hid most Memories (the project's ADR records among them) behind a
`… N unrelated hidden` row, so browsing them took an extra key on every visit.
`x` also matched no other fold key: the tree and board fold everything with
`Shift+Tab`.

## Decision

**Focus wires open with all context shown, and `Shift+Tab` collapses or expands
unrelated runs. `x` has no meaning in the wires.**

The zero value of the view state is the expanded one, so every entry into the
wires, and every reload of the graph, starts expanded. Everything else in
ADR 0042 stands: collapsing keeps the selected record and its linked endpoints,
paging still reaches every endpoint, and the selected ID survives the toggle.

## Options considered

- **Expanded by default, `Shift+Tab` toggles** (chosen): every record is one
  scroll away, and the toggle uses the key that folds the tree and the board.
- **Keep the collapsed default, rebind to `Shift+Tab`**: still hides the
  records the user came to read.
- **Remember the last choice in the config**: more state for a one-key toggle.

## Consequences

On a tall graph the linked endpoints of the selection can be off screen until
the user scrolls or collapses. `Shift+Tab` is the way back to the compact view.
Re-evaluate if wires gain a per-project preference store.
