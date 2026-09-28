# Staying fresh

A viewer that shows you last minute's data is a viewer you stop trusting. b9s reloads within half a second of any change, whoever made it, and the way it does so depends on the shape of the store.

## Dolt: ask for the hash

Dolt has a function called `DOLT_HASHOF_DB()`. It returns a hash of the whole database, and it is cheap, because Dolt already keeps that hash for its own content-addressed storage. Two calls that return the same hash saw the same data. Two that differ saw a change.

`DoltWatcher` in `internal/datasource/dolt_watcher.go` runs that query every 500 milliseconds, the `refresh.poll_interval` in the config, and compares. On a difference it sends one message on a channel and the UI reloads.

The subtle part is which hash. Dolt has a `HEAD` hash, the last commit, and a working-set hash, which includes uncommitted rows. `bd` writes rows and commits them, but not always in the same instant, and other tools write without committing at all. Polling `HEAD` would miss those. `DOLT_HASHOF_DB()` covers the working set, so a change is seen as soon as it is written, committed or not.

Why poll rather than subscribe? Because there is nothing to subscribe to. A MySQL wire connection has no change feed. One tiny query every half second is a fixed, predictable cost, and on a laptop it is invisible.

## Files: watch, then fall back to polling

For JSONL and SQLite the store is a file, and the operating system will tell you when a file changes. `pkg/watcher` uses fsnotify and adds a debounce window, because an editor or `bd` may write a file in several bursts and the UI should reload once, at the end.

The trouble is that file events are not reliable everywhere. On NFS, SMB and some other network filesystems, they do not arrive. The watcher detects those filesystems and switches to polling the file's modification time. A user on a mounted share gets a slightly slower reload rather than none.

## One message, whoever sends it

Every path ends in the same place: a reload message to the UI model. The Dolt poller sends it, the file watcher sends it, and `Ctrl-R` or `F5` sends it by hand.

```
   DoltWatcher ─ hash changed ──┐
   pkg/watcher ─ file changed ──┼──▶ reload msg ──▶ Model reloads issues
   Ctrl-R / F5 ─────────────────┘                  keeps cursor, marks, folds
```

The reload replaces the issue slice and then restores what the user had: the cursor on the same issue ID, the marked set, which epics were folded, the filter and the scroll. If the cursor issue was deleted, the next row takes its place, as in k9s. If you were in the middle of a search, the field keeps its text and the results refresh under it.

There is one exception to "reload immediately", and it belongs to the browser: a live change that arrives while a finger is on a row waits until the gesture ends, because a list that re-sorts under a swipe is a list that swipes the wrong row. The terminal has no gesture that lasts long enough to need that.

## Why the write path relies on this

Chapter two promised a reason b9s never patches its own copy after a write. This is it. Because the watcher is fast and covers every writer, the UI can treat every change the same way, whether it came from your `K` key, from a teammate's `bd close` in another terminal, or from an agent running in a container half a world away. The UI has one path for "the data changed", and its own writes take that path like everyone else's.

The alternative, optimistic updates, would need a second code path for "I changed it and I am waiting to see if I was right", plus a way to reconcile when the reload says otherwise. The half-second wait is cheaper than that, and it is honest: the issue leaves the screen when it has left the store.
