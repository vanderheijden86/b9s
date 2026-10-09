# One model

b9s is built on Bubble Tea, the Go framework from Charm that borrows its shape from Elm. A Bubble Tea program is one value, the model, and two functions on it. `Update` takes a message, usually a keypress, and returns a new model. `View` takes the model and returns a string, the whole screen. That is all. There are no widgets with their own state, no event bubbling, no callbacks.

The consequence for a program the size of b9s is that `pkg/ui` has one model and that model is large. `model.go` alone is over seven thousand lines. That number looks alarming until you see how it is organised: one file per concern, one method per key handler, and a strict order in which a key is offered to each.

## Where a key goes

Every key arrives in `Model.Update` and is offered to handlers in a fixed order. The first handler that takes it wins, and nothing below it sees it.

```
   key
    │
    ▼
   1. a modal overlay open?          tutorial, help, edit form, status
      └─ yes ──▶ it takes every key   picker, confirmation, project
    │                                 table, ':' prompt
    ▼
   2. the query field being edited?
      └─ yes ──▶ the key goes into the text, never into an action
    │
    ▼
   3. a global key?                   1-9 0, L, A, b, g, e, K, Delete,
      └─ yes ──▶ the global handler   Ctrl-N, Ctrl-E, ?, :, /
    │
    ▼
   4. the focused pane                tree, board, graph or detail
```

The order encodes three promises.

A modal takes everything. When the edit form is open, `j` types a `j` into the field, it does not move the tree. When a confirmation asks whether to close four issues, `q` does not quit the program. Bubble Tea's `huh` forms need to see every message type, not only keys, so the edit form is checked before the type switch on the message even begins.

The query field takes everything while you type. This is decision record 0002 in one sentence: a query letter never triggers an action. Press `/`, type `bug`, and neither `b` opens the board nor `u` does anything else. Press `Enter` to keep the filter and leave the field, or `Esc` to clear it.

Global keys beat pane keys. `b` opens the board from the tree, from the graph and from the detail. That consistency has a price, which the architecture document states plainly: the tree once had a bookmark on `b`, and it is now unreachable. Keys that fall into that trap are tracked as Beads issues rather than documented as features, and the user guide's key tables are the place to check before binding a new one.

## Views are overlays, the tree is home

The tree is always present. The board and the graph are overlays: `b` and `g` open them, `Esc` closes them and the tree is where you land. The detail pane sits beside the tree on a wide terminal, at least a hundred columns, and covers it on a narrow one.

Each view owns its own cursor and scroll state and nothing else. The board does not have its own filter. The graph does not have its own list of issues. They all read the one query result the root model produces, which is the subject of the next chapter, and they all reach for the same `IssueWriter` when they change something.

## What the files own

The split of `pkg/ui` by file is the map of the model:

| File | Owns |
|------|------|
| `model.go` | the root `Update`, the key routing above, the reload, the accessors tests use |
| `tree.go` | the tree: nesting, folding, marks, the cursor |
| `board.go`, `board_lanes.go`, `board_epics.go` | the kanban board and its epic designs |
| `graph.go` | the dependency graph |
| `query_state.go` | the parsed query |
| `commands.go` | the `:` prompt and its alias table |
| `edit_modal.go` | the create and edit forms |
| `issue_writer.go` | every call to `bd` |
| `project_*.go` | the header shortcuts, the project table, the switch state machine |
| `identities.go` | who the names in the issues are |
| `tutorial*.go` | the built-in tutorial |
| `background_worker.go` | work that must not block the update loop |

Nothing in the table is a Bubble Tea component with its own `Update`. They are all methods and helper types on the one model. That is less modular than a component tree and much easier to reason about: there is exactly one place where the state lives and exactly one order in which a key is routed.

## Off the loop

`Update` must return quickly. A Bubble Tea program that blocks in `Update` freezes the screen, because `View` cannot run until `Update` returns. Anything slow, a `bd` call, a database open, an identity load, runs as a `tea.Cmd`: a function that Bubble Tea executes on its own goroutine and whose result comes back later as a message.

That is why `IssueWriter` methods return `tea.Cmd` rather than an error, and why a project switch is a state machine rather than a function call. The model sends the work away, keeps drawing, and handles the answer when it arrives. Chapter eight shows the generation counter that keeps a late answer from a previous request from being mistaken for the current one.

`background_worker.go` handles the other kind of slow work, the kind that has nothing to do with a keypress: it records user activity so idle-time work can wait, and it runs the update check and similar chores when the user is not typing.

## Testing a model

A model with no components is a model you can test without a screen. Tests in `pkg/ui` build a `Model` from fixture issues, send it key messages, and read accessors such as `TreeSelectedID()` and `TreeNodeCount()`. No rendering, no terminal, no timing. When a test needs a new fact about the state, the rule is to add an accessor, not to parse the rendered string.
