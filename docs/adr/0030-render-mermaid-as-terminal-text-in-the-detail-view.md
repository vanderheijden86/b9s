---
type: ADR
id: "0030"
title: "Render supported Mermaid blocks as terminal text in the detail view"
status: active
date: 2026-10-01
---

## Context

Issue descriptions, design, acceptance criteria, notes and comments can contain
Mermaid fences. Glamour displays these as source code in the terminal detail
view. b9s needs readable diagrams without a browser, image protocol or Node.
The copy action must still export the original Markdown.

## Decision

**Render `graph` and `flowchart` in TD/TB and LR directions, and
`sequenceDiagram`, with the pure-Go
[`mermaid-ascii`](https://github.com/AlexanderGrooff/mermaid-ascii) library.
Keep the original code fence for unsupported syntax, rendering failures and
output wider than the detail pane.**

The detail view selects supported fences before Glamour runs. It renders each
diagram through `pkg/render` with a fresh Unicode `diagram.DefaultConfig`,
then inserts the result as preformatted terminal text. It catches library
panics at this boundary, sanitizes terminal control characters and measures
the final display width with Lip Gloss. A flowchart may first use the library's
compact layout at the current pane width. Resize renders the source again.
`formatIssueMarkdown` continues to produce the unmodified source.

## Options considered

- **`mermaid-ascii` library** (chosen): Its public `RenderDiagram` API draws
  Unicode and ASCII flowcharts and sequence diagrams. The library is MIT
  licensed, has recent commits, returns errors for many parse failures, and
  supports compact flowchart layout through `MaxWidth`. It has no browser or
  Node runtime. Some malformed flowchart lines are accepted as node labels, so
  the integration checks the header and keeps the source on known failures.
- **Small renderer in b9s**: Avoids another dependency, but would require a
  parser, graph layout, arrow routing and Unicode cell width handling for both
  diagram types. That is substantial code for a view feature.
- **CLI subprocess or image renderer**: Adds runtime installation and process
  or terminal protocol requirements to an otherwise local text view.

## Consequences

- b9s vendors the Go library and only its imported packages. The selected
  revision is pinned in `go.mod`.
- Supported diagrams become easier to scan, but wide and unsupported diagrams
  still display as code. The hint beside a wide diagram explains why.
- Mermaid syntax is larger than this renderer's supported subset. If real
  issue diagrams regularly fall back, reassess the renderer or the subset.
