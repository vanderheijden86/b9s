---
type: ADR
id: "0044"
title: "Keep terminal Memory wires and detect capabilities"
status: active
date: 2026-10-03
supersedes: "0043"
---

## Context

The user rejected the terminal network layout. They want the two-column wires
inside regular b9s, with Memory availability determined by the current project
and its Beads installation. Release names are not capability contracts.

## Decision

**Keep wires as the only terminal relationship layout. Detect support through
ready workspace metadata and actual CLI reads, without branch or version gates.**

`M` opens wires. The terminal network renderer, cursor and view-switch keys are
removed. Native semantics, status filters, literal search, expanded details,
context collapse, endpoint paging and integration with regular b9s remain.
Web graph layouts remain available. The Memory list supports body/version access.

Use the `bd` resolved on the application's PATH. A compatible future release
works without an allowlist change. The current adapter requires the current
Memory graph metadata and protocol. Unknown future protocols are not guessed.

Explain known failures from typed causes first and recognized CLI diagnostics
second: missing binary, Memories not enabled, unavailable commands, denied access,
unreachable database, timeout, schema mismatch and unsupported response. Preserve
the original diagnostic for unknown failures. Never migrate or enable a workspace
as a side effect of viewing it. Cached graphs cannot conceal failed CLI reads.

## Consequences

The TUI has one relationship layout and keeps its established controls.
Unsupported projects receive an explanation. Retrying a failed integrated read
creates a fresh browser generation, discarding replies from the previous attempt.
Protocol changes still require adapter changes, even if a release supports Memories.
