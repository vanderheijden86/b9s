# Writing through bd

Press `K` on an issue and a confirmation appears: close this one? Press `y`. A status line says `bd close bd-abc…`. Half a second later the issue is gone from the tree and the next row is under the cursor.

Four things happened, in order, in four different places.

```
   User        ui.Model         IssueWriter        bd          store       watcher
    │ K            │                 │              │            │            │
    ├─────────────▶│ confirm         │              │            │            │
    │ y            │                 │              │            │            │
    ├─────────────▶│ CloseIssues ───▶│ exec ───────▶│ write ────▶│            │
    │              │                 │◀── exit 0 ───┤            │            │
    │              │◀── BdResultMsg ─┤              │            │ hash moved │
    │              │◀───────────── reload ──────────────────────────────────┤
    │◀─ redraw ────┤                 │              │            │            │
```

## IssueWriter

`IssueWriter` in `pkg/ui/issue_writer.go` is the only thing in b9s that runs `bd`. At startup it looks for `bd` on the `PATH`. If it is not there, every write answers with a clear "bd is not available" message and the program stays a viewer.

Its public methods map one-to-one onto `bd` subcommands: `UpdateIssue`, `CreateIssue`, `CloseIssue`, `DeleteIssue`, `DeferIssue`, `AddComment`, `SetStatus`, `SetPriority`, and their plural forms `CloseIssues`, `DeleteIssues`, `SetStatuses`. Each builds an argument list and returns a `tea.Cmd` that runs `exec.Command("bd", args...)` with the working directory set to the project's checkout. The result comes back as a `BdResultMsg` carrying the operation, the issue ID and either success or `bd`'s output.

There is no abstraction between the method and the command line. `CloseIssue(id, reason)` becomes `bd close <id> --reason=<reason>`. A reader who knows `bd` can read the writer, and a change in `bd`'s flags is a change in one small function.

## Bulk actions are one call

Mark five issues with `Space`, press `K`, and b9s runs `bd close a b c d e` once, not five times. That matters for speed, since one `bd` start is a few hundred milliseconds, and it matters for atomicity, since one call is one Dolt commit.

Decision record 0011 fixes the rule for which issues a bulk key acts on: the marked set when anything is marked, otherwise the cursor row. The confirmation lists the targets so you can see which rule applied. Confirming clears the marks whether or not `bd` succeeds, because a mark that survives a failed close would make the next `K` act on something you had already decided about.

## The checkout is a proof

A write needs a folder to run in, because `bd` reads `.beads/metadata.json` from its working directory to find the store. The `Checkout` type carries that folder, and it can only be built by `NewCheckout`, which refuses any folder without a `.beads` directory.

That constructor is a small piece of type-level design. A project opened from its database alone, through the `:project` table, has no checkout, so it has no `Checkout` value, so it cannot reach `IssueWriter`. The writer answers such a request with a read-only message. The same holds for the all-projects view: nine databases, no single folder, read-only. There is no boolean `readOnly` flag to forget to check. The absence of the value is the flag.

The writer also knows when a project is in the middle of opening. `SetOpening` marks that window, and a write during it is refused with a message rather than sent to a `bd` that might be pointed at the wrong folder. Chapter eight shows why the window exists.

## Failure shows bd's words

A `bd` call that exits non-zero returns its stderr in the `BdResultMsg`, and the UI shows that text in the status line. b9s does not translate it. `bd`'s own error message is the most accurate description of what went wrong, it is what the user would see on the command line, and keeping it verbatim means the two tools never disagree about why a change was refused.

The browser version keeps the same rule at a different layer: a failed `bd` answers HTTP 422 with `bd`'s output in the body, and the phone shows it.

## No optimistic update

Chapter four made the case. After `bd` returns, the model does not touch its own issue slice. It waits for the watcher. On Dolt that wait is at most the poll interval, 500 milliseconds. On JSONL it is the debounce window. In both cases the screen changes once, when the store has changed, and a write that `bd` accepted but the store somehow did not is a write the user can see did not land.

## What bd does that b9s never will

It is worth listing, because it is the payoff for the whole chapter. `bd` decides which fields are required, which status transitions are legal, how a close of an epic treats its children, how a comment is stamped with an actor, how a deferred issue is scheduled, and how a Dolt write becomes a commit with a message. Every one of those rules applies to a change made in b9s, today and after the next `bd` release, and b9s contains no code for any of them.
