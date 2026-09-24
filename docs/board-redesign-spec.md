# Board TUI redesign exploration

Status: implemented as layouts A and E (ADR 0016); one layout still to be chosen

Beads epic: `bd-e5u3`

Mockup task: `bd-e5u3.1`

Interactive concepts: [board-redesign-concepts.html](mockups/board-redesign-concepts.html)

## Recommendation

Use **Concept A, Adaptive focus**, with the existing inspector behavior from Concept E.

The board should remain a status-oriented spatial overview, but it should stop assigning equal width and equal visual weight to every status. The focused column receives the most width, populated secondary columns remain useful, empty columns collapse to a narrow rail, and closed work defaults to a compact recent summary. Cards become compact rows with one selected surface instead of individually framed boxes.

This direction preserves the board's current mental model and keyboard contract while fixing the largest usability problem visible in the current screen: most of the terminal is spent on empty space, borders, repeated project names, and closed work rather than actionable work.

## Current problems

- Four equal columns reserve 25 percent of the terminal for an empty stored `BLOCKED` status and another 25 percent for 76 closed issues.
- Large bordered cards produce low information density. The gaps between cards are nearly as prominent as the data.
- The project name and repository-prefixed ID repeat on every card, then truncate before they help identify the issue.
- Border color carries readiness, dependency blocking, selection, and impact. Those meanings compete and cannot be understood from color alone.
- Stored status, computed dependency readiness, and dispatcher-owned `lane-stage` are different facts, but the board does not present that separation clearly.
- Long titles are clipped even on a very wide terminal because equal columns constrain every card to the same narrow width.
- Closed history visually competes with current work.
- The footer exposes many commands at once and weakens the information hierarchy.

## Concepts compared

| Concept | Information architecture | Strongest use | Main tradeoff | Relative effort |
|---|---|---|---|---|
| A. Adaptive focus | Status columns with focused width and collapsed rails | General daily use | Width changes with focus | Medium |
| B. Dense ledger | Grouped table with explicit metadata columns | Backlogs above 100 issues | Less spatial than Kanban | Low |
| C. Epic matrix | Epic rows crossed with workflow columns | Initiative and release planning | Ungrouped work needs special handling | High |
| D. Flow queues | Ready, in lane, waiting, delivered | Agent dispatch and unblock work | Uses computed workflow groupings, not stored status | Medium-high |
| E. Focus + inspector | Compact board plus persistent detail panel | Reviewing complex cards | Fewer cards fit horizontally | Low-medium |
| F. Narrow stack | Vertical collapsible status sections | Terminals below 80 columns | Cross-column comparison is sequential | Medium |

Concepts B through F should remain design references. They identify useful responsive and alternate-lens behavior, but should not become six separate production views.

## Information contract

The redesign must render four facts independently:

| Fact | Source | Presentation |
|---|---|---|
| Stored lifecycle status | `Issue.Status` | Primary status column or group |
| Dependency readiness | Open blocking dependencies | Explicit `blocked by ID` line, never a fabricated stored status |
| Dispatcher lane state | `lane-stage=VALUE` label | Dedicated `lane: value` field |
| Downstream impact | Reverse dependency index | `blocks N` field |

Recommended compact card anatomy:

```text
[eg0.4.2]  Wire the flow subscription transport             P1  2d
task       blocked by eg0.4.1              lane: blocked  blocks 0
```

Rules:

- Put the short issue path first and omit the project badge in single-project mode.
- Give the title all remaining width before secondary metadata.
- Use labels in addition to color for blocking, lane state, selection, and priority.
- Draw one selected surface. Unselected rows use separators, not full rectangular borders.
- Keep dependency blocker identity visible when present.
- Show at most the metadata that answers identity, urgency, readiness, ownership by lane, and impact.

## Responsive layout

| Terminal width | Layout |
|---|---|
| 160 columns and above | Four status regions. Focused column receives roughly 40 percent. Empty and closed regions may collapse to rails. |
| 110 to 159 columns | Focused column plus two useful secondary columns. Closed defaults to a recent-count rail. |
| 80 to 109 columns | Focused column plus one neighboring column. Other columns become labeled rails. |
| Below 80 columns | Concept F stacked sections. One expanded section at a time. |

An empty column remains discoverable by its rail and count, but it does not consume a full quarter of the screen. The selected issue remains selected when a breakpoint or column width changes.

## Interaction contract

Keep the current interaction model unless usability testing disproves it:

- `h` and `l` move between status regions.
- `j` and `k` move between issues.
- `1` through `4` jump to a status region.
- `gg`, `G`, page keys, and mouse wheel retain their existing navigation behavior.
- `Enter` opens issue detail; `Tab` moves focus to or hides the inspector.
- `s` cycles the existing status, priority, and type grouping modes.
- `/` edits the root-owned shared query. Filtering stays identical across list, tree, and board.
- Search remains explainable from visible identity data, consistent with ADR 0004: plain search uses IDs, titles, and labels.
- Mouse wheel behavior remains consistent with ADR 0003: it moves the focused issue list or focused detail viewport.

The first implementation should not add a new finite-state model. Adaptive width is derived presentation state from terminal width, populated columns, current focus, detail visibility, and grouping mode.

## Implementation map

The current `BoardModel` already owns the necessary selection, grouping, empty-column, search, expansion, and detail state. The redesign can stay inside the existing board boundary:

- `BoardModel.View`: replace equal-width allocation with a pure responsive layout policy.
- `renderCard`: split data selection from rendering, then render compact rows by default.
- `renderExpandedCard`: keep as an optional inline detail treatment, but do not make every card tall.
- `renderDetailPanel`: reuse for Concept E and preserve its current focus and scrolling behavior.
- `computeColumnStats`: retain counts, but present only metrics that affect a board decision.
- `blocksIndex` and `issueMap`: reuse for `blocked by` and `blocks N` fields.
- Issue labels: parse `lane-stage` as display-only dispatcher state. Do not mirror it into `Issue.Status`.

Suggested internal presentation types for implementation planning:

```go
type BoardLayout struct {
    Regions []BoardRegion
    Stacked bool
}

type BoardRegion struct {
    ColumnIndex int
    Width       int
    Collapsed   bool
}

type BoardCardView struct {
    ShortID      string
    Title        string
    Priority     string
    Age          string
    BlockedBy    string
    LaneStage    string
    BlocksCount  int
}
```

These are presentation contracts, not additional persisted state.

## Validation cases

Implementation should be accepted only when the same behavior is demonstrated for:

- The sparse screenshot case: 29 open, 1 in progress, 0 stored blocked, 76 closed.
- A dense board with at least 200 issues and every status populated.
- A board with open issues blocked by open dependencies while stored status remains `OPEN`.
- Lane stages `READY`, `IMPLEMENTING`, `REVIEWING`, `BLOCKED`, `ABANDONED`, and `DELIVERED`.
- Long titles, deep issue paths, cross-project IDs, and missing timestamps.
- Widths of 60, 80, 110, 160, and 220 columns.
- Search filtering, grouping mode changes, detail visibility changes, and snapshot refresh without losing selection.
- Keyboard and mouse navigation with bounded E2E tests and no browser launch.

Non-goals for the first implementation: drag and drop, editable card fields, a new stored status, a replacement query system, or a graph TUI redesign.
