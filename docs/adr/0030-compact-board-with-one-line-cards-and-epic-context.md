---
type: ADR
id: "0030"
title: "Compact the board: one-line cards and an epic cell that says what the epic is and where it stands"
status: proposed
date: 2026-10-02
---

## Context

ADR 0018 chose the epic rail: each epic in a boxed cell on the left, its
issues as boxed cards in status columns, one lane per epic. In daily use on
the b9s project the board spread out. A card is four or five lines, so seven
cards of one epic fill a 60-line terminal, and the next epic is below the
fold. The epic cell shows the ID, the title, a bar and `done/total`, so two
epics side by side differ only in their titles. A reader cannot tell what an
epic is about or where it stands without opening it.

The data is loaded already: an epic has a description, labels, an assignee
or owner, a due date and an update time, and its issues have statuses,
priorities and blockers. The board shows none of it in the rail.

The design was agreed in chat and is written up in
`docs/plans/2026-10-02-compact-board-design.md`.

## Decision

**A card is one line, and only the selected card expands to its box. The
epic cell keeps its box and gains the first sentence of the description,
its status counts, its urgent count, its labels, owner, due date and last
activity.** Rail or rows behind `v` and the hidden closed column stay as
ADR 0018 decided them.

- A card is `icon id title`, with `blocked <id>`, `lane: <stage>`,
  `blocks n` and `P0` or `P1` right-aligned. Age and P2 to P4 are not on
  it. The selected card is the rounded box with the tag line and the
  footer, and its lane grows to fit it.
- The epic cell has five content lines: ID and title; the description's
  first sentence; the bar with `done/total` and `P1 n`; `wip · waiting ·
  ready`; labels, owner, due date and last activity. Lines two and five are
  left out when they would be empty. A folded lane shows lines one and three.
- The counts are taken over the whole project, as `done/total` already is,
  so a filter never changes what an epic says about itself.
- The rail is a fifth of the width, between 16 and 34 cells.
- The epic rows header shows the same facts on two lines.

## Options considered

- **One-line cards, boxed epic cell with five lines** (chosen): a lane for
  seven issues takes seven lines, and the epic cell carries what makes the
  epic distinct. The box costs two lines per lane and keeps the look the
  rail had.
- **Epic cell without a box**: two lines fewer per lane. Declined; the box
  marks the epic as the lane's owner and the colour bar alone did not.
- **Boxed cards without a footer, empty columns folded**: three lines per
  card, and a board of twenty open issues still needs two screens.
- **An epic-first board that opens one epic's cards at a time**: the most
  compact, and it hides every card until one epic is selected, which the
  tree already does better.
- **Keep age and every priority on the card**: declined to keep the title
  wide. Both are on the selected card and in the detail pane.
- **A config switch for boxed cards**: declined. One card shape keeps one
  render path to test.

## Consequences

- A lane's height comes from its epic box in most lanes, and from the
  selected card's box in the lane that holds the selection. Moving the
  selection to another lane changes two lane heights, and the scroll keeps
  the selection in view as it does today.
- `boardEpic` grows by the description, labels, owner, due date and five
  counts, all computed in `rebuildEpicIndex` from the epic universe.
- The phone board and the wide web board keep their own cards and epic
  cells until a follow-up brings them the same facts.
- Re-evaluate if the epic box dominates lanes with one or two cards on
  short terminals, or if people miss the age on unselected cards.
