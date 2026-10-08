---
type: ADR
id: "0043"
title: "Integrate Memory network and wires in the main TUI"
status: superseded
superseded_by: "0044"
date: 2026-10-03
supersedes: "0042"
---

## Context

The user wants to explore Memories inside regular b9s with the same status,
search and detail controls used elsewhere. A separate terminal program hides
that capability. Dense graphs must fit a terminal without losing records.
The user also rejected shaded pearl artwork in the web graph.

## Decision

**Embed the Memory browser in the main model. `M` opens it, `V` switches network
and wires, and `\` expands details. Draw plain circular markers in rounded cards.**

All native data semantics and acquisition decisions from ADR 0042 remain:
literal content, installed Type identities, stored directions, no ADR parsing,
and bounded CLI reads. The web layouts and fixed positions remain available.

The main model owns navigation. Memory commands carry a generation so replies
from a previous project cannot enter the current project. Returning to Issues
preserves both browsers' selection. `Ctrl+\` retains clear-marks behavior.

The network uses stable terminal slots and explicit pages. Selection highlights
immediate neighbours and incident Links. The detail pane shows every incident
relationship, even if an endpoint is on another page. Focus wires retain their
context collapse and endpoint paging.

`A`, `O` and `C` filter Issue lifecycle status. Memories have no such field and
remain visible. Literal search matches IDs, titles and bodies, and both layouts
use the same filtered graph. Expanded details scroll independently.

The main TUI reuses its loaded Memory graph. The standalone command remains for
scripted reads and direct list/version access. Ordinary workspaces without a
native Memory graph receive an explanation rather than invented Memory data.

## Consequences

The graph is reachable without another process. Terminal paging avoids an
unreadable fit-all reduction. Off-page relationships remain available in detail.
The graph snapshot is not a live subscription. No extra inferred statuses,
custom relationships or database writes are introduced.

Regression tests cover entry and return, key conflicts, stale project replies,
filter composition, resizing, paging and full-screen details. A real PTY check
starts normal b9s and exercises `M`, `V`, status, search and `\`.
