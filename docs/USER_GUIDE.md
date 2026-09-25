# b9s user guide

b9s is a keyboard-driven terminal UI for [Beads](https://github.com/steveyegge/beads) issues, modelled on [k9s](https://k9scli.io/). This guide covers every view, key, query and setting. The [README](../README.md) is the short version.

## Contents

- [Starting b9s](#starting-b9s)
- [The screen](#the-screen)
- [Tree view](#tree-view)
- [Detail pane](#detail-pane)
- [Search](#search)
- [Marks and bulk actions](#marks-and-bulk-actions)
- [Editing issues](#editing-issues)
- [Creators, assignees and identities](#creators-assignees-and-identities)
- [Board view](#board-view)
- [Dependency graph](#dependency-graph)
- [Command prompt](#command-prompt)
- [Projects](#projects)
- [Data sources](#data-sources)
- [Configuration](#configuration)
- [Help and tutorial](#help-and-tutorial)
- [Mouse and tmux](#mouse-and-tmux)
- [Command-line options](#command-line-options)
- [Updating b9s](#updating-b9s)

## Starting b9s

Run `b9s` in a folder that has a `.beads` directory, or point `BEADS_DIR` at one:

```bash
b9s
BEADS_DIR=/path/to/repo/.beads b9s
```

In a linked git worktree b9s finds the `.beads` directory of the main checkout by itself.

`--filter` starts with a [query](#search) applied and `--repo` keeps only the issues whose ID starts with a prefix:

```bash
b9s --filter 'status:open label:backend'
b9s --repo api
```

`Ctrl-C` quits. b9s keeps the expand and collapse state of the tree in `.beads/tree-state.json`, so the next start looks like the last one.

## The screen

```text
┌──────────────────────────────────────────────────────────────────────┐
│ #  NAME      O  P  R │ shortcuts             │  b9s logo             │  header
│ 1  b9s       12 3  4 │ <1-9> project         │                       │
├──────────────────────────────────────────────────────────────────────┤
│ b9s  ·  Dolt 127.0.0.1:3306/b9s                                      │  title bar
├──────────────────────────────┬───────────────────────────────────────┤
│ ▸ [g9h] Epic: Run experiment │ # [g9h.2] Move to big-hetzner         │
│   ▸ [g9h.2] Move to big-h…   │                                       │
│     • [g9h.2.1] Close the f… │ Status: open   Priority: P1           │
│                              │ ...                                   │  tree | detail
├──────────────────────────────┴───────────────────────────────────────┤
│ 3 open · 1 marked      j/k move  Enter open  / search  : command  ? │  status line
└──────────────────────────────────────────────────────────────────────┘
```

- **Header.** The project table with the recent projects on `1`-`9`, the key hints and the logo. `Ctrl-E` or `H` hides and shows it.
- **Title bar.** The project name and its data source.
- **Tree and detail pane.** Side by side when the terminal is at least 100 columns wide, otherwise the detail pane opens over the tree. `<` and `>` move the divider.
- **Status line.** Counts, the active filters and marks, the key hints for the focused pane, and the result of the last action.

## Tree view

The tree is always present. Every issue sits under its parent, and an issue without a parent sits at the top level.

### Moving

| Keys | Action |
|------|--------|
| `j` `k`, `Up` `Down` | Move the cursor |
| `h` | Collapse the issue, or go to its parent when it is already collapsed |
| `l` | Expand the issue, or go to its first child when it is already expanded |
| `p` | Go to the parent |
| `{` `}` | Go to the first or last sibling |
| `Ctrl-F` `Ctrl-B`, `PgDn` `PgUp` | Page down and up |
| `Ctrl-D` `Ctrl-U` | Half a page down and up |
| `Home`, `End` `G` | Go to the top, the bottom |

### Folding

| Keys | Action |
|------|--------|
| `Tab` | Fold or unfold the issue under the cursor |
| `Shift-Tab` | Fold or unfold the whole tree |
| `X`, `Z` | Expand all, collapse all |
| `Ctrl-A` | Switch between expand all and collapse all |

### Filtering

| Keys | Action |
|------|--------|
| `o`, `C`, `r`, `a` | Show open, closed, ready or all issues |
| `L`, `A` | Put the labels or the assignees on `1`-`9`; press a digit to filter, `a` to clear |
| `f` | Show only the top-level branch of the issue under the cursor. Press again to undo |
| `x` | Show only the subtree of the issue under the cursor. Press again to undo |
| `F` | Follow: when another agent changes an issue, move the cursor to it |

Ready means open with no open blocker. Filters compose with each other and with the [search query](#search).

### Sorting and columns

| Keys | Action |
|------|--------|
| `s` | Pick the sort field and direction for this session |
| `\|` | Choose for each optional column whether it is automatic, shown or hidden: Lane state, Creator, Assignee, Updated and ID |
| `Ctrl-W` | Toggle wide columns |

The sort fields are priority, created, updated, title, status, type and deps. The default comes from the [configuration file](#configuration), and the tree starts sorted by creation date, newest first.

### Other tree keys

| Keys | Action |
|------|--------|
| `Enter` | Move focus to the detail pane |
| `d` | Show or hide the detail pane |
| `\` or `:layout` | Stack the detail pane below the tree at full width, or put it back to the right |
| `<`, `>` | Shrink or grow the tree's share of the screen |
| `c` | Copy the issue ID and title to the clipboard |
| `Ctrl-N` | Create an issue |
| `e`, `S`, `K`, `Delete` | Edit, set the status, close, delete ([bulk actions](#marks-and-bulk-actions)) |

## Detail pane

`Enter` moves focus to the detail pane, and `Enter` or `Esc` moves it back. The focused pane has the bright border, and the footer shows its keys. The pane renders the description, notes, comments and dependencies as Markdown.

| Keys | Action |
|------|--------|
| `j` `k` | Scroll |
| `Home`, `End` | Jump to the top, the bottom |
| `n`, `p` | Open the next or previous sibling |
| `c` | Copy the whole issue as Markdown |
| `d` | Hide the pane |
| `Enter`, `Esc` | Return to the tree |

## Search

Press `/` to query the loaded issues. The query field appears only while you type. Results update with every keystroke, and while the field is focused every key goes into the query, so a query letter never triggers an action.

| Keys | Action |
|------|--------|
| `Enter` | Hide the field and keep the query |
| `/` | Edit the query again |
| `Esc` | Clear the query |
| `Tab` | Complete a field name, a value, a label or a word from the loaded issues |
| `n`, `N` | Go to the next or previous match |
| `O` | Occur: show only the matches. Press again to show the tree |

### Plain words

Plain words match issue IDs, titles and labels, and every word must match. Short words match as a subsequence, so `inh map` finds `[inh] Inheritance mapping`. Longer words need contiguous text. Descriptions, notes, comments and dependency text are never searched, so every result is explained by what you see in the row.

### Predicates

A predicate narrows one field:

| Predicate | Example |
|-----------|---------|
| `id:` | `id:7rt1` |
| `title:` | `title:search` |
| `status:` | `status:open` |
| `priority:` | `priority:1` or `priority:p1` |
| `type:` | `type:epic` |
| `label:` | `label:ui` |
| `assignee:` | `assignee:andre` |
| `project:` | `project:b9s` |

Different fields combine with AND. Repeated values for one field combine with OR. `!` excludes a match. Priority is exact, so `p1` does not match `p10`. An incomplete predicate such as `label:` adds no constraint, so the list stays visible until you type a value.

```text
status:open status:blocked type:epic     open or blocked epics
priority:p1 label:frontend !assignee:me  someone else's P1 frontend work
```

### What the tree shows

A hit keeps its whole branch: the parents up to the top-level issue, and everything below it. Branches without a hit are hidden, so matching an epic shows its features and tasks, and a matching standalone task shows on its own. Hit rows keep their colours and the rest of the branch is dimmed. `Tab` still folds a revealed branch.

```text
query "tunnel"                              query "acceptance"
▸ [3bg] Epic: UAT             (dimmed)      ▸ [3bg] Epic: User Acceptance   (hit)
└─ ▸ [3bg.1] Verify           (dimmed)      ├─ ▸ [3bg.1] Verify            (dimmed)
   └─ • [3bg.1.1] … tunnel    (hit)         │  └─ • [3bg.1.1] Connect     (dimmed)
                                            └─ ▸ [3bg.2] Deploy Acceptance (hit)
```

The first match is placed in the upper third of the viewport, not at the bottom edge. Matches are counted within the active status, label and assignee filters, and within the subtree when `x` is active.

## Marks and bulk actions

| Keys | Action |
|------|--------|
| `Space`, `m` | Mark or unmark the issue under the cursor |
| `V`, `Ctrl-Space` | Mark the range from the nearest mark to the cursor |
| `u` | Unmark the issue under the cursor |
| `Ctrl-\`, `M`, `U` | Clear all marks |

`K` (close), `Delete` and `S` (set status) act on every marked issue, or on the cursor row when nothing is marked. Close and delete ask for confirmation first, and the confirmation lists the issues it covers. When the issue under the cursor disappears, the next issue takes its row.

`V` and `Ctrl-Space` do the same thing. macOS claims `Ctrl-Space` for input-source switching by default, so `V` is the key that always works.

## Editing issues

Every change goes through the `bd` CLI, so b9s and `bd` always agree on what is stored. b9s needs `bd` on `PATH` and a local checkout of the project to run it in. A project opened from its database alone is read-only, and the status line says so when you try to write.

| Keys | Action | Runs |
|------|--------|------|
| `e` | Edit the issue in a form | `bd update` |
| `Ctrl-N` | Create an issue | `bd create` |
| `S` | Pick a status: open, in_progress, blocked, deferred, pinned, hooked, review or closed | `bd update --status` |
| `K` | Close, after confirmation | `bd close` |
| `Delete` | Delete, after confirmation | `bd delete --force` |

The edit form has the fields Title, Status, Description, Priority, Type, Assignee, Labels, Defer until and Notes, and shows the Creator in its title. `Tab` and `Shift-Tab` move between fields, and `Enter` starts a new line in Description and Notes. `→` at the end of the typed text completes an assignee or label. `Ctrl-E` opens Description or Notes in `$EDITOR` and reads the text back when the editor exits. `Ctrl-S` saves and `Esc` cancels. A value in Defer until, such as `tomorrow` or `+3d`, runs `bd defer`.

The view reloads by itself once `bd` has written, because b9s watches the data source. The all-projects view (`0`) is read-only.

## Creators, assignees and identities

Every issue carries two people, and b9s shows them as separate fields:

| Field | Source | Changes |
|-------|--------|---------|
| Creator | `created_by`, the `bd` actor at creation time, with `owner` (the creator's git email) as detail | Never |
| Assignee | `assignee`, whoever holds the issue now | On every reassignment |

The detail pane and the board detail show both. In the tree, `|` adds either one as a column.

`Ctrl-N` fills in the Assignee with you, and the form title reads `as @you`. b9s finds you as `bd` does: `BEADS_ACTOR`, then `BD_ACTOR`, then `git config user.name` in the project, then `USER`. It passes that name to `bd create --actor`, so it becomes the Creator. Changing the Assignee field changes only the assignee.

### Mapping names to people

The same person often appears under several names: a git user name, an email, a lane agent name. List them once in the `bd` config key `b9s.identities`, and b9s shows one name for all of them:

```bash
bd config set b9s.identities '[
  {"name": "vanderheijden86", "kind": "human", "aliases": ["vanderheijden86@gmail.com", "andre"]},
  {"name": "lane-agents", "kind": "agent", "aliases": ["ubuntu", "BrownBear"]}
]'
```

- `kind` is `human`, `agent` or `pool`. Every name in `bd`'s own `claim.pools` key is a `pool` without further configuration.
- Names match as `bd`'s claim path compares them: `.`, `_` and `-` count as one separator, so `gastown.mayor` and `gastown_mayor` are the same name.
- A name without an entry shows as itself.
- The assignee picker (`A`) and the edit form's suggestions list each identity once, with the counts of all its aliases. Humans come first, then agents and pools, tagged `(agent)` and `(pool)`. Filtering on a name shows issues assigned to any of its aliases.
- An alias listed under two names is a configuration error. The first entry wins, and the health popup (`D`) reports the conflict.
- The list lives in the project database, so every clone and lane sees the same one, and it reloads with the issues. JSONL projects have no config table and show raw names.

The Dolt SQL login, for example `bd_b9s`, is one credential shared by every agent in a workspace. The health popup shows it as `SQL login: bd_b9s (shared credential)`, but b9s never treats it as a person. The popup's `You:` line shows who b9s creates issues as. See [ADR 0014](adr/0014-map-actors-to-identities-in-b9s-config.md).

## Board view

`b` opens the board and `Esc` returns to the tree. The board groups the visible issues into columns. `s` cycles the grouping:

| Grouping | Columns |
|----------|---------|
| Status | Open, In Progress, Blocked, Closed |
| Priority | P0 Critical, P1 High, P2 Medium, P3+ Other |
| Type | Bug, Feature, Task, Epic |

Columns with issues share the width equally. Each issue is a boxed card: the title on up to two lines, a tag line with `blocked by X`, `lane: stage` and `blocks N` when they apply, and a footer with the type icon, the ID, the priority and the age. Empty columns fold into narrow rails; `h` or `l` onto a rail opens it. The closed column is hidden until `c` shows it, and the bar above the board counts the hidden closed issues.

An issue belongs to its nearest epic ancestor through parent-child links. The board puts the issues of each epic in one horizontal lane, aligned across the columns, and has two designs for the epic itself ([ADR 0018](adr/0018-show-epics-as-a-rail-or-rows-and-hide-closed.md)). `v` switches between them and keeps the selection, and `ui.board_epics` in the config sets the one used at start.

| Design | What it shows |
|--------|---------------|
| Epic rail (default) | A column on the left holds each lane's epic: its ID, title, a completion bar with done/total and the issue count. The epic stays in view while its lane scrolls. |
| Epic rows | A header row above each lane with the epic's ID, title, issue count and completion bar. |

Issues without an epic sit in a last `No epic` lane. `Tab` folds the selected issue's epic: its cards leave the board and each column shows how many it hides. `Shift-Tab` folds every epic, or unfolds them all when one is folded. The epic's own issue is the lane header, not a card; selecting it highlights the lane.

Completion counts every issue under the epic in the project, closed ones included, so a filter never changes it. Lanes come in order of the most urgent open issue in them. A project without epics shows the plain board in every design.

A row separates four facts: the column is the stored status, `blocked by X` names an open blocker, `lane: stage` comes from the `lane-stage=` label, and `blocks N` counts the issues that wait on this one. In single-project mode IDs drop the project prefix.

| Keys | Action |
|------|--------|
| `h` `l`, `Left` `Right` | Change the column |
| `j` `k`, `Up` `Down` | Move between issues |
| `v` | Switch the epic design: rail or rows |
| `Tab` | Fold or unfold the selected issue's epic |
| `Shift-Tab` | Fold all epics, or unfold them all when one is folded |
| `c` | Show or hide the closed column (status swimlanes) |
| `0`, `$`, `gg`, `G` | First card, last card, top, bottom |
| `Enter` | Open the card in the detail pane |
| `o`, `r` | Show open or ready issues |
| `/`, `n` `N` | Search, next and previous match |
| `e` | Hide or show empty columns |
| `y` | Copy the issue ID |
| `Esc` | Back to the tree |

## Dependency graph

`g` opens the blocking neighbourhood of the issue under the cursor: what blocks it, and what it blocks. `j` and `k` move, `Enter` opens the selected issue in the detail pane, and `Esc` returns to the tree.

## Command prompt

`:` opens a prompt at the bottom of the screen, as in k9s. Type an alias and press `Enter`. `Tab` accepts the suggestion, and `Backspace` on an empty prompt closes it.

| Command | Aliases | Effect |
|---------|---------|--------|
| `:epic` | `:epics`, `:ep` | Show only epics |
| `:feature` | `:features`, `:feat`, `:ftr` | Show only features |
| `:task` | `:tasks` | Show only tasks |
| `:bug` | `:bugs` | Show only bugs |
| `:chore` | `:chores` | Show only chores |
| `:issues` | `:all` | Show every type again |
| `:project` | `:projects`, `:proj` | List the databases on the Dolt server |
| `:mouse` | | Hand the mouse to the terminal, or take it back |
| `:layout` | | Stack the detail pane below the tree, or put it back to the right |

## Projects

The header lists the project b9s started in and up to nine projects you opened recently, on the keys `1`-`9`. A project b9s has not seen yet is added as `1`, and a project already in the list keeps its number, so switching never renumbers the header. `0` combines the issues of every Dolt project in the list into one read-only view. b9s does not scan folders for projects. A recent project whose server or tunnel is down stays in the header and shows `✗` in place of its issue counts. A recent project whose database refuses the startup project's Dolt user stays out of the header for that session. It stays in `recent_projects` and comes back when b9s starts as a user with access to that database. The counts come from the checkout's configured source, never from a JSONL export beside a Dolt checkout. See [ADR 0008](adr/0008-list-recent-projects-instead-of-discovered-folders.md) and [ADR 0015](adr/0015-hide-recent-projects-the-startup-user-cannot-read.md).

`:project` lists the Beads databases that the startup project's Dolt user can read. Move with `j` and `k` and press `Enter` to open one. A database without a local checkout opens read-only, because writes run `bd` inside a checkout. See [ADR 0007](adr/0007-read-shared-beads-through-one-select-only-catalog-account.md).

b9s loads a project before it replaces the one on screen. While it loads, the status line shows `Opening <name>…` and `Esc` cancels. If the project cannot be opened, you stay where you were and a popup names the reason:

| Reason | Example |
|--------|---------|
| Not a Beads project | the folder has no `.beads` directory |
| Server unreachable | the Dolt server or SSH tunnel is down |
| Access denied | the Dolt user may not read that database |
| Not a Beads database | the database has no `issues` table, or does not exist |
| Unreadable | no issues file or Dolt configuration could be read |
| Timed out | the project did not open within 10 seconds |

Press `r` in the popup to retry. When the folder b9s starts in cannot be opened, b9s opens the recent project that last opened successfully and shows the same popup. With `--no-fallback`, or without a terminal, it exits with status 1 instead, so a script never acts on the wrong project. See [ADR 0012](adr/0012-open-a-project-before-replacing-the-visible-one.md).

## Data sources

b9s reads the `.beads` directory of the current folder, or the one in `BEADS_DIR`. When it finds more than one source, it uses the first of these:

| Source | Found through |
|--------|---------------|
| Dolt server | `.beads/metadata.json` with `"dolt_mode": "server"` |
| SQLite | `.beads/beads.db`, from older `bd` versions |
| JSONL | `.beads/issues.jsonl`, also in linked git worktrees |

A configured Dolt server always wins, even over a newer JSONL file. If b9s cannot connect to it, b9s falls back to SQLite and then JSONL. `D` shows the active source and the connection error while the header is visible.

b9s cannot read embedded Dolt (`bd init` without `--server`), because there is no server to connect to. [Migrating embedded Dolt to a server](embedded-to-server-migration.md) explains the move.

### Dolt connection

b9s takes the Dolt host, port, user and database from `metadata.json` and the password from `BEADS_DOLT_PASSWORD`. It sends that password only to a loopback address or to an exact `host:port` listed in `B9S_TRUSTED_DOLT_ENDPOINTS` (comma-separated), so a cloned repository cannot choose where your password goes. See [ADR 0013](adr/0013-trust-dolt-endpoints-before-sending-environment-credentials.md).

### Live reload

b9s polls the Dolt working-set hash every 500 ms by default, so changes from `bd` or another agent appear by themselves, uncommitted ones included. JSONL files reload on filesystem events, with polling on network filesystems where events are unreliable. `Ctrl-R` or `F5` reloads at once.

## Configuration

b9s reads `~/.config/b9s/config.yaml`, or `$XDG_CONFIG_HOME/b9s/config.yaml`. Every key is optional:

```yaml
ui:
  board_epics: rail       # rail or rows; v switches
  sort:
    field: created      # priority, created, updated, title, status, type or deps
    direction: desc     # asc or desc; leave out for the field's natural order
refresh:
  poll_interval: 500ms  # Dolt hash polling; at least 100ms; restart b9s after a change
lock_recent: false      # true freezes the list below
recent_projects:        # b9s maintains this list; edit it to remove an entry
  - name: b9s
    database: b9s
    host: 127.0.0.1:3306
    path: /Users/me/src/b9s   # empty for a database without a local checkout
```

`s` picks another sort for the current session only. The override survives reloads and is not saved, so the next start returns to the configured sort. If the file does not parse, for example because of an unknown sort field, b9s ignores the whole file, starts with the defaults and never saves over it. See [ADR 0010](adr/0010-take-tree-sort-from-user-config.md).

### Environment variables

| Variable | Effect |
|----------|--------|
| `BEADS_ACTOR`, `BD_ACTOR` | Who `Ctrl-N` creates issues as, before `git config user.name` and `USER` |
| `BEADS_DIR` | The `.beads` directory to read, instead of the one in the current folder |
| `BEADS_DOLT_PASSWORD` | The password for the Dolt user in `metadata.json` |
| `B9S_TRUSTED_DOLT_ENDPOINTS` | Comma-separated `host:port` entries that may receive that password, besides loopback |
| `B9S_DEBUG` | `1` writes debug messages to stderr |
| `XDG_CONFIG_HOME` | Where `b9s/config.yaml` lives; the default is `~/.config` |

## Help and tutorial

| Keys | Action |
|------|--------|
| `?` | The help overlay with every key |
| `Ctrl-S` in the help overlay | Search the overlay by key or description. `Enter` keeps the filter, `Esc` clears it |
| `` ` `` | The interactive tutorial. Progress is saved between sessions |

## Mouse and tmux

The mouse wheel moves through issues and scrolls the detail pane. Because b9s captures the mouse for this, a drag does not select text, and in tmux with `mouse on` the drag goes to b9s instead of starting copy mode.

Type `:mouse` to hand the mouse to the terminal, so a drag selects text. Type `:mouse` again to scroll with the wheel. Without it, hold the terminal's override key while you drag: `Option` in iTerm2, `Shift` in most other terminals. See [ADR 0003](adr/0003-capture-mouse-wheel-with-shift-selection.md).

A tmux binding that opens b9s with a query in a new window:

```tmux
bind-key B new-window -c '#{pane_current_path}' "b9s --filter 'status:open label:backend'"
```

## Command-line options

| Option | Effect |
|--------|--------|
| `--filter '<query>'` | Start with a [query](#search) applied |
| `--repo <prefix>` | Show only issues whose ID starts with the prefix, for example `api` |
| `--no-fallback` | Exit with status 1 when the current folder cannot be opened |
| `--debug` | Write a debug log to `.b9s/debug.log` |
| `--cpu-profile <file>` | Write a CPU profile |
| `--background-mode`, `--no-background-mode` | Turn the experimental background loader on or off for this run |
| `--check-update` | Report whether a newer release exists |
| `--update` | Install the latest release; `--yes` skips the prompt |
| `--rollback` | Go back to the version before the last update |
| `--version` | Print the version and build information |
| `--help` | List the options |

The background loader moves file reading off the UI thread. It is off by default. The flags win over `B9S_BACKGROUND_MODE=1` or `0` in the environment, which wins over `experimental.background_mode: true` in the configuration file.

## Updating b9s

`b9s --check-update` compares the running version with the latest GitHub release. `b9s --update` downloads that release, verifies its checksum, keeps the current binary as `<binary>.backup` and replaces it. `b9s --rollback` restores the backup. With Homebrew, `brew upgrade b9s` does the same job.
