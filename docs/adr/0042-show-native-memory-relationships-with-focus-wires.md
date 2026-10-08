---
type: ADR
id: "0042"
title: "Show native Memory relationships with focus wires and a stable web graph"
status: superseded
superseded_by: "0043"
date: 2026-10-03
supersedes: "0039, 0040"
---

## Context

Memory records have title and body, not ADR lifecycle fields. Interpreting body
frontmatter as status, supersession or hierarchy makes b9s display relationships
that Beads did not record. The user chose the existing two-column terminal wires
and the fixed-position web graph, and rejected matrix and chips modes.

Terminal wires also lost relevant endpoints when unrelated rows filled the screen.

## Decision

**Preserve native titles, bodies, Type identities and stored Link directions.
Use two-column focus wires in the terminal and stable card positions on the web.**

This supersedes ADR 0039's force layout, status colours, health filter and extra
terminal modes. It supersedes ADR 0040's inherited frontmatter interpretation.
ADR 0040's complete acquisition, ownership-based traversal and shared cache remain
the read contract, as confirmed by ADR 0041. ADR 0037's lazy body/version reads remain.

The pinned workspace has two Bead Types and four Link Types. Informational follows,
cites and related Links can connect Issues or Memories. The blocking Type
`preview-blocks-v1` connects Issues only: its source depends on its target.
Unknown Type names remain literal, without guessing their meaning from suffixes.
Related Links whose IDs contain child-of remain related. Bodies remain opaque prose.

The TUI keeps the existing Issue/Memory columns and wires. When space is limited,
unrelated runs collapse into counted context rows. Expanding context does not
change the selected ID. If relevant endpoints alone overflow, explicit paging
makes every endpoint reachable. The lower detail panel scrolls independently.
Same-kind relationships remain in the selected record's Link details.

Regular Issue detail shows incoming and outgoing Memory Links, including Links
on epics, with direction, native title and available notes. No verdict is inferred
from missing Links. The tree column shows a neutral Link count.

The web graph uses distinct Issue and Memory cards, full relationship labels,
line styles, selected-neighbour emphasis, pan and zoom. Selecting or filtering
does not move nodes. The graph may contain informational cycles, so it is not
presented as a DAG. Animation emphasizes selection or traces direction on request.

## Consequences

Memories with ADR prose remain readable with literal titles. ADR-only metadata
and derived health fields leave the shared model and browser API. Rendering
cannot rely on those fields. Matrix and chips keyboard modes are unavailable.
The CLI latency and snapshot limitations described in ADR 0041 still apply.

Regression tests cover literal content, exact Type labels, unchanged related
Links, incoming Memory Links, distant terminal endpoints and stable web positions.
