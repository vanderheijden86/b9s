---
type: ADR
id: "0018"
title: "Show epics as a rail or as rows, and hide the closed column"
status: active
date: 2026-09-25
---

## Context

ADR 0017 kept board layout A and offered three epic designs behind `v`:
lanes, chips and groups. In review, the lane idea won, but its first
version still read as a list of rows rather than a board. Five further
mockups (`docs/mockups/board-epic-lanes-options.html`) compared boxed
cards with the epic in a left rail, in a header row, and other variants.
The epic rail was the clear choice. Epic rows was worth keeping as an
alternative. Chips and groups were not used.

Two layout-A behaviours also got in the way. The focused column took most
of the width, so the other columns became unreadable, and the closed
column filled the board with finished work that is rarely the reason to
open it.

## Decision

**The board shows epics in two designs, the epic rail (default) and epic
rows, and `v` switches between them. Chips and groups are removed. The
closed column is hidden until `c` shows it, and every column with issues
gets an equal share of the width.**

- Both designs put each epic's issues in one horizontal lane that is
  aligned across the columns. Issues without an epic go in a last
  `No epic` lane.
- The rail design draws the epic (ID, title, completion bar, issue count)
  in a left column that stays in view while its lane scrolls. The rows
  design draws the same facts as a header row above the lane.
- The epic's own issue is the lane header and is not drawn as a card.
  Selecting it highlights the lane.
- `Tab` folds the selected epic and `Shift-Tab` folds or unfolds all, in
  both designs. A fold needs the epic's issue on the board, so a filter
  cannot hide a lane with no header to unfold it from.
- Cards are rounded boxes: title, an optional tag line, and a footer with
  type, ID, priority and age.
- `c` on the board toggles the closed column in the status grouping. It no
  longer applies the closed filter there. The bar counts the hidden
  closed issues and names the key.
- `ui.board_epics` accepts `rail` or `rows`. Any other value, including the
  old `lanes`, `chips` and `groups`, falls back to `rail`.

## Options considered

- **Rail by default, rows as the alternative** (chosen): the rail keeps
  the epic visible beside its cards, and rows fits narrow terminals where
  a rail costs too much width.
- **Keep all three 0017 designs**: more choice, but chips and groups had
  no users and tripled the rendering paths to test.
- **Rail only**: the least code, but narrow terminals lose a sixth of the
  width to the rail with no way out.
- **Keep the focus column**: gives one column room, at the cost of every
  other column's readability.

## Consequences

- One lane renderer serves both designs. The only difference is the rail
  width and the header row, so the two designs cannot drift apart.
- An existing config with `board_epics: lanes|chips|groups` still starts,
  on the rail.
- Anyone used to `c` as the closed filter on the board now sees the closed
  column toggle. `C` in the tree still filters closed issues.
- Re-evaluate if people turn the closed column on in most sessions, or if
  nobody uses epic rows.
