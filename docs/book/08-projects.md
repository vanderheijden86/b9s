# Projects

k9s has namespaces. b9s has projects, and three questions to answer about them: which ones to show, how to open one, and what to do while it is opening.

## Which ones: recent, not discovered

The header shows the digits `1` to `9` with a project name beside each. The first design, in the k9s document, had b9s scan the disk for `.beads` folders and list what it found. That was replaced early, in decision record 0008, with a list of recent projects kept in `~/.config/b9s/config.yaml`.

Discovery was dropped because it was slow, surprising and wrong. Slow because a scan of a home directory takes seconds. Surprising because the list changed when a folder was cloned or deleted, and a project's digit changed with it. Wrong because a `.beads` folder in an archived checkout is not a project you want on key `3`.

The recent list follows the k9s rules for favourite namespaces. A newly opened project goes to the front and takes `1`. A known project keeps its digit. `lock_recent: true` freezes the list so nothing moves. `0` is every project at once.

Decision record 0015 adds one filter. A recent project whose database refuses the startup user is left out of the header for the session. Showing it would offer a digit that leads to an error popup, and hiding it costs one line in the health popup that says why.

## Which ones: the catalog

`:project` opens a table of every database on the Dolt server that the startup user can read. `catalog.go` in `internal/datasource` lists them and classifies each into a closed set:

```
   ReachUnknown        not yet checked
   ReachReachable      SELECT works, issues table present
   ReachDenied         the server refused the user
   ReachNoIssuesTable  a database, but not a Beads one
   ReachServerDown     the connection failed
```

The classification comes from the MySQL error number, not from the error text, and that is the point of having a closed set. A down tunnel and a refused login look similar in prose and mean opposite things: one says "start the tunnel", the other says "ask for access". A tool that reported either as "could not open" would send the user down the wrong path half the time.

Decision record 0007 sets the credential the catalog uses: one SELECT-only account, the startup user, is the only credential b9s holds. Opening a database from the catalog reads it as that user. There is no per-project password lookup, no prompt, and nothing that could write.

## How to open one: the switch

A project switch is the one place in b9s where something slow happens in response to a single keypress. Opening a Dolt database over the tunnel takes from a few hundred milliseconds to, if the tunnel is down, a five-second connect timeout. The wrong design is obvious: clear the screen, show "loading", and hope.

Decision record 0012 chose the other design. **Open the new project before replacing the visible one.** While the new one loads, the old one stays on screen and stays usable. When the new one has loaded, it replaces the old one in one step. If it fails, the old one stays, and a popup shows the reason.

`project_switch.go` implements that as a small state machine:

```
                 digit / :project / startup
   ┌──────────┐ ────────────────────────────▶ ┌───────────┐
   │ SwitchIdle│                               │SwitchOpening│
   └──────────┘ ◀──── opened, failed, ──────── └───────────┘
                      or 10 s deadline
```

Two states, and a generation counter. Every request increments the generation and carries it into the `tea.Cmd` that opens the project. When the result comes back, `handleProjectOpened` checks the generation. A result from any other generation is stale and dropped. That is what makes pressing `2` then `3` in quick succession safe: the answer for `2` arrives, is recognised as old, and never replaces the screen that `3` is about to fill.

The deadline is ten seconds, with the Dolt driver's five-second connect timeout inside it. Every wait state in b9s has a deadline and a deadline action, and here the action is: stay on the old project, report a timeout.

While `SwitchOpening`, `IssueWriter.SetOpening` is set, and a write is refused with a message. A `bd` call that started against the old checkout and finished after the switch would have written to the wrong project.

## Why the failure has a type

A failed open returns an `OpenFailure` with an `OpenReason` from a closed set: `OpenNotAProject`, `OpenServerDown`, `OpenDenied`, `OpenNoIssuesTable`, `OpenUnreadable`, `OpenTimedOut`. The CLI and the TUI share the set.

That sharing is why the startup path can be honest. `chooseStartupProject` in `cmd/b9s/main.go` tries the folder b9s was started in. If that fails, it tries the most recent project that has opened before and shows the reason in a popup once the UI is up. But a script running `b9s` with `--no-fallback`, or with no terminal, gets an exit status and the reason on stderr instead. A script that silently opened a different project than the one it asked for would act on the wrong data, and the closed reason set is what lets the two front ends say the same thing in their own way.
