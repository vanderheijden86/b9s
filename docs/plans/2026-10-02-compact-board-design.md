# Compact Board: One-Line Cards and a Context-Rich Epic Cell

Beads task: bd-e5u3.20 (epic bd-e5u3).

## Contents

- [Problem](#problem)
- [Decisions](#decisions)
- [The lane](#the-lane)
- [The epic cell](#the-epic-cell)
- [The card](#the-card)
- [Epic rows](#epic-rows)
- [Counting](#counting)
- [Unchanged](#unchanged)
- [Testing](#testing)
- [Documentation and ADR](#documentation-and-adr)
- [Out of scope](#out-of-scope)

## Problem

A screenshot of the board on the b9s project, 200 columns wide, on 2026-10-02:

| What the screen shows | Why it is a problem |
|---|---|
| Seven cards of one epic fill the whole height | Each card is a box of four or five lines: edges, title, tag line, footer |
| The open column is 55 cells wide, the closed column too, and in progress is blank | Every column with issues gets an equal share, whatever a lane puts in it |
| The epic cell says `vd9g.1`, `Epic: BD Events`, a bar, `0/9`, `9 issues` | Two epics side by side differ only in the title. Nothing says what the epic is about or where it stands |

The data to tell epics apart is already loaded. An issue carries a description, labels, an assignee and owner, a due date and an update time. The board reads all of them for the detail pane and shows none of them in the rail.

## Decisions

Agreed in chat on 2026-10-02, one question at a time:

1. The epic cell tells **what the epic is about** (description, labels, owner) and **where it stands** (status counts, P0/P1, last activity, due date). "What comes next" and "its structure" were offered and not chosen.
2. Cards are **one line**. The selected card expands to the current box.
3. The epic cell **keeps its box**. A box without edges was offered to save two lines per lane and was declined.
4. Age is not on the one-line card. P2 to P4 are not on it either.
5. No new key and no config value. The boxed card is not kept behind a switch.

## The lane

A lane is as tall as the taller of its epic box and its longest column, plus the rule under it. With one-line cards the epic box sets the height of most lanes, so the box is the budget: five content lines at most, seven lines boxed.

```
 EPIC  93                         OPEN 87 · P1 14                       IN PROGRESS 10
 ─────────────────────────────────────────────────────────────────────────────────────────
╭────────────────────────────────╮
┃ ▾ ◆ vd9g.1 BD Events           │ ● 1.8  Integration and E2E tests…  blocked 1.6 · P1
┃ Live activity feed from bd     │ ▲ 1.6  Add the :activity view to…  blocked 1.13 · P1
┃ events, in the TUI and web.    │ ● 1.10 Reply on GH #11 with find…  blocked yv9a
┃ ━━━━━━━━──────── 7/16 · P1 2   │ ● 1.9  ADR 0027 and docs for the…  blocked 1.6
┃ wip 1 · waiting 6 · ready 1    │ ● 1.7  Diagnostics overlay: activ…  blocked 1.13
┃ #tui #web · @andre · 4d        │ ● 1.5  Web store: GET /api/activi…  blocked 1.13
╰────────────────────────────────╯ ● 1.12 Community-tools PR to Bead…  blocked vd9g.2
 ─────────────────────────────────────────────────────────────────────────────────────────
╭────────────────────────────────╮
┃ ▾ ◆ bjqc k9s-style marking     │ ▲ 4    Bulk priority and label e…                 ● 5  Live refresh moves the…  P1
┃ and bulk actions in tree view  │                                                   ╭──────────────────────────────╮
┃ Mark issues with Space and     │                                                   │ Bulk close and delete over   │
┃ act on all of them at once.    │                                                   │ marked issues                │
┃ ━━━━━━━━━━━━━━── 5/6           │                                                   │ blocks 1                     │
┃ wip 0 · waiting 1 · ready 1    │                                                   │ ▲ 2             P2 · 2w      │
╰────────────────────────────────╯                                                   ╰──────────────────────────────╯
```

The rail is a fifth of the width, between 16 and 34 cells, instead of a sixth capped at 26. The description needs the room, and one-line cards give it back.

## The epic cell

Content lines, top to bottom. Each line is cut with `…` at the inner width.

| Line | Content | When |
|---|---|---|
| 1 | `▾ ◆ id` in the epic's colour, then the title, bold, wrapped to at most two lines | Always |
| 2 | The first sentence of the description, muted, wrapped to at most two lines | The description is not empty |
| 3 | The progress bar, ` done/total`, and ` · P1 n` where n counts open P0 and P1 issues | Always |
| 4 | `wip a · waiting b · ready c` | Always |
| 5 | `#label` for each label, `@owner`, `due 14 Oct`, and the last activity as `4d`, joined by ` · ` | Any of them exists. Activity always exists, so the line is there when the epic has a label, an owner or a due date, and the activity moves to line 4's end otherwise |

Line 4 ends with ` · 4d` when line 5 is absent, so the last activity is always on screen.

The first sentence ends at the first `.`, `!` or `?` followed by a space or the end, or at the first blank line, whichever comes first. Markdown headings and list markers at the start of the description are dropped before the cut.

`@owner` is the assignee, or the owner when there is no assignee, shown through the identity registry the detail pane uses (ADR 0014), so a known actor shows as the configured name.

A due date is red when it is in the past. `due` is left out when there is none.

A folded lane shows line 1 with the `▸` marker and the title on one line, then line 3. The hidden counts per column stay as they are.

The `No epic` lane keeps its one-line cell and its issue count.

The box is drawn by `railBox` as today: the epic's colour on the left side, the border colour on the other edges, the selected epic on the highlight fill with primary edges. A lane shorter than three lines still draws the content behind the coloured side without a box.

## The card

One line, in the column's width:

```
● 1.8  Integration and E2E tests for the activity view  blocked 1.6 · P1
```

| Part | Content |
|---|---|
| Icon | The type icon in its colour, as the footer draws it today |
| ID | The short ID, bold, in the primary colour. A search match draws it in the match colour |
| Title | The title without its own `[path] ` prefix, cut with `…` to the cells the tags leave |
| Tags | Right-aligned, joined by ` · `, in this order: `blocked <id>` in the blocked colour, `lane: <stage>` in the in-progress colour, `blocks n` in the feature colour, `P0` or `P1` in bold red |

Tags are dropped from the right when the title would get fewer than eight cells, the priority last, as `cardTags` drops them today. The ID is never dropped.

The selected card is the box `cardLines` draws today: title on up to two lines, the tag line, the footer with icon, ID, priority and age. The lane grows to the box, and the other columns in the lane stay aligned with it, because lane height is computed from `cardHeight` and `cardHeight` now takes the selection into account. The selection surface fills the box as today.

The current search match that is not selected keeps the one-line shape, on the match background `rowSurface` gives it.

The plain board, without epics, draws the same one-line cards under its column headers. The `n/m · k more` footer stays.

## Epic rows

The rows design draws the same five lines of facts in its header box, on two lines:

```
╭───────────────────────────────────────────────────────────────────────────────────────╮
┃ ▾ ◆ vd9g.1 BD Events  wip 1 · waiting 6 · ready 1 · #tui #web · @andre · 4d  ━━━━━━──── 7/16 · P1 2 │
┃ Live activity feed from bd events, in the TUI and web.                                         │
╰───────────────────────────────────────────────────────────────────────────────────────╯
```

Line one is cut from the right: the counts go before the labels, owner and activity, and the bar is dropped before any of them when the width is under 60 cells, as `laneRow` drops it today. Line two is absent when the description is empty. A folded row is line one alone.

## Counting

`boardEpic` grows by the fields the cell shows. All are computed in `rebuildEpicIndex` over the epic universe, the whole project, so a filter never changes them, as `Done` and `Total` are today.

| Field | Definition |
|---|---|
| `Description` | The epic issue's description, sanitised, first sentence only |
| `Labels`, `Owner`, `Due` | The epic issue's own fields |
| `Urgent` | Open issues under the epic with priority 0 or 1, the epic itself excluded |
| `InProgress` | Issues under the epic with status `in_progress` |
| `Waiting` | Open issues under the epic with an open blocker, by `openBlocker`, whatever their stored status. This matches the column header's `waiting` |
| `Ready` | Open issues under the epic with no open blocker, not deferred, not the epic itself |
| `LastActivity` | The latest `UpdatedAt` over the epic and every issue under it |

"Open" here is any status that is not closed-like, as `isClosedLikeStatus` decides. An issue whose stored status is `blocked` and that has no open blocker counts as neither waiting nor ready; it is in the open column with no claim about readiness.

## Unchanged

- The rail and rows designs behind `v`, and `ui.board_epics`.
- `Tab` and `Shift-Tab` on epics, `z` and `Z` on columns, `{` and `}`, `f`, `c`, `s`, every move key and the epic column.
- Column headers, the board bar, the key hints and the `n/m · k more` footer.
- Lane order, the `No epic` lane, the epic palette.
- The phone board and the wide web board. Both have their own renderers in `web/src/render.ts`.

## Testing

Unit tests in `pkg/ui`, written first:

- `cardHeight` is 1 for an unselected card at any width, and the box height for the selected one.
- `cardLines` on one line: icon, ID, cut title, tags in order, tags dropped from the right, the ID kept at eight cells.
- `railLines`: every line with a full epic; no description line for an empty description; no fifth line without labels, owner and due date, and the activity then on line 4; the first sentence cut at `.`, at a blank line, and with a heading dropped; a past due date in red; a folded cell with two lines.
- `rebuildEpicIndex` counts: urgent, in progress, waiting, ready and last activity on a fixture with a blocker, a deferred issue and a closed child.
- `lanesBody`: a lane's height is the epic box with few cards and the column with many; the selected card's box grows its lane and the other columns stay aligned; the rows header has two lines with a description and one without.
- The board layout and epic tests that assert the box shape change to the one-line shape.

Then `go test ./pkg/ui/`, `go test ./... -skip DoltIntegration -p 2`, and the E2E board tests in `tests/e2e/` once on the result, with their bounded timeouts.

## Documentation and ADR

- ADR 0030 supersedes ADR 0018. It restates rail or rows behind `v` and the hidden closed column, and records the one-line card and the epic cell's five lines. ADR 0018 gets `status: superseded` and `superseded_by: "0030"` when 0030 goes active.
- README: the board paragraph describes the one-line card, the expanded selection and what the epic cell shows.
- `docs/ARCHITECTURE.md` names the new `boardEpic` fields where it describes the board.

## Out of scope

- The wide web board and the phone board. A follow-up ticket brings the epic cell's facts and one-line cards to `web/src/render.ts`.
- A config switch for boxed cards. Declined; the design doc records the choice.
- "What comes next" (the next ready issue in the cell) and "its structure" (feature children in the cell). Offered and not chosen; either can come as a later line if daily use asks for it.
