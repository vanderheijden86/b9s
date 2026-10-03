---
type: ADR
id: "0032"
title: "Group board cards with nested feature brackets"
status: active
date: 2026-10-03
---

## Context

The one-line cards chosen in ADR 0030 make large closed columns dense. Epic
lanes give the outer context, but feature members mix with unrelated cards
when the column is sorted only by priority and date.

## Decision

**Keep the epic lanes and status columns. Within each column, place feature
members together inside muted square brackets, with nested brackets for
parents inside the feature.** This extends ADR 0030 without changing its
card shapes or epic presentation.

- Derive membership from parent-child dependencies in the full project
  universe. Do not infer relationships from ID prefixes, labels or blockers.
- Name each bracket with its parent's ID and title. A filtered parent can
  supply context without becoming an extra card or changing the count.
- Keep the parent issue selectable in its own status column. The bracket
  heading is context, not another selectable row.
- Order sibling feature groups by the first member in the existing priority
  and date order. Within a group, show the parent first, then direct members,
  then nested groups. Place ungrouped cards after feature groups.
- The same order drives navigation, search, selection and rendering.
- Use the theme's border color. Keep the selected card's existing highlight.
- Limit ancestry traversal to 32 nodes and stop on repeated IDs. Limit
  indentation by available width, keeping at least 12 cells for a card.
  Deeper descendants remain visible in their enclosing bracket.
- Feature-only projects use the brackets without adding an empty epic rail.

## Options considered

- **Nested brackets** (chosen): shows membership beside the cards while
  retaining the status columns and compact card shape.
- **Feature swimlanes**: makes each feature a full-width lane, competing
  with the existing epic lanes and increasing empty space.
- **Feature badges on every card**: preserves the flat sort but repeats text
  and makes long closed columns harder to scan.

## Consequences

Each visible group adds two boundary lines. Selection scrolling includes
these lines and the expanded card's actual width and height. Narrow columns
show fewer nesting levels. Features in several statuses have a bracket in
each applicable column. The browser board retains its separate presentation.

Re-evaluate if the boundaries cost too much height for many tiny groups or
if users need to collapse individual features independently of their epic.
