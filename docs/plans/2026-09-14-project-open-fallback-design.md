# Opening a Project That Fails: Readable Reasons and a Way Back

Beads task: bd-6e8h.8 (epic bd-6e8h).

## Contents

- [Problem](#problem)
- [Decisions](#decisions)
- [Failure reasons](#failure-reasons)
- [CLI startup](#cli-startup)
- [Switching projects in the TUI](#switching-projects-in-the-tui)
- [The failure popup](#the-failure-popup)
- [Architecture](#architecture)
- [Switch state machine](#switch-state-machine)
- [Recent projects](#recent-projects)
- [Testing](#testing)
- [Documentation and ADR](#documentation-and-adr)
- [Out of scope](#out-of-scope)

## Problem

Measured on the binary at fc4cb08:

| Situation | Output | Problem |
|---|---|---|
| Folder without `.beads` | `Error loading beads: failed to read beads directory: open …/.beads`, exit 1 | A raw Go error with no way forward |
| Project without issues | `No issues found. Create some with 'bd create'!`, exit 0 | Reads as success and exits |
| Dolt server unreachable | `no beads JSONL file found in …/.beads`, exit 1 | The real cause (connection refused) is lost |

In the TUI, `SwitchProjectMsg` clears the current project's issues before the
new project loads. When the Dolt connection or reload then fails, only a status
line remains over an empty tree, and there is no way back to the project that
worked.

## Decisions

| Question | Decision |
|---|---|
| Failure text | A closed set of reasons, each with a readable message and concrete next steps, shared by CLI and TUI |
| Interactive start in a broken folder | Print the reason, then open the project that most recently opened successfully, with the reason in a popup |
| No terminal, or `--no-fallback` | Print the reason and exit 1; never open a different project silently |
| Empty project | Not a failure: it opens normally and the popup shows the `bd create` hint |
| TUI switch | Load first; replace the visible project only when the new one loaded |
| While a switch is loading | Browsing continues; the header shows `Opening X…`; `Esc` cancels; edits wait |
| Remembering what worked | `opened_at` on each recent entry, written only after issues load |
| Failed startup folder | Not added to `recent_projects`; an entry already listed keeps its slot, marked `✗` |

## Failure reasons

| Reason | Message | Try |
|---|---|---|
| `NotAProject` | `~/x is not a Beads project` | `bd init`, or start b9s inside a project |
| `ServerDown` | `Cannot reach the Dolt server at 127.0.0.1:3306` | `launchctl list \| grep dolt-tunnel`, or `bd-fallback local` |
| `Denied` | `bd_b9s may not read database ghost` | `direnv allow`, or check `BEADS_DOLT_PASSWORD` |
| `NoIssuesTable` | `Database ghost is not a Beads database` | `bd init --server --database=ghost` |
| `Unreadable` | `.beads/issues.jsonl cannot be parsed (line 12)` | Fix or re-export the file with `bd export` |
| `TimedOut` | `ghost did not open within 10s` | Retry; check the tunnel and server load |

The reason is a closed enum, never free text. It extends the existing
`datasource.Reachability` classification, which already separates
`ServerDown`, `Denied` and `NoIssuesTable`, so a down tunnel is never reported
as denied access. The original error is kept as detail for the health popup
(`Shift+D`).

An empty project is a success whose hint (`bd create`) is shown in the same
popup style, without the fallback.

## CLI startup

```
 b9s starts in folder X
        │
   OpenProject(X) ──ok──▶ open X, touch recent, record opened_at
        │fail
        ▼
 print reason and Try lines to stderr
        │
 stdin and stdout are terminals, no --no-fallback, and a last good project exists?
        │yes                                     │no
        ▼                                        ▼
 OpenProject(last good)                      exit 1
   ok ──▶ TUI on that project, popup queued
   fail ──▶ print both reasons, exit 1
```

```
b9s: cannot open ghost
  Reason  the Dolt server at 127.0.0.1:1 is not reachable (connection refused)
  Try     launchctl list | grep dolt-tunnel
          bd-fallback local
Opening b9s instead, the last project that opened successfully (12 min ago).
```

Today's silent fallback from an unreachable Dolt server to a local JSONL export
stays a success with a warning in the health popup, as before.

## Switching projects in the TUI

```
 Showing b9s ──key 3──▶ Opening ghost
                            │
         ┌──────────────────┼────────────────────────┐
         ▼                  ▼                        ▼
      loaded             failed               Esc or a newer switch
         │                  │                        │
   show ghost,         stay on b9s,             stay on b9s,
   save opened_at      popup with reason        ignore the late result
```

- The current project stays visible and browsable until the new one loaded.
- The header shows `Opening ghost…` while the load runs.
- Edit actions show `Edits wait until ghost has opened` instead of running, so
  a change can never land in the wrong project.
- Each switch carries a generation number. A result whose generation is not
  the latest is dropped.

## The failure popup

```
╭─ Cannot open ghost ──────────────────────────────╮
│ The Dolt server at 127.0.0.1:1 is not reachable. │
│                                                  │
│ Try                                              │
│   launchctl list | grep dolt-tunnel              │
│   bd-fallback local                              │
│                                                  │
│ Still showing b9s.                               │
│ r retry   Esc close   Shift+D connection details │
╰──────────────────────────────────────────────────╯
```

`r` dispatches the same switch again. The popup is also used for a failed
startup folder, where the last line reads `Showing b9s, the last project that
opened successfully.`

## Architecture

```
 internal/datasource/open.go   OpenProject(project) -> Opened | OpenFailure
                               OpenFailure{Reason, Project, Detail}
                               reason text and Try lines, total over Reason
 cmd/b9s/main.go               startup through OpenProject, TTY check,
                               --no-fallback, fallback to LastOpened
 pkg/ui/project_switch.go      SwitchState {Idle, Opening}, generation,
                               projectOpenedMsg / projectOpenFailedMsg,
                               applyOpenedProject (the only place that
                               replaces the visible project's data)
 pkg/ui/open_failure_popup.go  popup view and keys (r, Esc, Shift+D)
 pkg/config/recent.go          RecentProject.OpenedAt, MarkOpened, LastOpened
```

`OpenProject` does no UI work: it discovers sources, connects, and loads
issues, returning either everything the model needs to show the project
(issues, source info, watcher) or a classified failure. The switch handler in
`model.go` shrinks to dispatching it.

## Switch state machine

| State | Accepted events | Deadline | Deadline action |
|---|---|---|---|
| `Idle` | switch requested | none | none |
| `Opening{target, generation}` | opened, failed, `Esc`, newer switch | 10s | failed with `TimedOut` |

- **opened** (matching generation): apply the project, save `opened_at`, go to
  `Idle`.
- **failed** (matching generation): keep the visible project, show the popup,
  go to `Idle`.
- **`Esc`**: go to `Idle`; the running load's result arrives with a stale
  generation and is dropped.
- **newer switch**: increment the generation, stay `Opening` with the new
  target.

The Dolt connection's own 5s connect timeout sits inside the 10s deadline.
Entering `Opening` always starts the load and arms the deadline, whichever key,
table row or retry requested the switch.

## Recent projects

- `opened_at` is written only after issues load, through the existing merged,
  atomic `SaveRecentTo`.
- `LastOpened(except)` returns the entry with the newest `opened_at`, skipping
  the project that just failed. Entries without `opened_at` (from older configs)
  are never chosen.
- A startup folder that fails is not touched into the list. An entry already
  there keeps its slot and is marked `✗` in the header.

## Testing

TDD throughout.

- **Unit**
  - Every `Reason` has a message and at least one Try line (a table test that
    fails when a reason is added without text).
  - Classification: missing `.beads`, refused connection, access denied,
    missing `issues` table, unparsable JSONL, deadline.
  - Switch states: opened applies; failed keeps the visible project and opens
    the popup; `Esc` then a late result is dropped; a newer switch drops the
    older result; the deadline produces `TimedOut`; edits are refused while
    `Opening`.
  - `LastOpened` ordering, skipping the failed project and entries without
    `opened_at`; a failed startup folder is not added.
- **E2E** (PTY via `script`, isolated `XDG_CONFIG_HOME`)
  - Start in a folder without `.beads` with a seeded recent list: the header
    shows the fallback project and the popup names the reason.
  - Same without a terminal: exit 1 and the reason on stderr.
  - Switch to a project whose server port is closed: the previous project stays
    visible and the popup shows `Cannot reach the Dolt server`.

## Documentation and ADR

- ADR 0012: load a project before replacing the visible one, and fall back to
  the last project that opened successfully.
- README: failure reasons, the fallback, `--no-fallback`, and `opened_at`.

## Out of scope

- A connection that drops while a project is already open.
- Forgetting a recent project in-app (bd-6e8h.7).
