---
type: ADR
id: "0032"
title: "Rank tree parents by the newest date in their subtree"
status: active
date: 2026-10-04
---

## Context

The tree sorts siblings within their parent. Under the Created and Updated
sorts each node ranked by its own date only. A subtask created or edited
minutes ago under a 13-hour-old epic therefore stayed out of sight: the epic
kept its own old date and sat below newer roots, so the epic being worked on
was hard to find.

## Decision

**Under the Created and Updated sorts, a tree node ranks by the newest such
date anywhere in its subtree, at every level. The date column still shows each
issue's own date.**

Other sorts are unchanged. Children of an epic keep their natural title order
under Created (bd-9gbb). The flat list has no hierarchy, so each row ranks by
its own date there.

## Options considered

- **Subtree-newest date for Created and Updated, own date in the column**
  (chosen): active epics rise with their work; the column stays a true
  property of the row.
- **Updated only**: smaller change, but a new subtask would not lift its epic
  under the default Created sort, which is the case that prompted this.
- **Also show the rolled-up date in the column**: the order matches the column,
  but the column then misstates when the epic itself was created or changed.

## Consequences

- An old epic with fresh activity can sit above a newer epic whose own date is
  later, while its date column shows an older value. The expanded child row
  carries the date that explains the position.
- Sorting walks each sibling's subtree once per level, O(n × depth), which is
  small next to rendering.
- Revisit if users want the rolled-up date visible, for example as a separate
  column.
