---
type: ADR
id: "0048"
title: "Draw Memory Links as UML connectors"
status: active
date: 2026-10-08
---

## Context
The web Memory graph told its four Link kinds apart by colour and by an
invented choice of solid, dashed and dotted lines. At 1.25 px the colours were
hard to read on a large graph, and the line styles carried no meaning a reader
already knew: dotted for related and dashed for cites had to be learned from
the legend.

## Decision
**Each Link kind is drawn as the UML connector with the same meaning, and the
reader sets the line thickness (1 to 6 px, default 2 px) with − and + buttons.**

| Link kind  | UML connector        | Line   | Head            |
|------------|----------------------|--------|-----------------|
| follows    | realization          | dashed | hollow triangle |
| depends on | dependency           | dashed | open arrow      |
| cites      | directed association | solid  | open arrow      |
| related    | association          | solid  | none            |

Work that follows a decision realizes it, as a class realizes an interface.
`blocks` is a dependency of the source on the target. A citation navigates
one way. Related has no stored direction worth showing, so it has no head.
A kind b9s does not recognise keeps a solid line with an open arrow in its
stored direction, and claims no UML meaning. Colour stays as a second cue.

## Options considered
- **UML connectors** (chosen): a notation many readers know, and the line
  reads without its colour. Two kinds share the dashed line and differ only by
  the head, which a thicker line makes visible.
- **Invented dash patterns**: what the graph had; meaning lives only in the legend.
- **Colour only, thicker lines**: fails for colour-blind readers and in print.

## Consequences
The legend draws a sample of each connector beside its kind and UML name.
Arrowheads use stroke-width marker units, so they grow with the chosen
thickness. A new Link kind needs a row in `uml()` in `web/src/memory.ts`;
without one it falls back to the plain arrow. Revisit if the TUI wires view
adopts a notation, so both views agree.
