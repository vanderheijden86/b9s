---
type: ADR
id: "0016"
title: "Keep two board layouts until one is chosen"
status: superseded
date: 2026-09-24
superseded_by: "0017"
---

## Context

The board drew every status as an equal-width column of bordered cards. On a
typical project most columns are empty or hold closed history, so the width
went to nothing while the working column truncated titles. A card also mixed
four independent facts (stored status, dependency readiness, dispatcher lane
state and downstream impact) into one border colour.

`docs/board-redesign-spec.md` and the concept mockup compare six layouts. Two
survived review: A, adaptive focus, and E, focus plus a persistent inspector.
Choosing between them needs daily use on real projects, not another mockup.

## Decision

**Ship both A and E behind one board, switch them with `v`, and delete the old
card board.** `ui.board_layout` (`adaptive` or `inspector`) sets the layout at
start. Both layouts render the same `BoardModel` state (columns, swimlanes,
search, selection, empty-column toggle), so the layout is derived presentation
state and not a new state machine. When one layout is chosen, the other and
the `v` key are removed and `b` keeps opening the survivor.

Shared across both layouts:

- One region planner (`planBoardRegions`) decides which columns render in
  full and which fold into rails, from the terminal width, the column counts
  and the focus. Closed work in status mode is always a rail.
- One row contract (`boardCardView`) keeps the four facts apart:
  the column is the stored status, `blocked by X` names an open blocker,
  `lane: stage` comes from the `lane-stage=` label and `blocks N` from the
  reverse dependency index.
- Rows have no borders. Only the selected row carries a surface.

## Options considered

- **Both layouts behind `v`** (chosen): the reviewer compares them on live
  data in seconds. Cost: two row renderers and an inspector that may be
  deleted later.
- **Pick A now, as the spec recommends**: less code, but the choice rests on a
  static mockup.
- **Keep the old board beside the new ones**: no regression risk, but three
  layouts triple the rendering tests, and the old one has already been judged
  worse.
- **A layout per key (`b` and another letter)**: global keys are scarce
  (ARCHITECTURE.md, key routing), and a second board key would outlive the
  experiment.

## Consequences

- The Tab key on the board no longer expands a card. In layout E it moves focus
  to the inspector, where `j`/`k` scroll. In layout A it shows a hint.
- `l` on the board changes the column, as `h` already did. The global label
  picker on `l` no longer takes the key while the board has focus.
- The inline card expansion and the glamour detail panel are gone. `Enter`
  still opens the full detail view.
- Re-evaluate when one layout has been used for a week of lane work, or when a
  bug has to be fixed in both renderers.
