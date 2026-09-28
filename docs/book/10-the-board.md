# The board

Press `b` and the tree gives way to columns: open, in progress, blocked, and, when you ask for it, closed. Each issue is a card. `h` and `l` move between columns, `j` and `k` within one, and `Enter` opens the card. The board is the view most people picture when they hear "issue tracker", and it is also the view whose design took the most iterations to settle. Three decision records in a row, 0016, 0017 and 0018, tell the story, and the story is a good example of how b9s makes design choices.

## Two layouts, then one

The first board was a card grid. It worked and it was cramped: on a laptop terminal, four columns of cards at forty characters each left no room for a title. The redesign in `docs/board-redesign-spec.md` proposed several layouts, and two survived to code: layout A, an adaptive column board that folds columns you are not looking at into narrow rails, and layout E, a list with an inspector pane.

Decision record 0016 refused to pick. **Ship both behind one board, switch them with `v`, and delete the old card board.** Both rendered the same `BoardModel` state, columns, swimlanes, search, selection, so the layout was presentation and not a second state machine. The record said, in as many words, that when one layout was chosen the other would be removed.

A won the same day. Decision record 0017, dated like 0016, removed E, its inspector and the config key, and gave `v` a new job: cycling three ways of showing epics on the board.

## Epics on a board

A status board and an epic hierarchy pull in different directions. The columns say where each issue is. The epic says what it is part of. A board that shows only columns loses the second fact, and it is usually the fact that matters when you are asking "how far along is the mobile web work?"

Record 0017 tried three answers under `v`. Lanes cut every column into horizontal bands, one per epic, aligned across the columns so an epic reads as one row of the board. Chips kept the plain column order and tagged each card with a coloured bar and the epic's name. Groups sorted each column by epic under a subheader.

Record 0018 kept lanes and threw the rest away. **The board shows epics in two designs, the epic rail and epic rows, and `v` switches between them.** Both are lane designs. The rail, the default, draws each epic in a fixed left column, with its ID, title, a completion bar and an issue count, and its lane runs to the right of it across every status column:

```
   EPIC 3          OPEN 4              IN PROGRESS 2       BLOCKED 1
   ─────────────   ─────────────────   ─────────────────   ───────────────
   bd-foit         bd-foit.24          bd-foit.21          bd-foit.23
   Mobile web      Detail sheet        Wide board          Back button
   ▓▓▓▓▓░░░ 5/8    bd-foit.25
                   Sheet on Back
   ─────────────   ─────────────────   ─────────────────   ───────────────
   bd-ol0x         bd-ol0x
   The book        Write the book
   ░░░░░░░░ 0/1
   ─────────────   ─────────────────   ─────────────────   ───────────────
   No epic         bd-q1
                   Loose chore
```

The rows design draws the same facts as a header row above each lane instead of a cell beside it, which suits a narrow terminal. `Tab` folds the selected epic's lane to one line, `Shift-Tab` folds or unfolds all, and closed work stays hidden until `c` shows it, since a closed column full of cards was the main thing crowding the board.

## The rules under both designs

A few facts are computed once and shared by everything the board draws.

An issue's epic is its nearest ancestor of type epic along parent links. Completion counts every issue under the epic in the whole project, not only the ones on the board, so the bar does not change when you filter. Issues with no epic go in a last `No epic` lane. The epic's own issue is the lane header and is never drawn as a card, so selecting it highlights the lane rather than a card.

`planBoardRegions` decides, from the terminal width, the column counts and the focus, which columns render in full and which fold into rails. `boardCardView` is the row contract that keeps four facts apart on a card: the column is the stored status, `blocked by X` names an open blocker, `lane: stage` comes from a `lane-stage=` label, and `blocks N` comes from the reverse dependency index. Each is a different thing and a card that blurred them would lie.

## Folding

`z` folds the column under the cursor into a rail and `Z` unfolds every column. A rail is a narrow vertical strip with the column name and its count, and clicking or moving onto it unfolds it. Folding is what lets four or five columns fit on a laptop: the two you are working between are wide, the rest are a few characters each.

## Why three records rather than one

It would have been possible to write one record after the fact saying "the board has an epic rail". It would have lost the useful part. Record 0016 says why two layouts shipped together and what the exit condition was. Record 0017 says which one won and why chips and groups were tried. Record 0018 says why they lost. A future contributor who proposes epic chips again finds, in ten minutes of reading, that they were built, used and removed, and why. The board chapter of this book is short because those three documents already did the work.
