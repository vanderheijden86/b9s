---
type: ADR
id: "0065"
title: "Keep web board issue rows title-focused"
status: active
date: 2026-11-03
superseded_by: null
---

## Context

The web port of ADR 0030 retained borders and spacing around ordinary cards.
Selection added metadata, and ADR 0022 required a parent breadcrumb for nested
issues. These repeated lines make Open and In Progress difficult to scan.
Task bd-e5u3.20.14 asks for the terminal board's compact presentation while
preserving the useful rich epic cells. The latest feedback permits selection
to expand but asks that it avoid unnecessary clutter.

## Decision

Ordinary web board issues are borderless rows containing status, short ID,
title and P0 or P1 when applicable. Selection highlights the row and allows
the title to wrap. It adds no metadata or parent breadcrumb. Complete metadata
and parent relationships remain in issue details.

This refines the web port of ADR 0030. Its terminal presentation remains in
force. This supersedes ADR 0022's parent-line requirement. Its sticky lane
headers, epic index, navigation and folding remain in force.
Epic summary content, hierarchy ordering and indentation, and the lane height
and scrolling rules of ADRs 0053 and 0054 remain in force.
Phone and tablet rows retain the 44 px touch target from ADR 0052.

```mermaid
sequenceDiagram
    participant User
    participant Board
    participant Details
    User->>Board: Select an issue row
    Board-->>User: Highlight and wrap the title
    Board->>Details: Open the complete issue
    Details-->>User: Show metadata and parent relationships
```

## Options considered

- Keep boxed selected cards with metadata: repeats information available in
  details and changes the density when the cursor moves.
- Hide secondary fields with CSS: leaves duplicate content in the markup.
- Render title-focused rows and keep complete details (chosen): preserves
  navigation with a smaller ordinary row and an unchanged epic summary.

## Consequences

Desktop columns fit more issue titles. Selected long titles can take more than
one line. Parent context across columns requires opening details, while
same-cell indentation and epic summaries retain visible hierarchy context.
Urgency remains visible without repeating ordinary priorities.

## Advice

Check both cursor states, cross-column parents, keyboard navigation and details.
Check epic summaries separately from ordinary issue rows. Density must not
reduce phone or tablet touch targets below 44 px.
