---
type: ADR
id: "0046"
title: "The Memory lever hides views but keeps graph workspace Issues"
status: active
date: 2026-10-07
---

## Context

ADR 0045 adds `B9S_MEMORY=off` (and `memory: off` in the config) to turn the
Memory views off, and skips the graph read whenever Memory is not ready. Taken
literally, the lever then skips the graph read too. A graph workspace refuses
`bd export`, so its Issues come only from the graph read, and with the lever
set such a project failed to open at all. The end-to-end test against a
preview-like bd found this.

## Decision

**The lever removes the Memory views only. The loader decides whether to read
the graph from the workspace and the bd probe alone, and with the lever set it
reads the graph for the Issues and then discards it.**

- `DetectMemory`, which `M`, the web Memory tab and `b9s memories` use, still
  answers `off` before any bd call.
- The loader uses the same detection without the lever. With the lever set, it
  does not keep the graph, so the MEMORY LINKS column, the Issue detail section
  and `/api/memory-graph` all see no graph.

## Options considered

- **Lever hides views, loader ignores it** (chosen): a graph workspace stays
  usable as an Issue tracker. The graph read still runs, so the lever saves no
  load time on a graph workspace.
- **Lever also skips the graph read**: rejected. It turns a switch meant to
  remove a feature into one that makes graph workspaces unopenable.
- **Read Issues some other way when the lever is set**: there is no other way.
  The graph CLI refuses export, and b9s never opens the store directly (ADR 0041).

## Consequences

The lever is safe to set globally. On a graph workspace it costs the same load
as before and hides only Memory. Re-evaluate if a graph workspace gains an
Issue-only read, which would let the lever skip the traversal as well.
