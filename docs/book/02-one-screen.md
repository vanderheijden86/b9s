# One screen, two paths

Start b9s in a folder that has a `.beads` directory and you get one screen. On the left, a tree: epics at the top, their features and tasks nested below, a cyan bar on the row your cursor is on. On the right, the detail of that row rendered as Markdown. Across the top, a header with the logo, the digits `1` to `9` for your recent projects, and a title that names the project and where its data comes from. Along the bottom, one line of key hints.

That is the whole program, from the outside. From the inside, the interesting thing is not what is on the screen. It is that everything on the screen arrives by one path, and everything you change leaves by a different one.

```
                    reads (direct, fast, live)
     ┌──────────────────────────────────────────────┐
     │                                              ▼
  Dolt server / SQLite / JSONL               ┌────────────┐
     ▲                                       │  b9s       │
     │                                       │  ui.Model  │
     │            bd CLI ◀───── exec ─────── └────────────┘
     └────────────┘
                    writes (through bd, always)
```

b9s reads its store directly. It opens a MySQL connection to a Dolt server, or a SQLite file, or a JSONL file, and loads every issue into memory. Reads are cheap, so it does them often: every keystroke in the search field filters the loaded set without asking the store anything.

b9s never writes to the store. When you close an issue, b9s runs `bd close <id>` as a child process and waits. When `bd` returns, the store has changed, the watcher notices, and the reload arrives through the same read path as everything else. The UI does not patch its own copy of the issue. It waits to be told.

This split is the first design decision in the repository and the one that everything else leans on. It is worth spelling out why.

## Why not write directly

Writing directly would be faster to implement and faster to run. It would also mean b9s carries a second copy of every rule that `bd` enforces: which fields an issue must have, what a valid status transition is, how a close cascades to children, how a comment is timestamped, how the Dolt working set is committed. Those rules change with every `bd` release. A viewer that mirrors them is a viewer that drifts.

Running `bd` means b9s and `bd` can never disagree about what is stored. If `bd` refuses a change, b9s shows `bd`'s own error text. If `bd` gains a new validation, b9s gains it the same day, without a line of Go.

The cost is latency. A `bd` call takes tens to hundreds of milliseconds, and a Dolt-backed one can take longer. b9s pays that cost once per write and hides it: the command runs off the update loop, a status line says what is happening, and the reload arrives when it arrives.

## Why not read through bd

The other half of the split is less obvious. `bd list --json` exists. Why open a database connection at all?

Because a viewer that polls `bd list` every half second is a viewer that spawns two processes a second, each of which opens its own database connection, parses its own configuration and exits. That is fine for a script and hostile for a program that stays open all day on a laptop. Reading directly costs one connection and one query per change, and the change detection is a single `SELECT DOLT_HASHOF_DB()`.

Reading directly also means b9s can show projects that `bd` cannot: a database on the shared server that has no checkout on this machine, or nine projects at once in the all-projects view. Those are read-only, for reasons the write path explains, but they are readable.

## The k9s lineage

The interaction model comes from k9s, and the design document that started the rewrite (`docs/k9s-inspired-design.md`, February 2026) says why in one sentence: complex data becomes manageable when you combine instant context switching, command-driven navigation and progressive disclosure.

In k9s the context is a namespace. In b9s it is a project, a folder with a `.beads` directory or a database on the Dolt server. The digits switch between recent ones, `0` shows all of them at once, and `:project` opens a table of everything the server will let you read.

Command-driven navigation is the `:` prompt. `:epic` filters to epics, `:bug` to bugs, `:issues` clears the type filter, `:layout` and `:wrap` change the display, `:project` opens the table. The aliases live in a closed table in `pkg/ui/commands.go`, ninety-three lines long, and `Tab` completes them. There is no free-form command language and no plugin hook, on purpose. A closed table is one you can test in full.

Progressive disclosure is the tree. An epic folds to one line or opens to show its children. `Enter` on an issue opens the detail. `g` opens the dependency graph from that issue outward. `b` opens the board. `Esc` steps back. Every deeper view is reached from a shallower one and returns to it.

k9s also gave b9s its look: the full-row cursor, the one-line status, plain text tokens instead of emoji, the header that `Ctrl-E` hides. None of that is architecture. All of it is why the program feels like one thing rather than a collection of features.

## What the rest of the book covers

The read path is chapters three and four: finding a project, reading it, and noticing when it changes. The UI is chapters five and six: one model that routes keys, and one query that every view shares. The write path is chapter seven. Projects, the people in them and the board follow. Then the browser, which reuses all of the above behind an HTTP server. Then the tests, and finally the decision records.

Keep the diagram at the top of this chapter in mind. Every chapter is a closer look at one of its arrows.
