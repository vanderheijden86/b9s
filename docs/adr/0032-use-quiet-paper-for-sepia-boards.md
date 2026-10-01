---
type: ADR
id: "0032"
title: "Use Quiet Paper for Sepia boards"
status: active
date: 2026-10-01
---

## Context

ADR 0018 defines rounded cards and boxed epic cells for the terminal board.
Those shapes help separate work on a dark background. On Sepia paper, the
repeated outlines compete with issue titles and make the board look dense.
Three Sepia Kanban mockups compared a flat page, index cards, and ruled
columns. The flat Quiet Paper variant was selected for the live board.

## Decision

**Sepia boards use flat issue rows, fine separators, a filled selected row,
and an epic rail with a single colored side. Dracula keeps the rounded cards
and boxed epic cells from ADR 0018.** The terminal and web boards share this
visual treatment. The board's grouping, navigation, folding, and issue facts
remain as defined in ADR 0018.

## Options considered

- **Quiet Paper** (chosen): issue titles read as rows on one sheet, with the
  selected row and epic side carrying the visual focus. Adjacent issues have
  less separation than boxed cards.
- **Index cards**: soft surfaces separate issues clearly, but repeat the card
  weight that made the Sepia board feel crowded.
- **Ruled journal**: compact rows show more work, but the darker paper and
  column rules add visual noise.

## Consequences

- The light and dark renderers use different board frames but retain the same
  issue content and layout rules.
- A selected item stays visible through its filled row and side mark.
- Re-evaluate if users cannot scan adjacent flat rows or lose the selected
  item in a dense epic lane.
