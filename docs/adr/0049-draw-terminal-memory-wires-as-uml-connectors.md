---
type: ADR
id: "0049"
title: "Draw terminal Memory wires as UML connectors"
status: superseded
superseded_by: "0050"
date: 2026-10-08
---

## Context
[ADR 0048](0048-draw-memory-links-as-uml-connectors.md) draws each Link kind
in the web graph as the UML connector with the same meaning, and asks to be
revisited when the terminal wires adopt a notation. The wires still used their
own glyphs: a heavy line for follows and depends on, a dashed `┄` for cites, a
dotted `·` for related, and a filled `▶` head on every kind. A reader who
learned the web legend had to learn a second one for the terminal.

## Decision
**The terminal wires and the Links list draw each kind as the same UML
connector as the web graph, in box-drawing glyphs.**

| Link kind  | UML connector        | Line | Head (out / back) |
|------------|----------------------|------|-------------------|
| follows    | realization          | `╌`  | `▷` / `◁`         |
| depends on | dependency           | `╌`  | `>` / `<`         |
| cites      | directed association | `─`  | `>` / `<`         |
| related    | association          | `─`  | none              |

A kind b9s does not recognise keeps a solid line with `>` / `<` in its stored
direction and claims no UML meaning, as in the web graph. One table in
`pkg/ui/memory_graph_view.go` feeds the lanes, the Issue-side tails, the Links
list and the legend, which names each UML connector.

## Options considered
- **UML glyphs** (chosen): one notation in both views. `▷` is the closest
  single-cell hollow triangle, and `>` reads as an open arrow.
- **Keep the terminal glyphs**: no change, but two legends for one meaning.
- **Colour only**: fails on monochrome terminals and for colour-blind readers.

## Consequences
Related Links lose their arrowhead in the terminal, as they did in the web
graph. Their direction is still stored and shown in the Links list label. A new
Link kind needs a row in `umlConnectors` and in the web `uml()` table, or it
falls back to the plain arrow in both views.
